"""Helpers for NearProd packaging; Python standard library only, no network."""
from __future__ import annotations

import gzip
import hashlib
import io
import json
import re
import tarfile
from pathlib import Path

TARGETS = (("darwin", "arm64"), ("darwin", "amd64"), ("linux", "amd64"), ("linux", "arm64"))
STABLE = re.compile(r"(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\Z")
REPOSITORY = re.compile(r"[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*\Z")


def validate_version(value: str) -> str:
    if not STABLE.fullmatch(value):
        raise ValueError("Esta primera distribución admite versiones estables X.Y.Z, sin v ni ceros iniciales.")
    return value


def validate_repository(value: str) -> str:
    if not REPOSITORY.fullmatch(value) or value.endswith(".git"):
        raise ValueError("Usa el repositorio real en formato OWNER/REPO, sin URL, .git ni credenciales.")
    return value


def require_clean_worktree(status: str) -> None:
    if status.strip():
        raise ValueError("Release requiere un checkout limpio; guarda y revisa todos los cambios o usa --snapshot solo para validar.")


def archive_name(version: str, system: str, arch: str) -> str:
    if (system, arch) not in TARGETS:
        raise ValueError("Destino no soportado")
    return f"nearprod_{version}_{system}_{arch}.tar.gz"


def digest(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def check_embedded_inputs(root: Path) -> None:
    # go:embed all:dist includes hidden files. Fail closed on unexpected files.
    dist = root / "internal/webui/dist"
    public = {p.relative_to(root / "ui/public").as_posix() for p in (root / "ui/public").rglob("*") if p.is_file()}
    javascript = {"ui/" + p.relative_to(root / "ui/src").with_suffix(".js").as_posix()
                  for p in (root / "ui/src").rglob("*")
                  if p.suffix in (".ts", ".tsx") and not p.name.endswith(".d.ts")}
    vendor = {"vendor/" + n for n in ("react.js", "react-dom.js", "react-LICENSE.txt", "react-dom-LICENSE.txt", "go-LICENSE.txt")}
    allowed = public | javascript | vendor
    for path in dist.rglob("*"):
        if path.is_symlink() or path.name.lower().startswith(".env") or path.suffix.lower() in (".key", ".pem", ".p12") or (path.is_file() and path.relative_to(dist).as_posix() not in allowed):
            raise ValueError(f"Asset inesperado o enlace en el embed: {path.relative_to(root)}")
    for base in (root / "examples", root / "internal/acceptanceassets/examples"):
        for path in base.rglob("*"):
            if path.is_symlink():
                raise ValueError(f"No se empaquetan enlaces de fixtures: {path.relative_to(root)}")
            name = path.name.lower()
            if (name.startswith(".env") and name != ".env.example") or name.endswith((".key", ".pem", ".p12")) or name in {"id_rsa", "id_ed25519", ".ds_store", "node_modules", ".git"}:
                raise ValueError(f"Archivo no permitido en fixtures embebidas: {path.relative_to(root)}")
    for name in ("index.html", "ui/App.js", "vendor/react.js", "vendor/react-dom.js"):
        if not (dist / name).is_file():
            raise ValueError(f"Falta un asset obligatorio: {name}")


def make_archive(output: Path, members: dict[str, tuple[bytes, int]], epoch: int) -> None:
    # Normalized tar/gzip metadata; identical inputs/toolchain yield identical archives.
    with output.open("xb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as gz:
            with tarfile.open(fileobj=gz, mode="w", format=tarfile.PAX_FORMAT) as tar:
                for name, (data, mode) in sorted(members.items()):
                    if Path(name).is_absolute() or ".." in Path(name).parts:
                        raise ValueError("Ruta de archivo insegura")
                    info = tarfile.TarInfo(name)
                    info.size, info.mode, info.mtime = len(data), mode, epoch
                    info.uid = info.gid = 0
                    info.uname = info.gname = ""
                    tar.addfile(info, io.BytesIO(data))


def write_checksums(directory: Path) -> None:
    items = sorted(p for p in directory.iterdir() if p.is_file() and p.name != "SHA256SUMS.txt")
    (directory / "SHA256SUMS.txt").write_text("".join(f"{digest(p)}  {p.name}\n" for p in items), encoding="utf-8")


def render_formula(directory: Path, repository: str) -> str:
    repository = validate_repository(repository)
    meta = json.loads((directory / "build-info.json").read_text(encoding="utf-8"))
    if meta.get("snapshot") or meta.get("prebuiltUI"):
        raise ValueError("No se genera un tap público para snapshots ni UI precompilada sin validar.")
    version = validate_version(meta["version"])
    lines = [
        "# Generado a partir de los archives reales; no editar los hashes a mano.",
        "class Nearprod < Formula",
        '  desc "Gestor personal de entornos locales Docker Compose"',
        f'  homepage "https://github.com/{repository}"',
        f'  version "{version}"',
        '  license "MIT"',
    ]
    for system, brew_os in (("darwin", "macos"), ("linux", "linux")):
        lines.append(f"  on_{brew_os} do")
        for arch, brew_arch in (("arm64", "arm"), ("amd64", "intel")):
            name = archive_name(version, system, arch)
            file = directory / name
            if not file.is_file() or file.is_symlink():
                raise ValueError(f"Falta el archive real: {name}")
            lines.extend([
                f"    on_{brew_arch} do",
                f'      url "https://github.com/{repository}/releases/download/v{version}/{name}"',
                f'      sha256 "{digest(file)}"',
                "    end",
            ])
        lines.append("  end")
    lines.extend([
        "", "  def install", '    bin.install "nearprod"',
        '    prefix.install "LICENSE", "THIRD_PARTY_NOTICES.md"',
        '    pkgshare.install "INSTALAR.md", "licenses"', "  end", "",
        "  def caveats", "    <<~EOS",
        "      Abre el panel con: nearprod ui",
        "      Actualiza con: brew upgrade nearprod",
        "      No ejecutes nearprod install sobre la instalación Homebrew.",
        "      Una función/alias anterior de nearprod puede ocultar este ejecutable:",
        "        type -a nearprod",
        "      Docker/Compose y el Engine (por ejemplo Colima) se gestionan aparte.",
        "      Actualizar NearProd no reinicia el agente ni tus contenedores.",
        "    EOS", "  end", "", "  test do",
        '    assert_equal version.to_s, shell_output("#{bin}/nearprod --version").strip',
        "  end", "end", "",
    ])
    return "\n".join(lines)
