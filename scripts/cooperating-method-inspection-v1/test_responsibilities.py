import argparse
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def sha(data):
    return hashlib.sha256(data).hexdigest()


def observed(aid, operation, root=None):
    trace = []
    original = aid.require

    def checked(condition, message):
        trace.append([bool(condition), message])
        original(condition, message)

    aid.require = checked
    try:
        result = {"output": operation()}
    except Exception as exc:
        result = {"exception": type(exc).__name__, "message": str(exc)}
    finally:
        aid.require = original
    result["validation_order"] = trace
    if root and "output" in result and isinstance(result["output"], tuple):
        for key in ("facts_sha256", "scan_receipt_sha256"):
            result["output"][1][key] = "<TEMP_" + key.upper() + ">"
    serialized = json.dumps(result, ensure_ascii=False, separators=(",", ":"))
    return serialized.replace(str(root), "<TEMP_ROOT>") if root else serialized


def checkpoint_inputs(check, root, mode):
    witness = {"path": "m.go", "start_offset": 0, "end_offset": 4, "range_sha256": "range", "evidence_ids": ["case"]}
    facts = {"declarations": [witness]}
    rows = [{**witness, "case_id": "case"}]
    manifest = {"sha256": "source", "files": [{"path": "m.go", "sha256": "source-file"}]}
    item = {"concrete_burden": "boundary obligation", "witnesses": [witness]}
    value = {"version": 1, "source_identity": "source", "preferred_candidate": copy.deepcopy(item), "whole_owner_competitor": copy.deepcopy(item), "comparison_reason": "source comparison", "competitor_selection_reason": "full owner compared", "selected_investigation": "preferred_candidate", "disposition": "retain", "next_step": "review finite claims", "characterization": {"status": "passed", "obligation_id": "target", "observable_boundary": "observable result", "next_step": "execute oracle", "attempts": []}}
    state = value["characterization"]
    artifacts = {}
    for index, exit_code in enumerate((1, 0)):
        oracle = b"target oracle"
        files = copy.deepcopy(manifest["files"]) + [{"path": "oracle_test.go", "sha256": sha(oracle)}]
        receipt = {"status": "child_exited", "child_exit": exit_code, "command": ["go", "test", "-run", "Target"], "source_before": [{"files": files}], "source_after": [{"files": copy.deepcopy(files)}], "active_seconds": index + 2, "queue_seconds": index + 3}
        artifacts[index] = [oracle, receipt]
        state["attempts"].append({"obligation_id": "target", "interpretation": "finite oracle assertions"})
    last = state["attempts"][-1]
    receipt = artifacts[1][1]
    if mode == "planned":
        state.update(status="planned", attempts=[])
    elif mode == "blocked":
        state.update(status="blocked", attempts=[])
    elif mode == "failed":
        state.update(status="failed", attempts=state["attempts"][:1])
    elif mode == "attempted":
        state["status"] = "attempted"
    elif mode == "stale-checkpoint-and-missing-competitor":
        value["source_identity"] = "stale"
        del value["whole_owner_competitor"]
    elif mode == "missing-competitor":
        del value["whole_owner_competitor"]
    elif mode == "stale-candidate":
        value["whole_owner_competitor"]["witnesses"][0]["range_sha256"] = "stale"
    elif mode == "unbound-evidence":
        value["whole_owner_competitor"]["witnesses"][0]["evidence_ids"] = ["unrelated"]
    elif mode == "invalid-status":
        state["status"] = "invalid"
    elif mode == "planned-with-attempts":
        state["status"] = "planned"
    elif mode == "no-attempts":
        state["attempts"] = []
    elif mode == "missing-obligation":
        del state["obligation_id"]
    elif mode == "unrelated-obligation-and-missing-oracle":
        last["obligation_id"] = "unrelated"
    elif mode == "nonterminal-and-stale-source":
        receipt["status"] = "running"
        receipt["source_before"] = []
    elif mode == "missing-command-and-stale-source":
        receipt["command"] = []
        receipt["source_before"] = []
    elif mode == "missing-interpretation":
        del last["interpretation"]
    elif mode == "stale-latest-source":
        receipt["source_before"][0]["files"][0]["sha256"] = "stale"
    elif mode == "ambiguous-source":
        receipt["source_before"].append(copy.deepcopy(receipt["source_before"][0]))
    elif mode == "unbound-oracle-and-changed-source":
        receipt["source_before"][0]["files"] = copy.deepcopy(manifest["files"])
        receipt["source_after"] = []
    elif mode == "changed-execution-source":
        receipt["source_after"] = []
    elif mode == "earlier-stale-source-later-pass":
        artifacts[0][1]["source_before"] = []
    elif mode == "earlier-nonterminal-later-pass":
        artifacts[0][1]["status"] = "running"
    elif mode == "passed-latest-failed":
        receipt["child_exit"] = 1
    elif mode == "failed-latest-passed":
        state["status"] = "failed"
    elif mode == "missing-cost":
        del receipt["active_seconds"]
    elif mode == "missing-receipt-key":
        del receipt["source_before"]
    if mode in ("unrelated-latest-pass", "versioned-replacement", "replacement-no-reason", "replacement-wrong-identity"):
        receipt["command"] = ["go", "test", "-run", "Replacement"]
        if mode != "unrelated-latest-pass":
            last["replaces"] = {"oracle_sha256": sha(artifacts[0][0]), "command_sha256": sha(json.dumps(artifacts[0][1]["command"], separators=(",", ":")).encode())}
            if mode != "replacement-no-reason":
                last["replacement_reason"] = "versioned replacement remains subject to semantic review"
            if mode == "replacement-wrong-identity":
                last["replaces"]["oracle_sha256"] = "unrelated"
    if mode == "versioned-oracle-replacement":
        artifacts[1][0] = b"replacement oracle"
        receipt["source_before"][0]["files"][-1]["sha256"] = sha(artifacts[1][0])
        receipt["source_after"] = copy.deepcopy(receipt["source_before"])
        last["replaces"] = {"oracle_sha256": sha(artifacts[0][0]), "command_sha256": sha(json.dumps(artifacts[0][1]["command"], separators=(",", ":")).encode())}
        last["replacement_reason"] = "explicit replacement"
    for index, attempt in enumerate(state["attempts"]):
        oracle, item_receipt = artifacts[index]
        for kind, data in (("oracle", oracle), ("receipt", json.dumps(item_receipt).encode())):
            name = f"{kind}-{index}.json"
            (root / name).write_bytes(data)
            attempt[kind] = {"path": name, "sha256": sha(data)}
    if mode in ("missing-oracle", "unrelated-obligation-and-missing-oracle"):
        (root / last["oracle"]["path"]).unlink()
    if mode == "missing-receipt":
        (root / last["receipt"]["path"]).unlink()
    if mode == "stale-oracle":
        (root / last["oracle"]["path"]).write_bytes(b"stale")
    if mode == "escaping-artifact":
        last["oracle"]["path"] = "../escape"
    return lambda: check.validate(value, facts, rows, manifest, root)


