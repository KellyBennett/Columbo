#!/usr/bin/env python3
"""Reject reachable repository packages omitted from the official lint targets."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
parser.add_argument("--json", type=Path)
args = parser.parse_args()
root = args.root.resolve()
# Include test dependencies: an independent test exception cannot add a hidden
# local package. These are official Go package metadata calls, not source parsing.
def packages(dependencies=False, directories=None):
    command = [os.environ.get("GO", "go"), "list", "-json"]
    if directories is not None:
        # Explicit directories plus -e retain metadata for entirely inactive
        # packages (for example cgo-only source at CGO_ENABLED=0).
        command.extend(["-e", *directories])
    else:
        command.append("-test")
        if dependencies:
            command.append("-deps")
        command.append("./...")
    try:
        result = subprocess.run(command, cwd=root, text=True, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, timeout=180)
    except (OSError, subprocess.TimeoutExpired) as error:
        raise SystemExit(f"Official Go package inventory unavailable: {error}")
    if result.returncode:
        raise SystemExit(f"Official Go package inventory failed:\n{result.stderr}")
    decoder, offset, values = json.JSONDecoder(), 0, []
    while offset < len(result.stdout):
        while offset < len(result.stdout) and result.stdout[offset].isspace():
            offset += 1
        if offset == len(result.stdout):
            break
        value, offset = decoder.raw_decode(result.stdout, offset)
        values.append(value)
    return values

targets = {Path(value["Dir"]).resolve() for value in packages(False) if value.get("Dir")}
issues, seen = [], set()
for value in packages(True):
    if not value.get("Dir"):
        continue
    directory = Path(value["Dir"]).resolve()
    if not directory.is_relative_to(root) or directory in targets or directory in seen:
        continue
    seen.add(directory)
    issues.append({"path": directory.relative_to(root).as_posix(), "line": 1,
                   "check": "database-package-inventory",
                   "message": "Reachable repository package is absent from ./... lint targets: " + value["ImportPath"]})
# Discover only the physical source package directories, without inspecting SQL
# or resolving Go symbols. Query all of them together using the official Go tool.
source_dirs = sorted({"./" + path.parent.relative_to(root).as_posix()
                      for path in root.rglob("*.go")
                      if not path.is_symlink() and not any(part in
                          {".git", ".tools", "vendor", "testdata"}
                          for part in path.relative_to(root).parts)})
for value in packages(directories=source_dirs) if source_dirs else []:
    if not value.get("Dir"):
        continue
    directory = Path(value["Dir"]).resolve()
    if not directory.is_relative_to(root):
        continue
    for field in ["IgnoredGoFiles", "CgoFiles"]:
        for filename in value.get(field, []):
            issues.append({"path": (directory / filename).relative_to(root).as_posix(), "line": 1,
                           "check": "database-package-inventory",
                           "message": "Unreviewed implicit source build variant (" + field + ")"})

if args.json:
    args.json.parent.mkdir(parents=True, exist_ok=True)
    args.json.write_text(json.dumps(issues, indent=2) + "\n")
for issue in issues:
    print(f'{issue["path"]}:1: {issue["message"]}')
if not issues:
    print("Database boundary reachable-package inventory: clean")
sys.exit(bool(issues))
