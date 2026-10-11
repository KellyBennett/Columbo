import copy
import importlib.util
import json
from pathlib import Path
import sqlite3
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("aid", Path(__file__).with_name("inspect.py"))
aid = importlib.util.module_from_spec(spec)
spec.loader.exec_module(aid)


def declaration(name="Read", kind="method", receiver="p.Owner", path="model.go", start=0):
    return {"path": path, "start_offset": start, "end_offset": start + 10, "symbol": name, "kind": kind, "receiver_key": receiver, "calls": []}


def row(decl, role="primary", case="c"):
    return {k: decl[k] for k in ("path", "start_offset", "end_offset", "symbol")} | {"source_role": role, "case_id": case}


def fixture():
    return {"declarations": [declaration(), declaration("Short", start=10), declaration("Generated", path="generated.go")], "types": [{"key": "p.Owner", "alias": False, "fields": []}], "files": [{"path": "model.go", "ordinary_target": True}, {"path": "generated.go", "ordinary_target": False}], "limits": []}


class QueueTests(unittest.TestCase):
    def test_primary_peer_unsampled_siblings_and_dedup(self):
        facts = fixture()
        rows = [row(facts["declarations"][0]), row(facts["declarations"][1], "peer:helper")]
        result = aid.bind(rows, [], facts)
        self.assertEqual(result["eligible_associations"], 2)
        self.assertEqual(result["included_associations"], 2)
        self.assertEqual(len(result["receiver_seeds"]), 1)
        self.assertEqual(len(result["receiver_seeds"][0]["methods"]), 3)
        self.assertEqual(result["receiver_seeds"][0]["status"], "not-reached")

    def test_ambiguous_free_and_outside_never_seed(self):
        facts = fixture()
        facts["declarations"] += [declaration("Free", kind="function", start=20)]
        rows = [row(d) for d in facts["declarations"]]
        facts["declarations"].append(copy.deepcopy(facts["declarations"][0]))
        result = aid.bind(rows, [], facts)
        self.assertEqual(len(result["excluded_associations"]), 3)
        self.assertEqual(result["included_associations"], 1)

    def test_promoted_paths_preserve_diamond_and_held_boundary(self):
        def field(name, key, embedded=False):
            return {"names": [name], "type_key": key, "embedded": embedded, "type_text": key}
        leaf = {"key": "Leaf", "fields": [field("inner", "Hidden")]}
        owner = {"key": "Owner", "fields": [field("a", "Mid", True), field("b", "Mid", True), field("held", "Leaf")]}
        mid = {"key": "Mid", "fields": [field("leaf", "Leaf")]}
        found, unknown = aid.storage(owner, {"Mid": [mid], "Leaf": [leaf]})
        self.assertFalse(unknown)
        self.assertEqual({tuple(f["field_path"]) for f in found}, {("a",), ("a", "leaf"), ("b",), ("b", "leaf"), ("held",)})

    def test_sql_eligibility_keeps_every_primary_and_peer_role(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "report.sqlite"
            with sqlite3.connect(path) as db:
                db.executescript("CREATE TABLE report(schema_version); INSERT INTO report VALUES(8); CREATE TABLE files(id,path); INSERT INTO files VALUES(1,'m.go'); CREATE TABLE declarations(id,file_id,symbol,start_line,end_line,start_offset,end_offset); INSERT INTO declarations VALUES(1,1,'Short',1,1,0,4),(2,1,'Peer',2,2,5,9); CREATE TABLE cases(id,ordinal,smell,verdict,suppressed,primary_declaration_id); CREATE TABLE case_declarations(case_id,declaration_id,role); INSERT INTO cases VALUES('a',0,'long-function','WARN',0,1),('b',1,'prose-comment','FAIL',0,1),('c',2,'long-function','FAIL',1,1),('d',3,'long-function','PASS',0,1); INSERT INTO case_declarations VALUES('a',2,'helper'),('a',1,'also-primary'),('b',2,'helper');")
            rows, excluded = aid.select(path)
            self.assertEqual(len(rows), 3)
            self.assertEqual({r["source_role"] for r in rows}, {"primary", "peer:helper", "peer:also-primary"})
            self.assertEqual(len(excluded), 3)

    def test_manifest_rejects_stale_extra_symlink_escape(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "m.go").write_text("package p")
            entries = [{"path": "m.go", "sha256": aid.digest(b"package p")}]
            manifest = {"files": entries, "sha256": aid.manifest_identity(entries)}
            self.assertEqual(aid.source_files(root, manifest)["m.go"], entries[0]["sha256"])
            (root / "m.go").write_text("package q")
            with self.assertRaisesRegex(ValueError, "stale source"):
                aid.source_files(root, manifest)
            entries[0]["path"] = "../escape"
            manifest["sha256"] = aid.manifest_identity(entries)
            with self.assertRaisesRegex(ValueError, "unsafe"):
                aid.source_files(root, manifest)

    def test_scan_scope_rejects_unbound_configuration(self):
        receipt = {"cwd": "/source", "source_before": [{"root": "/source"}]}
        self.assertEqual(aid.scan_scope(["columbo", "--no-history", "--output", "result.sqlite", "./..."], receipt), (["./..."], ".columbo.yml"))
        with self.assertRaisesRegex(ValueError, "not bound"):
            aid.scan_scope(["columbo", "--config", "/other/config.yml"], receipt)

    def test_nested_witness_hash_is_checked(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            data = b"package p"
            (root / "m.go").write_bytes(data)
            span = {"path": "m.go", "file_sha256": aid.digest(data), "range_sha256": "bad", "start_offset": 0, "end_offset": 2}
            with self.assertRaisesRegex(ValueError, "witness hash"):
                aid.verify_witnesses({"methods": [{"calls": [{"target": span}]}]}, root, {"m.go": aid.digest(data)})


if __name__ == "__main__":
    unittest.main()
