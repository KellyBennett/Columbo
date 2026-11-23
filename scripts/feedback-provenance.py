#!/usr/bin/env python3
"""Retain the checkout and tool identity used for a feedback analysis."""
import argparse
import hashlib
import json
import os
import sqlite3
from pathlib import Path
import subprocess


def git(root, *args):
    return subprocess.check_output(
        ["git", "-C", str(root), *args], text=True, stderr=subprocess.DEVNULL
    ).strip()


def provenance(root, analyzer, config, environment, evidence=None):
    result = {"version": 1, "clean": False}
    try:
        result["analyzed_commit"] = git(root, "rev-parse", "HEAD")
        result["parents"] = git(root, "show", "-s", "--format=%P", "HEAD").split()
        result["clean"] = not git(root, "status", "--porcelain", "--untracked-files=all")
    except (OSError, subprocess.CalledProcessError) as error:
        result["git_gap"] = str(error)
    event_path = environment.get("GITHUB_EVENT_PATH")
    if event_path:
        event = json.loads(Path(event_path).read_text())
        pull = event.get("pull_request", {})
        result["head_commit"] = pull.get("head", {}).get("sha", "")
        result["base_commit"] = pull.get("base", {}).get("sha", "")
    result["event_commit"] = environment.get("GITHUB_SHA", "")
    result["module"] = environment.get("COLUMBO_WORKING_DIRECTORY", ".")
    result["run_id"] = environment.get("GITHUB_RUN_ID", "")
    result["run_attempt"] = environment.get("GITHUB_RUN_ATTEMPT", "")
    result["staged"] = environment.get("COLUMBO_STAGED", "true")
    result["analyzer_sha256"] = hashlib.sha256(Path(analyzer).read_bytes()).hexdigest()
    result["analyzer_version"] = subprocess.check_output([str(analyzer), "--version"], text=True).strip()
    result["go_version"] = subprocess.check_output(["go", "version"], text=True).strip()
    config_path = root / (config or ".columbo.yml")
    result["config_path"] = config or ".columbo.yml"
    if config_path.is_file():
        result["config_text"] = config_path.read_text()
        result["config_sha256"] = hashlib.sha256(config_path.read_bytes()).hexdigest()
    if evidence is not None and (evidence / "report.sqlite").is_file():
        uri = (evidence / "report.sqlite").resolve().as_uri() + "?mode=ro&immutable=1"
        with sqlite3.connect(uri, uri=True) as db:
            result["cases"] = [row[0] for row in db.execute("SELECT id FROM cases ORDER BY id")]
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--working-directory", required=True, type=Path)
    parser.add_argument("--evidence", required=True, type=Path)
    parser.add_argument("--analyzer", required=True, type=Path)
    parser.add_argument("--config", default="")
    args = parser.parse_args()
    data = provenance(args.working_directory, args.analyzer, args.config, os.environ, args.evidence)
    (args.evidence / "feedback-provenance.json").write_text(json.dumps(data, indent=2)+"\n")


if __name__ == "__main__":
    main()
