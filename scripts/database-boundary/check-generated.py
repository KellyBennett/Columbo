#!/usr/bin/env python3
"""Regenerate the four claimed sqlc files in a private copy, then byte-compare."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
parser.add_argument("--sqlc", default=os.environ.get("SQLC"))
args = parser.parse_args()
root = args.root.resolve()
sqlc = args.sqlc or str(Path(os.environ.get("COLUMBO_TOOLS_BIN", root / ".tools/bin")) / "sqlc")
sqlc = shutil.which(sqlc) or str(Path(sqlc).resolve())
version = next(line.split("=", 1)[1] for line in
               (Path(__file__).parent / "tools.env").read_text().splitlines()
               if line.startswith("SQLC_VERSION="))
manifest = json.loads((Path(__file__).parent / "manifest.json").read_text())
try:
    actual = subprocess.check_output([sqlc, "version"], text=True).strip()
except (OSError, subprocess.CalledProcessError) as error:
    raise SystemExit(f"Pinned sqlc missing; run ./scripts/install-database-tools.sh ({error})")
if actual != "v" + version:
    raise SystemExit(f"sqlc version mismatch: expected v{version}, got {actual}")
if not (root / "sqlc.yaml").is_file():
    raise SystemExit("sqlc.yaml missing; claimed generated files cannot be verified")
with tempfile.TemporaryDirectory(prefix="columbo-sqlc-check-") as temporary:
    # sqlc config/SQL paths resolve relative to its config. A copy prevents even
    # failed generation or the verification itself from altering the worktree.
    copy = Path(temporary) / "source"
    shutil.copytree(root, copy, ignore=shutil.ignore_patterns(
        ".git", ".tools", "__pycache__", "*.sqlite", "*.sqlite-*", "columbo"))
    # Never let stale copied outputs count as freshly generated evidence. Config
    # changes that redirect output or stop emitting Querier must fail, too.
    for name in manifest["generated"]:
        (copy / name).unlink(missing_ok=True)
    generated = subprocess.run([sqlc, "generate", "-f", "sqlc.yaml"], cwd=copy)
    if generated.returncode:
        raise SystemExit("sqlc generate failed; generated output could not be verified")
    different = []
    for name in manifest["generated"]:
        before, after = root / name, copy / name
        if not before.is_file() or not after.is_file() or before.read_bytes() != after.read_bytes():
            different.append(name)
    expected = {manifest["lifecycle"], *manifest["generated"]}
    for path in (copy / "internal/snapshotdb").rglob("*.go"):
        name = path.relative_to(copy).as_posix()
        if name not in expected:
            different.append(name)
    if different:
        for name in sorted(set(different)):
            print(f"{name}: sqlc output differs; regenerate with pinned sqlc v{version}")
        sys.exit(1)
print(f"sqlc v{version} generated output: clean")
