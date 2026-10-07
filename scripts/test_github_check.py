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

    def test_staged_projection_defers_legacy_cases_and_never_clears_terminal(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "staged.sqlite"
            shutil.copyfile(self.snapshot, path)
            with sqlite3.connect(path) as db:
                db.execute("INSERT INTO refactoring_stages VALUES ('untangle',0,'Untangle Behavior','Gather behavior','active',0,1)")
                db.execute("INSERT INTO refactoring_stages VALUES ('ownership',1,'Assign Ownership','Assign owners; no completion condition','locked',1,0)")
                db.execute("INSERT INTO stage_collectors VALUES ('untangle',0,'nested-field-decision',1)")
                db.execute("INSERT INTO advisory_groups VALUES ('stage-issue','nested-field-decision','Fixture','Gather behavior','Lexical evidence')")
                declaration = db.execute("SELECT id FROM declarations ORDER BY id LIMIT 1").fetchone()[0]
                db.execute("INSERT INTO advisory_sites VALUES ('stage-issue',0,?,'','')", (declaration,))
                db.execute("INSERT INTO advisory_receipts VALUES ('stage-issue',0,0,'condition','Fixture',1,1,0,1,'x')")
            annotations, summary = publisher.read_snapshot(path, 1)
            self.assertEqual(1, len(annotations))
            self.assertEqual("Columbo: nested-field-decision", annotations[0]["title"])
            self.assertIn("Untangle Behavior: active", summary)
            self.assertNotIn("11 failed", summary)
            with sqlite3.connect(path) as db:
                db.execute("DELETE FROM advisory_receipts WHERE group_id='stage-issue'")
                db.execute("DELETE FROM advisory_sites WHERE group_id='stage-issue'")
                db.execute("DELETE FROM advisory_groups WHERE id='stage-issue'")
                db.execute("UPDATE stage_collectors SET issue_count=0")
                db.execute("UPDATE refactoring_stages SET state='cleared',issue_count=0 WHERE id='untangle'")
                db.execute("UPDATE refactoring_stages SET state='active' WHERE id='ownership'")
            annotations, summary = publisher.read_snapshot(path, 0)
            self.assertEqual([], annotations)
            self.assertIn("Assign Ownership: active", summary)
            self.assertIn("Pending definition; this is not completion", summary)
            with self.assertRaisesRegex(ValueError, "stored stage issues"):
                publisher.read_snapshot(path, 1)

    def test_advisories_appear_in_summary_without_annotations(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "advisories.sqlite"
            shutil.copyfile(self.snapshot, path)
            with sqlite3.connect(path) as db:
                db.execute("INSERT INTO advisory_groups VALUES ('test-evidence','choice-set','fixture.Context.Items','Review ownership','Bounded evidence')")
            annotations, summary = publisher.read_snapshot(path, 1)
            self.assertEqual(11, len(annotations))
            self.assertIn("test-evidence", summary)
            self.assertIn("Review ownership", summary)

    def test_real_snapshot_preserves_findings_and_policy(self):
        before = self.snapshot.read_bytes()
        annotations, summary = publisher.read_snapshot(self.snapshot, 1)
        self.assertEqual(11, len(annotations))
        self.assertEqual(before, self.snapshot.read_bytes())
        self.assertIn("11 failed, 0 warned, 0 suppressed", summary)
        first = annotations[0]
        self.assertEqual(("all.go", 13, 25, "failure"),
                         (first["path"], first["start_line"], first["end_line"], first["annotation_level"]))
        self.assertIn("C-b10bccb46b", first["message"])
        self.assertIn("function-lines", first["message"])
        cosmetic = next(item for item in annotations if "cosmetic-extraction" in item["title"])
        self.assertIn("Policy CE-001", cosmetic["message"])
        self.assertIn("Review:", cosmetic["message"])
        self.assertIn("lead:", cosmetic["message"])

    def test_prose_comment_guidance_is_projected(self):
        annotations, _ = publisher.read_snapshot(self.snapshot, 1)
        finding = next(item for item in annotations if item["title"] == "Columbo: prose-comment")
        self.assertEqual(("comments.go", 3, "failure"),
                         (finding["path"], finding["start_line"], finding["annotation_level"]))
        self.assertIn("regression test that fails when the unwanted behavior is introduced", finding["message"])
        self.assertIn("Comments are not a reliable enforcement mechanism", finding["message"])

    def test_correlations_add_no_annotations_or_verdicts(self):
        before = publisher.read_snapshot(self.snapshot, 1)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "without-correlations.sqlite"
            shutil.copyfile(self.snapshot, path)
            with sqlite3.connect(path) as db:
                self.assertGreater(db.execute("SELECT COUNT(*) FROM correlations").fetchone()[0], 0)
                for table in ("correlation_evidence", "correlation_guidance", "correlation_role_candidates", "correlation_cases", "correlations"):
                    db.execute(f"DELETE FROM {table}")
            after = publisher.read_snapshot(path, 1)
        self.assertEqual(before, after)

    def test_selection_flow_and_policy_are_projected(self):
        annotations, _ = publisher.read_snapshot(self.snapshot, 1)
        finding = next(item for item in annotations if item["title"] == "Columbo: selection-use-coupling")
        for text in ("fixture.Sender", "fixture.Email", "fixture.SMS", "selection-decision selection.go:", "selection-origin selection.go:", "selection-flow selection.go:", "selected-message selection.go:", "Send(string) error", "Close() error", "Policy SUC-001", "limit >= 2"):
            self.assertIn(text, finding["message"])
        self.assertEqual("failure", finding["annotation_level"])

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
        self.assertEqual(10, len(annotations))
        self.assertEqual("warning", annotations[0]["annotation_level"])
        self.assertIn("9 failed, 1 warned, 1 suppressed", summary)
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
        self.assertEqual(11, len(annotations))
        self.assertIn("11 failed, 0 warned, 0 suppressed", summary)
        finding = next(item for item in annotations if "excessive-dependencies" in item["title"])
        self.assertIn("dependencies fixture.Dependencies: 6 (limit > 5)", finding["message"])
        self.assertNotIn("Dependency origins", finding["message"])

    def test_coordination_clues_are_projected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "coordination.sqlite"
            shutil.copyfile(self.snapshot, path)
            with sqlite3.connect(path) as db:
                case_id = db.execute("SELECT id FROM cases WHERE smell='repeated-variant-decision' LIMIT 1").fetchone()[0]
                ordinal = db.execute("SELECT MAX(ordinal)+1 FROM clues WHERE case_id=?", (case_id,)).fetchone()[0]
                for offset, kind in enumerate(("variant-shared-write", "variant-write-read")):
                    clue = db.execute("INSERT INTO clues(case_id,ordinal,kind,subject,value_type) VALUES (?,?,?,?,'list')", (case_id, ordinal+offset, kind, "source.go:10:20:n")).lastrowid
                    db.execute("INSERT INTO clue_values(clue_id,ordinal,value) VALUES (?,0,?)", (clue, "lexical evidence only; selector x; roots 10 -> 20"))
            annotations, _ = publisher.read_snapshot(path, 1)
        finding = next(item for item in annotations if "repeated-variant-decision" in item["title"] and "variant-shared-write" in item["message"])
        self.assertIn("variant-write-read", finding["message"])
        self.assertIn("lexical evidence only", finding["message"])

    def test_duplicate_locations_are_projected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "duplicates.sqlite"
            shutil.copyfile(self.snapshot, path)
            with sqlite3.connect(path) as db:
                case_id = db.execute("SELECT id FROM cases WHERE ordinal=0").fetchone()[0]
                db.execute("UPDATE cases SET smell='duplicate-code',verdict='WARN' WHERE id=?", (case_id,))
                receipts = db.execute("SELECT id FROM source_receipts WHERE case_id=? ORDER BY ordinal LIMIT 2", (case_id,)).fetchall()
                for receipt in receipts:
                    db.execute("UPDATE source_receipts SET kind='duplicate-fragment' WHERE id=?", receipt)
            annotations, _ = publisher.read_snapshot(path, 1)
        finding = next(item for item in annotations if item["title"] == "Columbo: duplicate-code")
        self.assertEqual("warning", finding["annotation_level"])
        self.assertEqual(2, finding["message"].count("Duplicate fragment: all.go:"))
        self.assertIn("(bytes ", finding["message"])

    def test_variant_domain_sites_and_policy_are_projected(self):
        annotations, _ = publisher.read_snapshot(self.snapshot, 1)
        finding = next(item for item in annotations if item["title"] == "Columbo: repeated-variant-decision")
        self.assertEqual("failure", finding["annotation_level"])
        for text in ("value:fixture.SourceKind", "repeated-variant-set", "variant-support", "variant-set", "Policy RVD-001", "in fixture.Fetch", "in fixture.Check", "factory", "visitor/adapter", "lead: If independent operations", "avoid: Adding wrapper objects"):
            self.assertIn(text, finding["message"])
        self.assertEqual(2, finding["message"].count("Variant decision: variants.go:"))

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
