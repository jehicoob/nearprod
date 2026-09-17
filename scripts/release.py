#!/usr/bin/env python3
"""Build archives locally. No tags, commits, network publishing or installations."""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

from release_lib import (TARGETS, archive_name, check_embedded_inputs, make_archive,
                         render_formula, require_clean_worktree, validate_version,
                         write_checksums)

ROOT = Path(__file__).resolve().parents[1]


def output(*cmd: str) -> str:
    return subprocess.check_output(cmd, cwd=ROOT, text=True, stderr=subprocess.PIPE).strip()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="Directorio NUEVO de salida; nunca se sobrescribe")
    parser.add_argument("--tag", help="Tag a validar, por ejemplo v0.7.0")
    parser.add_argument("--repository", default=os.getenv("GITHUB_REPOSITORY", ""), help="OWNER/REPO real; solo se usa para generar la fórmula")
    parser.add_argument("--snapshot", action="store_true", help="Validación local NO publicable; permite otro toolchain")
    parser.add_argument("--prebuilt-ui", action="store_true", help="Solo con --snapshot; usa los assets entregados")
    args = parser.parse_args()
    version = validate_version((ROOT / "VERSION").read_text().strip())
    if args.tag and (args.tag != f"v{version}" or args.snapshot):
        parser.error("El tag debe ser v + VERSION y no puede ser un snapshot.")
    if args.prebuilt_ui and not args.snapshot:
        parser.error("--prebuilt-ui requiere --snapshot; no es un release publicable.")
    if not args.snapshot:
        require_clean_worktree(output("git", "status", "--porcelain=v1", "--untracked-files=all"))
    if args.tag:
        if output("git", "rev-parse", "HEAD") != output("git", "rev-list", "-n", "1", f"refs/tags/{args.tag}"):
            parser.error("El checkout no corresponde al tag solicitado.")
    actual_go = output("go", "env", "GOVERSION")
    expected_go = "go" + (ROOT / ".go-version").read_text().strip()
    if not args.snapshot and actual_go != expected_go:
        parser.error(f"Release requiere {expected_go}, no {actual_go}. Actualiza/revisa .go-version o usa --snapshot solo para validar.")
    if not args.prebuilt_ui:
        actual_node = output("node", "--version").removeprefix("v")
        expected_node = (ROOT / ".node-version").read_text().strip()
        if not args.snapshot and actual_node != expected_node:
            parser.error(f"Release requiere Node {expected_node}; encontrado {actual_node}.")
        subprocess.run(["npm", "run", "build:ui"], cwd=ROOT, check=True)
        if not args.snapshot:
            require_clean_worktree(output("git", "status", "--porcelain=v1", "--untracked-files=all"))
    check_embedded_inputs(ROOT)
    if args.snapshot:
        version += "-dev"
    destination = (args.output or ROOT / "dist" / version).resolve()
    if destination.exists():
        parser.error(f"La salida ya existe: {destination}. Elige otro --output; no se borra automáticamente.")
    destination.parent.mkdir(parents=True, exist_ok=True)
    try:
        commit = output("git", "rev-parse", "HEAD")
        epoch = int(output("git", "show", "-s", "--format=%ct", "HEAD"))
    except (subprocess.CalledProcessError, FileNotFoundError):
        commit, epoch = "unknown", 0
    metadata = {"version": version, "commit": commit, "go": actual_go,
                "cgo": False, "snapshot": args.snapshot, "prebuiltUI": args.prebuilt_ui,
                "targets": [f"{s}/{a}" for s, a in TARGETS]}
    # Build inside a temporary sibling; failures never expose a partial output directory.
    with tempfile.TemporaryDirectory(prefix=".nearprod-release-", dir=destination.parent) as tmp:
        stage = Path(tmp) / "artifacts"
        stage.mkdir()
        for system, arch in TARGETS:
            env = {**os.environ, "GOOS": system, "GOARCH": arch, "VERSION": version, "BUILD_UI": "0", "CGO_ENABLED": "0"}
            subprocess.run(["sh", "scripts/build.sh"], cwd=ROOT, env=env, check=True)
            binary = ROOT / "bin" / f"nearprod-{system}-{arch}"
            members = {"nearprod": (binary.read_bytes(), 0o755)}
            for source, name in (("LICENSE", "LICENSE"), ("THIRD_PARTY_NOTICES.md", "THIRD_PARTY_NOTICES.md"), ("docs/INSTALAR-BINARIO.md", "INSTALAR.md")):
                members[name] = ((ROOT / source).read_bytes(), 0o644)
            for source in sorted((ROOT / "internal/webui/dist/vendor").glob("*-LICENSE.txt")):
                members[f"licenses/{source.name}"] = (source.read_bytes(), 0o644)
            members["build-info.json"] = (json.dumps({**metadata, "target": f"{system}/{arch}"}, indent=2).encode() + b"\n", 0o644)
            make_archive(stage / archive_name(version, system, arch), members, epoch)
        (stage / "build-info.json").write_text(json.dumps(metadata, indent=2) + "\n")
        (stage / "RELEASE_NOTES.md").write_text(
            f"# NearProd {version}\n\n"
            "Binarios para macOS/Linux ARM64 y AMD64. Verifica SHA256SUMS.txt antes de ejecutar.\n\n"
            "El panel se abre con `nearprod ui`. Node, npm y Go no son dependencias de ejecución. "
            "Docker/Compose y su Engine siguen siendo requisitos externos para gestionar contenedores.\n\n"
            "No se incluye firma Developer ID ni notarización Apple. Se conservan los términos de LICENSE.\n",
            encoding="utf-8")
        if args.repository and not args.snapshot:
            (stage / "nearprod.rb").write_text(render_formula(stage, args.repository), encoding="utf-8")
        write_checksums(stage)
        stage.rename(destination)
    print(f"Artefactos: {destination}")
    if args.snapshot:
        print("SNAPSHOT DE VALIDACIÓN: NO PUBLICAR estos binarios.")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as exc:
        print(f"Release no generado: {exc}", file=sys.stderr)
        sys.exit(1)