def navigation_inputs(mode):
    def decl(name, receiver, path="m.go", start=0, kind="method"):
        return {"path": path, "start_offset": start, "end_offset": start + 4, "symbol": name, "kind": kind, "receiver_key": receiver, "calls": []}

    def field(name, key, embedded=False):
        return {"names": [name], "type_key": key, "type_text": key, "embedded": embedded}

    declarations = [decl("Z", "p.Z"), decl("A", "p.A", start=4), decl("Generated", "p.Z", "generated.go"), decl("Free", "", start=8, kind="function")]
    types = [{"key": "p.Z", "alias": False, "fields": [field("mid", "p.Mid", True), field("held", "p.Held"), field("unknown", "foreign.External")]}, {"key": "p.A", "alias": False, "fields": []}, {"key": "p.Mid", "alias": False, "fields": [field("cycle", "p.Z", True)]}, {"key": "p.Held", "alias": False, "fields": [field("hidden", "p.Hidden")]}, {"key": "p.Hidden", "alias": False, "fields": []}]
    declarations[0]["calls"] = [{"name": "local", "target": {"path": "m.go", "start_offset": 40}}, {"name": "unknown", "reason": "interface dispatch unresolved"}]
    declarations[2]["calls"] = [{"name": "generated-local", "target": {"path": "generated.go", "start_offset": 10}}]
    facts = {"declarations": declarations, "types": types, "files": [{"path": "m.go", "ordinary_target": True}, {"path": "generated.go", "ordinary_target": False}], "limits": ["one hop only", "other build variants excluded"]}
    rows = [{k: d[k] for k in ("path", "start_offset", "end_offset", "symbol")} | {"case_id": f"c{i}", "source_role": "primary"} for i, d in enumerate(declarations)]
    rows.insert(1, {**rows[0], "source_role": "peer:helper"})
    rows.insert(2, copy.deepcopy(rows[0]))
    if mode == "ambiguous-range":
        declarations.append(copy.deepcopy(declarations[0]))
    elif mode == "symbol-mismatch":
        rows[0]["symbol"] = "stale"
    elif mode == "missing-range":
        rows[0]["end_offset"] += 1
    elif mode == "unresolved-receiver":
        declarations[0]["receiver_key"] = "missing"
    elif mode == "empty-receiver":
        declarations[0]["receiver_key"] = ""
    elif mode == "ambiguous-receiver":
        types.append(copy.deepcopy(types[0]))
    elif mode == "alias-does-not-duplicate":
        types.append({**types[0], "alias": True})
    elif mode == "outside-before-ambiguity":
        facts["files"][0]["ordinary_target"] = False
        declarations.append(copy.deepcopy(declarations[0]))
    elif mode == "free-before-unresolved-receiver":
        declarations[0]["kind"] = "function"
        declarations[0]["receiver_key"] = "missing"
    elif mode == "missing-row-offset":
        del rows[0]["start_offset"]
    elif mode == "empty":
        rows = []
    return rows, [{"id": "excluded", "reasons": ["PASS", "prose-comment"]}], facts


