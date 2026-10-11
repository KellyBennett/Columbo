import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("checkpoint", Path(__file__).with_name("checkpoint.py"))
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)


class CheckpointTests(unittest.TestCase):
    def setUp(self):
        self.witness = {"path": "m.go", "start_offset": 0, "end_offset": 4, "range_sha256": "source-range", "evidence_ids": ["c"]}
        self.facts = {"declarations": [self.witness]}
        self.rows = [{**self.witness, "case_id": "c"}]
        self.manifest = {"sha256": "source", "files": [{"path": "m.go", "sha256": "source-file"}]}
        item = {"concrete_burden": "Concrete source obligation", "witnesses": [self.witness]}
        self.value = {"version": 1, "source_identity": "source", "preferred_candidate": item, "whole_owner_competitor": copy.deepcopy(item), "comparison_reason": "Source-based comparison", "competitor_selection_reason": "Compared remaining full-owner obligations", "selected_investigation": "whole_owner_competitor", "disposition": "pre-attempt", "next_step": "Characterize public boundary", "characterization": {"status": "planned", "obligation_id": "public-behavior", "observable_boundary": "observable behavior", "next_step": "Run public oracle", "attempts": []}}

    def validate(self, root="."):
        return check.validate(self.value, self.facts, self.rows, self.manifest, root)

    def test_planned_does_not_claim_executed_oracle(self):
        result = self.validate()
        self.assertEqual(result["characterization_status"], "planned")
        self.assertEqual(result["semantic_priority"], "not adjudicated")
        self.assertEqual(result["behavior_preservation"], "not proven")

    def test_missing_competitor_is_rejected(self):
        del self.value["whole_owner_competitor"]
        with self.assertRaisesRegex(ValueError, "competitor required"):
            self.validate()

    def test_stale_source_and_unbound_evidence_are_rejected(self):
        self.value["source_identity"] = "old"
        with self.assertRaisesRegex(ValueError, "binding mismatch"):
            self.validate()
        self.value["source_identity"] = "source"
        self.value["whole_owner_competitor"]["witnesses"][0]["range_sha256"] = "stale"
        with self.assertRaisesRegex(ValueError, "stale"):
            self.validate()

    def test_declared_oracle_must_exist_even_with_successful_suite(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(ValueError, "missing or stale artifact"):
                check.artifact({"path": "missing_oracle_test.go", "sha256": "claimed"}, Path(tmp))

    def test_pass_needs_executed_source_bound_oracle(self):
        self.value["characterization"]["status"] = "passed"
        with self.assertRaisesRegex(ValueError, "bound artifact"):
            self.validate()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            oracle = b"an actual oracle"
            (root / "oracle.go").write_bytes(oracle)
            receipt = {"status": "child_exited", "child_exit": 0, "command": ["go", "test"], "source_before": [{"files": self.manifest["files"]}], "source_after": [{"files": self.manifest["files"]}]}
            (root / "receipt.json").write_text(json.dumps(receipt))
            reference = lambda name: {"path": name, "sha256": check.aid.digest((root / name).read_bytes())}
            self.value["characterization"]["attempts"] = [{"obligation_id": "public-behavior", "oracle": reference("oracle.go"), "receipt": reference("receipt.json"), "interpretation": "finite assertions only"}]
            with self.assertRaisesRegex(ValueError, "oracle not bound"):
                self.validate(root)
            receipt["source_before"][0]["files"] = self.manifest["files"] + [{"path": "oracle_test.go", "sha256": check.aid.digest(oracle)}]
            receipt["source_after"] = copy.deepcopy(receipt["source_before"])
            (root / "receipt.json").write_text(json.dumps(receipt))
            self.value["characterization"]["attempts"][0]["receipt"] = reference("receipt.json")
            self.assertEqual(self.validate(root)["structural_and_binding_checks"], "passed")
            first = copy.deepcopy(self.value["characterization"]["attempts"][0])
            receipt["child_exit"] = 1
            receipt["active_seconds"] = 2
            receipt["queue_seconds"] = 3
            (root / "failure.json").write_text(json.dumps(receipt))
            first["receipt"] = reference("failure.json")
            self.value["characterization"]["attempts"].insert(0, first)
            result = self.validate(root)
            self.assertEqual([h["child_exit"] for h in result["attempt_history"]], [1, 0])
            receipt["child_exit"] = 0
            receipt["command"] = ["go", "test", "-run", "Unrelated"]
            (root / "unrelated.json").write_text(json.dumps(receipt))
            latest = self.value["characterization"]["attempts"][-1]
            latest["receipt"] = reference("unrelated.json")
            with self.assertRaisesRegex(ValueError, "unrelated latest pass"):
                self.validate(root)
            latest["replaces"] = {k: result["attempt_history"][0][k] for k in ("oracle_sha256", "command_sha256")}
            latest["replacement_reason"] = "Explicitly recorded replacement; semantic adequacy still requires review"
            self.assertEqual(self.validate(root)["structural_and_binding_checks"], "passed")
            receipt = copy.deepcopy(receipt)
            receipt["source_before"][0]["files"][0]["sha256"] = "stale-source"
            (root / "stale.json").write_text(json.dumps(receipt))
            latest["receipt"] = reference("stale.json")
            with self.assertRaisesRegex(ValueError, "source differs"):
                self.validate(root)



if __name__ == "__main__":
    unittest.main()
