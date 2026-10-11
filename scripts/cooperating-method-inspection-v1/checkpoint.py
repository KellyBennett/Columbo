import argparse
import importlib.util
import json
from pathlib import Path

spec = importlib.util.spec_from_file_location("inspection_aid", Path(__file__).with_name("inspect.py"))
aid = importlib.util.module_from_spec(spec)
spec.loader.exec_module(aid)


def artifact(reference, root):
    path = Path(root) / reference["path"]
    aid.require(path.resolve().is_relative_to(Path(root).resolve()), "artifact escapes evidence root")
    aid.require(path.is_file() and aid.digest(path.read_bytes()) == reference["sha256"], "missing or stale artifact")
    return path


def candidate(value, facts, associations):
    aid.require(value.get("concrete_burden") and value.get("witnesses"), "candidate burden and exact witnesses required")
    for witness in value["witnesses"]:
        keys = ("path", "start_offset", "end_offset", "range_sha256")
        matches = [d for d in facts["declarations"] if all(d[k] == witness[k] for k in keys)]
        aid.require(len(matches) == 1, "stale or ambiguous candidate witness")
        aid.require(witness.get("evidence_ids"), "candidate evidence required")
        for identity in witness["evidence_ids"]:
            aid.require(any(r["case_id"] == identity and all(r[k] == witness[k] for k in keys[:3]) for r in associations), "candidate evidence is not bound to declaration")


def validate_attempt(attempt, manifest, root):
    oracle, receipt = validate_attempt_execution(attempt, root)
    validate_attempt_source(receipt, manifest, oracle)
    current = {"oracle_sha256": attempt["oracle"]["sha256"], "command_sha256": aid.digest(json.dumps(receipt["command"], separators=(",", ":")).encode())}
    return current, {**current, "child_exit": receipt["child_exit"], "active_seconds": receipt.get("active_seconds"), "queue_seconds": receipt.get("queue_seconds")}


def validate_attempt_execution(attempt, root):
    oracle = artifact(attempt["oracle"], root)
    receipt = aid.read_json(artifact(attempt["receipt"], root))
    aid.require(receipt["status"] == "child_exited" and isinstance(receipt["child_exit"], int), "characterization execution not terminal")
    aid.require(receipt["command"] and attempt.get("interpretation"), "actual command and result interpretation required")
    return oracle, receipt


def validate_attempt_source(receipt, manifest, oracle):
    compatible = []
    for source in receipt["source_before"]:
        files = {f["path"]: f.get("sha256") for f in source["files"]}
        if all(files.get(f["path"]) == f.get("sha256") for f in manifest["files"]):
            compatible.append(source)
    aid.require(len(compatible) == 1, "characterization source differs from checkpoint")
    aid.require(any(f.get("sha256") == aid.digest(oracle.read_bytes()) for source in receipt["source_before"] for f in source["files"]), "oracle not bound in execution source receipt")
    aid.require(any(source["files"] == compatible[0]["files"] for source in receipt["source_after"]), "characterization source changed during execution")


def characterization(value, facts, manifest, root):
    status = value.get("status")
    aid.require(status in ("planned", "blocked", "attempted", "passed", "failed"), "characterization status required")
    attempts = value.get("attempts", [])
    aid.require(value.get("observable_boundary"), "observable boundary required")
    if status in ("planned", "blocked"):
        aid.require(not attempts and value.get("next_step"), "pre-attempt state needs next step and no completion claim")
        return
    aid.require(attempts, "attempted characterization needs bound artifact and receipt")
    previous = None
    history = []
    aid.require(value.get("obligation_id"), "characterization obligation required")
    for attempt in attempts:
        aid.require(attempt.get("obligation_id") == value["obligation_id"], "unrelated characterization obligation")
        current, checked = validate_attempt(attempt, manifest, root)
        if previous is not None and current != previous:
            aid.require(attempt.get("replaces") == previous and attempt.get("replacement_reason"), "unrelated latest pass: oracle or command changed without explicit versioned replacement")
        previous = current
        history.append(checked)
    if status == "passed":
        aid.require(history[-1]["child_exit"] == 0, "passed claim conflicts with latest execution")
    if status == "failed":
        aid.require(history[-1]["child_exit"] != 0, "failed claim conflicts with latest execution")
    return history


def validate(checkpoint, facts, rows, manifest, root):
    aid.require(checkpoint["version"] == 1 and checkpoint["source_identity"] == manifest["sha256"], "checkpoint source binding mismatch")
    for key in ("preferred_candidate", "whole_owner_competitor"):
        aid.require(key in checkpoint, "preferred candidate and strongest whole-owner competitor required")
        candidate(checkpoint[key], facts, rows)
    aid.require(checkpoint.get("comparison_reason") and checkpoint.get("competitor_selection_reason"), "source-based comparison and strongest-competitor rationale required")
    aid.require(checkpoint.get("selected_investigation") in ("preferred_candidate", "whole_owner_competitor"), "selected candidate required")
    aid.require(checkpoint.get("disposition") and checkpoint.get("next_step"), "disposition and next step required")
    history = characterization(checkpoint["characterization"], facts, manifest, root) or []
    return {"attempt_history": history, "cumulative_costs": {key: sum(x[key] for x in history) if history and all(isinstance(x[key], (int, float)) for x in history) else None for key in ("active_seconds", "queue_seconds")}, "structural_and_binding_checks": "passed", "semantic_priority": "not adjudicated", "behavior_preservation": "not proven", "characterization_status": checkpoint["characterization"]["status"]}


def main():
    parser = argparse.ArgumentParser(description="Optional experimental checkpoint structure/binding check")
    for name in ("checkpoint", "database", "facts", "manifest", "binding", "scan-receipt", "evidence-root"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    facts, binding = aid.verify_binding(args.database, args.facts, args.manifest, args.binding, args.scan_receipt)
    rows, exclusions = aid.select(args.database)
    result = validate(aid.read_json(args.checkpoint), facts, rows, aid.read_json(args.manifest), args.evidence_root)
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
