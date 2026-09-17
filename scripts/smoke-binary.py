#!/usr/bin/env python3
"""CLI + HTTP smoke with isolated HOME, empty PATH, and no real Docker."""
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--expected", required=True)
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    with tempfile.TemporaryDirectory(prefix="nearprod-smoke-", dir="/tmp") as raw:
        root = Path(raw).resolve()
        home = root / "home"
        home.mkdir(mode=0o700)
        executable = root / "nearprod"
        shutil.copyfile(binary, executable)
        executable.chmod(0o755)
        env = {"HOME": str(home), "NEARPROD_HOME": str(home / ".nearprod"),
               "PATH": "", "LANG": "C", "TMPDIR": str(root)}
        for command in ("version", "--version"):
            result = subprocess.run([str(executable), command], cwd=root, env=env, text=True,
                                    capture_output=True, timeout=10, check=True)
            assert result.stdout.strip() == args.expected, result.stdout
        print("PASS version y --version fuera del repositorio, sin Node/Go/Docker en PATH", flush=True)
        with (root / "agent.log").open("w+") as log:
            process = subprocess.Popen([str(executable), "serve", "--foreground", "--port", "0"],
                                       cwd=root, env=env, stdout=log, stderr=log)
            try:
                info = None
                deadline = time.monotonic() + 15
                while time.monotonic() < deadline:
                    if process.poll() is not None:
                        log.seek(0)
                        raise AssertionError(f"El agente terminó: {log.read()}")
                    try:
                        info = json.loads((home / ".nearprod/agent.json").read_text())
                        break
                    except (OSError, ValueError):
                        time.sleep(0.1)
                assert info is not None, "No apareció agent.json"
                opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
                for path in ("/", "/ui/App.js", "/ui/infrastructure.js", "/vendor/react.js", "/vendor/react-dom.js", "/styles.css", "/api/health"):
                    with opener.open(info["url"] + path, timeout=5) as response:
                        data = response.read()
                        assert response.status == 200 and len(data) > 20, path
                        assert response.headers["Content-Security-Policy"], path
                        if path == "/api/health":
                            assert json.loads(data)["version"] == args.expected
                print("PASS HTTP real, assets embebidos, CSP y versión de health", flush=True)
                for path, status in (("/api/catalog", 401), ("/ruta-inexistente", 404), ("/ui/no-existe.js", 404)):
                    try:
                        opener.open(info["url"] + path, timeout=5)
                    except urllib.error.HTTPError as exc:
                        assert exc.code == status, (path, exc.code)
                    else:
                        raise AssertionError(f"No se respetó aislamiento/404: {path}")
                print("PASS API protegida y rutas inexistentes sin fallback indiscriminado", flush=True)
            finally:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)
        print("PASS proceso cerrado; únicamente se utilizó HOME temporal", flush=True)


if __name__ == "__main__":
    main()
