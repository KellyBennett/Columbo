"""Regression tests against a real snapshot and an independent Checks API sink."""

import copy
import importlib.util
import os
from pathlib import Path
import shutil
import sqlite3
import tempfile
import unittest
from unittest.mock import patch
from urllib.error import HTTPError

spec = importlib.util.spec_from_file_location("github_check", Path(__file__).with_name("github-check.py"))
publisher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publisher)


class ChecksSink:
    def __init__(self, fail_at=None):
        self.calls = []
        self.fail_at = fail_at

    def request(self, method, suffix, payload):
        self.calls.append((method, suffix, copy.deepcopy(payload)))
        if len(self.calls) == self.fail_at:
            raise RuntimeError("permission or network failure")
        return {"id": 42, "html_url": "https://github.com/example/repo/runs/42"}


class PublisherTests(unittest.TestCase):
    def setUp(self):
        self.snapshot = Path(os.environ["COLUMBO_TEST_SNAPSHOT"])

    def test_real_snapshot_preserves_findings_and_policy(self):
        before = self.snapshot.read_bytes()
        annotations, summary = publisher.read_snapshot(self.snapshot, 1)
        self.assertEqual(8, len(annotations))
        self.assertEqual(before, self.snapshot.read_bytes())
        self.assertIn("8 failed, 0 warned, 0 suppressed", summary)
        first = annotations[0]
        self.assertEqual(("all.go", 13, 25, "failure"),
                         (first["path"], first["start_line"], first["end_line"], first["annotation_level"]))
        self.assertIn("C-b10bccb46b", first["message"])
        self.assertIn("function-lines", first["message"])
        cosmetic = next(item for item in annotations if "cosmetic-extraction" in item["title"])
        self.assertIn("Policy CE-001", cosmetic["message"])
        self.assertIn("Review:", cosmetic["message"])
        self.assertIn("lead:", cosmetic["message"])

    def test_dependency_origins_are_projected(self):
        annotations, _ = publisher.read_snapshot(self.snapshot, 1)
        finding = next(item for item in annotations if "excessive-dependencies" in item["title"])
        self.assertIn("dependencies fixture.Dependencies: 6 (limit > 5)", finding["message"])
        self.assertIn("Dependency origins (lists overlap; each identity counts once)", finding["message"])
        self.assertIn("Declared or constructed types:", finding["message"])
        self.assertIn("Does the caller know details its dependency could own?", finding["message"])
        self.assertNotIn("may coordinate responsibilities with separate ownership", finding["message"])

    def test_stored_warn_and_suppression_are_respected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "saved.sqlite"
            shutil.copyfile(self.snapshot, path)
            with sqlite3.connect(path) as db:
                db.execute("UPDATE cases SET verdict='WARN' WHERE ordinal=0")
                db.execute("UPDATE cases SET suppressed=1 WHERE ordinal=1")
            annotations, summary = publisher.read_snapshot(path, 1)
        self.assertEqual(7, len(annotations))
        self.assertEqual("warning", annotations[0]["annotation_level"])
        self.assertIn("6 failed, 1 warned, 1 suppressed", summary)
        self.assertFalse(any("C-ad03ea5957" in item["message"] for item in annotations))

    def test_snapshot_without_dependency_origins_is_readable(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "old.sqlite"
            shutil.copyfile(self.snapshot, path)
            with sqlite3.connect(path) as db:
                ids = "SELECT id FROM clues WHERE kind LIKE 'dependency-use-%'"
                db.execute(f"DELETE FROM clue_receipts WHERE clue_id IN ({ids})")
                db.execute(f"DELETE FROM clue_values WHERE clue_id IN ({ids})")
                db.execute(f"DELETE FROM clues WHERE id IN ({ids})")
            annotations, summary = publisher.read_snapshot(path, 1)
        self.assertEqual(8, len(annotations))
        self.assertIn("8 failed, 0 warned, 0 suppressed", summary)
        finding = next(item for item in annotations if "excessive-dependencies" in item["title"])
        self.assertIn("dependencies fixture.Dependencies: 6 (limit > 5)", finding["message"])
        self.assertNotIn("Dependency origins", finding["message"])

    def test_batches_do_not_drop_or_duplicate_findings(self):
        for count in (0, 1, 50, 51, 155):
            with self.subTest(count=count):
                sink = ChecksSink()
                annotations = [{"message": f"case-{i}"} for i in range(count)]
                publication = self.publication(sink, annotations)
                publication.publish()
                sent = [a for _, _, body in sink.calls for a in body["output"]["annotations"]]
                self.assertEqual(annotations, sent)
                self.assertTrue(all(len(body["output"]["annotations"]) <= 50 for _, _, body in sink.calls))
                self.assertEqual(count, publication.published)
                self.assertEqual("POST", sink.calls[0][0])
                self.assertEqual("a" * 40, sink.calls[0][2]["head_sha"])
                self.assertEqual("completed", sink.calls[-1][2]["status"])
                self.assertEqual("failure", sink.calls[-1][2]["conclusion"])
                self.assertTrue(all(body["status"] == "in_progress" for _, _, body in sink.calls[:-1]))

    def test_partial_publication_fails_without_retrying_append(self):
        sink = ChecksSink(fail_at=2)
        publication = self.publication(sink, [{"message": str(i)} for i in range(155)])
        with self.assertRaises(RuntimeError):
            publication.publish()
        self.assertEqual(50, publication.published)
        self.assertEqual(3, len(sink.calls))
        self.assertNotIn("annotations", sink.calls[-1][2]["output"])
        self.assertEqual("failure", sink.calls[-1][2]["conclusion"])
        self.assertIn("50/155", sink.calls[-1][2]["output"]["summary"])

    def test_exit_2_does_not_read_or_publish_stale_report(self):
        annotations, summary = publisher.read_snapshot("does-not-exist.sqlite", 2)
        self.assertEqual([], annotations)
        self.assertIn("Analysis failed", summary)
        with self.assertRaises(ValueError):
            publisher.read_snapshot(self.snapshot, 0)

    def test_successful_empty_check(self):
        sink = ChecksSink()
        publication = self.publication(sink, [], exit_code=0)
        publication.publish()
        self.assertEqual("success", sink.calls[0][2]["conclusion"])
        self.assertEqual(0, publication.evidence()["published_annotations"])

    def test_missing_and_corrupt_snapshots_fail(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "missing.sqlite"
            with self.assertRaises(FileNotFoundError):
                publisher.read_snapshot(path, 1)
            path.write_text("not a database")
            with self.assertRaises(sqlite3.Error):
                publisher.read_snapshot(path, 1)

    def test_api_error_does_not_disclose_token(self):
        api = publisher.ChecksAPI("example/repo", "secret-token")
        error = HTTPError(api.base, 403, "forbidden", {}, None)
        with patch.object(publisher, "urlopen", side_effect=error):
            with self.assertRaisesRegex(RuntimeError, "HTTP 403") as caught:
                api.request("POST", "", {})
        self.assertNotIn("secret-token", str(caught.exception))

    def test_message_limits_are_utf8_byte_limits(self):
        message = publisher.limited("🙂" * 20000)
        self.assertLessEqual(len(message.encode()), 65536)
        self.assertIn("Truncated", message)

    def publication(self, sink, annotations, exit_code=1):
        return publisher.Publication(sink, "Columbo findings", "a" * 40,
                                     "https://github.com/example/repo/actions/runs/1",
                                     exit_code, "stored verdicts", annotations)


if __name__ == "__main__":
    unittest.main()
