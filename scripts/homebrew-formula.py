#!/usr/bin/env python3
"""Generate the formula from real archives without contacting GitHub."""
import argparse
import sys
from pathlib import Path
from release_lib import render_formula

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--repository", required=True, help="OWNER/REPO real que almacena las releases")
parser.add_argument("--artifacts", type=Path, required=True)
parser.add_argument("--output", type=Path, required=True)
args = parser.parse_args()
try:
    text = render_formula(args.artifacts.resolve(), args.repository)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("x", encoding="utf-8") as f:
        f.write(text)
except (OSError, ValueError, KeyError) as exc:
    parser.exit(1, f"No se generó la fórmula: {exc}\n")
print(args.output)