def binding_inputs(aid, root, mode):
    source = root / "source"
    source.mkdir()
    (source / "m.go").write_bytes(b"package p")
    (source / ".columbo.yml").write_bytes(b"config")
    files = [{"path": p.name, "sha256": sha(p.read_bytes())} for p in sorted(source.iterdir())]
    manifest = {"files": files, "sha256": aid.manifest_identity(files)}
    facts = {"version": 1, "environment": {"GOOS": "linux"}, "patterns": ["./..."], "config_file": ".columbo.yml", "config_sha256": sha(b"config"), "config": {}, "root": str(source), "files": [{"path": "m.go", "sha256": sha(b"package p")}], "declarations": [], "types": []}
    receipt = {"status": "child_exited", "child_exit": 1, "command": ["columbo", "--output", "report.sqlite"], "cwd": str(source), "environment": facts["environment"].copy(), "source_before": [{**manifest, "root": str(source)}], "source_after": [{**manifest, "root": str(source)}]}
    binding = {"source_identity": manifest["sha256"], "database_hash_provenance": "current-observation", "database_path_at_scan": "report.sqlite", "patterns": ["./..."], "configuration": {}, "scope_provenance": "external source receipt"}
    if mode == "version-before-hash":
        facts["version"] = 2
    elif mode == "source-before-status":
        binding["source_identity"] = "stale"
        receipt["status"] = "running"
    elif mode == "provenance":
        binding["database_hash_provenance"] = "unproven"
    elif mode == "nonterminal":
        receipt["status"] = "running"
    elif mode == "staged-before-output":
        receipt["command"] = ["columbo", "--staged"]
    elif mode == "output":
        binding["database_path_at_scan"] = "different.sqlite"
    elif mode in ("source-before", "source-after"):
        receipt[mode.replace("-", "_")] = []
    elif mode == "environment-before-patterns":
        receipt["environment"]["GOOS"] = "windows"
        binding["patterns"] = ["./other"]
    elif mode == "patterns":
        binding["patterns"] = ["./other"]
    elif mode == "config-path":
        facts["config_file"] = "other.yml"
    elif mode == "configuration":
        binding["configuration"] = {"other": True}
    elif mode == "scope-provenance":
        binding["scope_provenance"] = ""
    elif mode == "stale-source":
        (source / "m.go").write_bytes(b"package stale")
    elif mode == "extra-source":
        (source / "extra.go").write_bytes(b"package p")
    elif mode == "config-hash":
        facts["config_sha256"] = "stale"
    elif mode == "duplicate-facts-file":
        facts["files"] *= 2
    elif mode == "facts-scope":
        facts["files"] = []
    elif mode == "witness-hash":
        facts["declarations"] = [{"path": "m.go", "file_sha256": sha(b"package p"), "range_sha256": "stale", "start_offset": 0, "end_offset": 4}]
    values = {"database": b"fixed database bytes", "facts": json.dumps(facts).encode(), "manifest": json.dumps(manifest).encode(), "scan_receipt": json.dumps(receipt).encode()}
    for name, data in values.items():
        (root / name).write_bytes(data)
        binding[name + "_sha256"] = sha(data)
    if mode in ("database-hash", "facts-hash", "manifest-hash", "scan_receipt-hash", "version-before-hash"):
        name = "database" if mode == "version-before-hash" else mode.removesuffix("-hash")
        binding[name + "_sha256"] = "stale"
    (root / "binding").write_text(json.dumps(binding))
    return lambda: aid.verify_binding(root / "database", root / "facts", root / "manifest", root / "binding", root / "scan_receipt")


