"""Verify that feedback evidence identifies the checkout rather than guessing from event metadata."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location(
    "feedback_provenance", Path(__file__).with_name("feedback-provenance.py")
)
PROVENANCE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROVENANCE)


class FeedbackProvenanceTests(unittest.TestCase):
    def test_records_actual_checkout_config_and_cases_without_credentials(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            module = root / "module"
            module.mkdir()
            config = module / "settings.yml"
            config.write_text("limits:\n  dependencies: 7\n")
            subprocess.run(["git", "init", "-q", str(module)], check=True)
            subprocess.run(["git", "-C", str(module), "add", "."], check=True)
            subprocess.run(["git", "-C", str(module), "-c", "user.name=Test", "-c",
                            "user.email=test@example.invalid", "commit", "-qm", "fixture"], check=True)
            actual = PROVENANCE.git(module, "rev-parse", "HEAD")
            event = root / "event.json"
            event.write_text(json.dumps({"pull_request": {"head": {"sha": "head"}, "base": {"sha": "base"}}}))
            analyzer = root / "analyzer"
            analyzer.write_text("#!/bin/sh\necho 'columbo test'\n")
            analyzer.chmod(0o700)
            go = root / "go"
            go.write_text("#!/bin/sh\necho 'go version test'\n")
            go.chmod(0o700)
            evidence = root / "evidence"
            evidence.mkdir()
            with sqlite3.connect(evidence / "report.sqlite") as db:
                db.execute("CREATE TABLE cases(id TEXT)")
                db.execute("INSERT INTO cases VALUES ('C-1')")
            environment = {"GITHUB_EVENT_PATH": str(event), "GITHUB_SHA": "event-sha",
                           "GH_TOKEN": "must-not-be-recorded", "GITHUB_RUN_ID": "123"}
            with patch.dict(os.environ, {"PATH": str(root)+os.pathsep+os.environ["PATH"]}):
                result = PROVENANCE.provenance(module, analyzer, "settings.yml", environment, evidence)
                self.assertEqual(result["analyzed_commit"], actual)
                self.assertNotEqual(result["analyzed_commit"], environment["GITHUB_SHA"])
                self.assertEqual(result["head_commit"], "head")
                self.assertEqual(result["cases"], ["C-1"])
                self.assertTrue(result["clean"])
                self.assertEqual(result["config_sha256"], hashlib.sha256(config.read_bytes()).hexdigest())
                self.assertNotIn("must-not-be-recorded", json.dumps(result))
                config.write_text("changed\n")
                changed = PROVENANCE.provenance(module, analyzer, "settings.yml", environment, evidence)
                self.assertFalse(changed["clean"])


if __name__ == "__main__":
    unittest.main()