def outcomes(source):
    aid = load("contract_inspect", source / "inspect.py")
    check = load("contract_checkpoint", source / "checkpoint.py")
    result = {}
    checkpoint_modes = ("passed", "planned", "blocked", "failed", "attempted", "stale-checkpoint-and-missing-competitor", "missing-competitor", "stale-candidate", "unbound-evidence", "invalid-status", "planned-with-attempts", "no-attempts", "missing-obligation", "unrelated-obligation-and-missing-oracle", "missing-oracle", "missing-receipt", "stale-oracle", "escaping-artifact", "nonterminal-and-stale-source", "missing-command-and-stale-source", "missing-interpretation", "stale-latest-source", "ambiguous-source", "unbound-oracle-and-changed-source", "changed-execution-source", "earlier-stale-source-later-pass", "earlier-nonterminal-later-pass", "passed-latest-failed", "failed-latest-passed", "missing-cost", "missing-receipt-key", "unrelated-latest-pass", "versioned-replacement", "replacement-no-reason", "replacement-wrong-identity", "versioned-oracle-replacement")
    for mode in checkpoint_modes:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            result["checkpoint/" + mode] = observed(check.aid, checkpoint_inputs(check, root, mode), root)
    navigation_modes = ("full", "ambiguous-range", "symbol-mismatch", "missing-range", "unresolved-receiver", "empty-receiver", "ambiguous-receiver", "alias-does-not-duplicate", "outside-before-ambiguity", "free-before-unresolved-receiver", "missing-row-offset", "empty")
    for mode in navigation_modes:
        inputs = navigation_inputs(mode)
        original = copy.deepcopy(inputs)
        result["navigation/" + mode] = observed(aid, lambda: aid.bind(*inputs))
        if inputs != original:
            raise AssertionError("navigation input mutated: " + mode)
    binding_modes = ("bound", "database-hash", "facts-hash", "manifest-hash", "scan_receipt-hash", "version-before-hash", "source-before-status", "provenance", "nonterminal", "staged-before-output", "output", "source-before", "source-after", "environment-before-patterns", "patterns", "config-path", "configuration", "scope-provenance", "stale-source", "extra-source", "config-hash", "duplicate-facts-file", "facts-scope", "witness-hash")
    for mode in binding_modes:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            result["binding/" + mode] = observed(aid, binding_inputs(aid, root, mode), root)
    return result


class ResponsibilityContractTests(unittest.TestCase):
    def test_frozen_v1_serialized_outputs_failures_and_validation_order(self):
        root = Path(__file__).parent
        expected = json.loads((root / "test_responsibilities.expected.json").read_text())
        actual = outcomes(root)
        self.assertEqual(list(actual), list(expected))
        for name in expected:
            with self.subTest(name=name):
                self.assertEqual(actual[name], expected[name])


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    with args.output.open("x") as stream:
        json.dump(outcomes(args.source), stream, indent=2)
        stream.write("\n")
