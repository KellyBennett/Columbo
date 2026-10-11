import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import sqlite3

HERE = Path(__file__).resolve().parent


def digest(data):
    return hashlib.sha256(data).hexdigest()


def read_json(path):
    return json.loads(Path(path).read_text())


def require(condition, message):
    if not condition:
        raise ValueError(message)


def manifest_identity(entries):
    return digest(json.dumps(entries, sort_keys=True, separators=(",", ":")).encode())


def source_files(root, manifest):
    root = Path(root)
    require(root.is_absolute() and root.resolve() == root, "source root must be absolute without symlinks")
    entries = manifest["files"]
    require(manifest_identity(entries) == manifest["sha256"], "invalid source manifest identity")
    by_path = {}
    for entry in entries:
        relative = entry["path"]
        path = PurePosixPath(relative)
        require(not path.is_absolute() and ".." not in path.parts and str(path) == relative, "unsafe manifest path")
        require(relative not in by_path and "symlink" not in entry, "duplicate or symlink manifest entry")
        actual = root / relative
        require(actual.resolve() == actual, "symlink source path refused")
        require(digest(actual.read_bytes()) == entry["sha256"], "stale source: " + relative)
        by_path[relative] = entry["sha256"]
    actual_paths = set()
    for path in root.rglob("*"):
        if ".git" in path.relative_to(root).parts:
            continue
        require(not path.is_symlink(), "symlink source path refused")
        if path.is_file():
            actual_paths.add(path.relative_to(root).as_posix())
    require(actual_paths == set(by_path), "source file set differs from manifest")
    return by_path


def scan_scope(command, receipt):
    values = {"--config": ".columbo.yml"}
    arguments = list(command[1:])
    while arguments and arguments[0].startswith("-"):
        flag = arguments.pop(0)
        if flag == "--no-history":
            continue
        require(flag in ("--config", "--output") and arguments, "unsupported scan command/scope")
        values[flag] = arguments.pop(0)
    configuration = Path(values["--config"])
    if not configuration.is_absolute():
        configuration = Path(receipt["cwd"]) / configuration
    roots = [Path(item["root"]) for item in receipt["source_before"] if configuration.is_relative_to(Path(item["root"]))]
    require(len(roots) == 1, "configuration not bound to scan source")
    return arguments or ["./..."], configuration.relative_to(roots[0]).as_posix()


def verify_binding(database, facts_path, manifest_path, binding_path, receipt_path):
    binding = read_json(binding_path)
    manifest = read_json(manifest_path)
    facts = read_json(facts_path)
    receipt = read_json(receipt_path)
    require(facts["version"] == 1, "unsupported facts version")
    artifacts = [(database, "database_sha256"), (facts_path, "facts_sha256"),
                 (manifest_path, "manifest_sha256"), (receipt_path, "scan_receipt_sha256")]
    verify_artifact_binding(artifacts, binding, manifest)
    command = verify_scan_execution(receipt, binding)
    verify_source_receipt(receipt, manifest)
    config_path = verify_scan_configuration(command, receipt, binding, facts)
    verify_source_facts(facts, manifest, config_path)
    return facts, binding


def verify_artifact_binding(artifacts, binding, manifest):
    for path, key in artifacts:
        require(digest(Path(path).read_bytes()) == binding[key], "binding hash mismatch: " + key)
    require(binding["source_identity"] == manifest["sha256"], "binding source mismatch")
    require(binding["database_hash_provenance"] in ("scan-time", "current-observation"), "database hash provenance required")


def verify_scan_execution(receipt, binding):
    require(receipt["status"] == "child_exited" and receipt["child_exit"] in (0, 1), "scan did not complete")
    command = receipt["command"]
    require("--staged" not in command, "ordinary scan required")
    require("--output" in command and command[command.index("--output") + 1] == binding["database_path_at_scan"], "scan output path mismatch")
    return command


def verify_source_receipt(receipt, manifest):
    for phase in ("source_before", "source_after"):
        matches = [m for m in receipt[phase] if m["sha256"] == manifest["sha256"] and m["files"] == manifest["files"]]
        require(len(matches) == 1, "scan/source binding absent or changed")


def verify_scan_configuration(command, receipt, binding, facts):
    for key, value in facts["environment"].items():
        require(receipt["environment"].get(key, "") == value, "scan/runtime environment mismatch: " + key)
    patterns, config_path = scan_scope(command, receipt)
    require(binding["patterns"] == facts["patterns"] == patterns, "package patterns mismatch")
    require(facts["config_file"] == config_path, "scan configuration path mismatch")
    require(binding["configuration"] == facts["config"], "configuration mismatch")
    require(binding["scope_provenance"], "explicit scope provenance required")
    return config_path


def verify_source_facts(facts, manifest, config_path):
    files = source_files(Path(facts["root"]), manifest)
    require(files.get(config_path, "") == facts["config_sha256"], "configuration file hash mismatch")
    listed = {f["path"]: f["sha256"] for f in facts["files"]}
    require(len(listed) == len(facts["files"]), "duplicate facts file")
    require(listed == {p: h for p, h in files.items() if p.endswith(".go")}, "facts/full-source Go scope mismatch")
    verify_witnesses(facts, facts["root"], files)


def verify_witnesses(value, root, files):
    if isinstance(value, dict):
        if "range_sha256" in value:
            verify_span(value, root, files)
        for child in value.values():
            verify_witnesses(child, root, files)
    elif isinstance(value, list):
        for child in value:
            verify_witnesses(child, root, files)


def verify_span(span, root, files):
    require(span["file_sha256"] == files.get(span["path"]), "source witness file mismatch")
    data = (Path(root) / span["path"]).read_bytes()
    begin, end = span["start_offset"], span["end_offset"]
    require(0 <= begin <= end <= len(data), "source witness range invalid")
    require(digest(data[begin:end]) == span["range_sha256"], "source witness hash mismatch")


def select(database):
    with sqlite3.connect(Path(database).resolve().as_uri() + "?mode=ro", uri=True) as connection:
        connection.row_factory = sqlite3.Row
        require(connection.execute("PRAGMA integrity_check").fetchone()[0] == "ok", "database integrity failure")
        require(connection.execute("SELECT schema_version FROM report").fetchone()[0] == 8, "unsupported database schema")
        rows = [dict(row) for row in connection.execute((HERE / "select-seeds.sql").read_text())]
        excluded = [dict(row) for row in connection.execute("SELECT id, ordinal, smell, verdict, suppressed FROM cases WHERE suppressed <> 0 OR verdict = 'PASS' OR smell = 'prose-comment' ORDER BY ordinal")]
    for row in excluded:
        row["reasons"] = (["suppressed"] if row["suppressed"] else []) + (["PASS"] if row["verdict"] == "PASS" else []) + (["prose-comment"] if row["smell"] == "prose-comment" else [])
    return rows, excluded


def storage(definition, types):
    found, unknown = [], []

    def walk(owner, prefix, ancestors):
        for field in owner["fields"]:
            names = field["names"] or [field["type_text"]]
            for name in names:
                path = prefix + [name]
                key = field.get("type_key", "")
                matches = types.get(key, [])
                if len(matches) != 1:
                    unknown.append({"field_path": path, "field": field, "reason": "builtin, compound, imported or unavailable selected local definition; no storage semantics inferred"})
                    continue
                target = matches[0]
                found.append({"field_path": path, "field": field, "definition": target, "relationship": "declared embedding" if field["embedded"] else "declared held field; usage unknown"})
                if field["embedded"]:
                    if key in ancestors:
                        unknown.append({"field_path": path, "reason": "embedding cycle; expansion stopped"})
                    else:
                        walk(target, path, ancestors | {key})

    walk(definition, [], {definition["key"]})
    return found, unknown


def source_indexes(facts):
    declarations, types = {}, {}
    for decl in facts["declarations"]:
        key = (decl["path"], decl["start_offset"], decl["end_offset"])
        declarations.setdefault(key, []).append(decl)
    for definition in facts["types"]:
        if not definition["alias"]:
            types.setdefault(definition["key"], []).append(definition)
    targets = {f["path"] for f in facts["files"] if f["ordinary_target"]}
    return declarations, types, targets


def qualify_declaration(row, declarations, types, targets):
    matches = declarations.get((row["path"], row["start_offset"], row["end_offset"]), [])
    if row["path"] not in targets:
        return None, "outside configured ordinary target paths"
    if len(matches) != 1 or matches[0]["symbol"] != row["symbol"]:
        return None, "no unique exact source declaration range and symbol"
    if matches[0]["kind"] != "method":
        return None, "source-confirmed free function, not instance method"
    if not matches[0].get("receiver_key") or len(types.get(matches[0].get("receiver_key"), [])) != 1:
        return None, "unresolved or ambiguous source receiver definition"
    return matches[0], ""


def enrich_seed(seed, facts, types):
    key = seed["receiver_key"]
    seed["receiver_definition"] = types[key][0]
    seed["methods"] = [d for d in facts["declarations"] if d.get("receiver_key") == key]
    seed["storage_definitions"], seed["unresolved_storage"] = storage(types[key][0], types)
    seed["one_hop_calls"] = [c for d in seed["methods"] for c in d["calls"] if c.get("target")]
    seed["unresolved_calls"] = [c for d in seed["methods"] for c in d["calls"] if not c.get("target")]
    seed["review_obligation"] = "Read every sibling body and justified one-hop/storage context. Record co-updates, reads, handle obligations, uncertainty, concrete alternative and benefit/cost. Retain with reasons or leave not-reached; no change is required."


def bind(rows, excluded_cases, facts):
    declarations, types, targets = source_indexes(facts)
    seeds, exclusions = {}, []
    for row in rows:
        declaration, reason = qualify_declaration(row, declarations, types, targets)
        if reason:
            exclusions.append({**row, "reason": reason})
            continue
        key = declaration["receiver_key"]
        seed = seeds.setdefault(key, {"receiver_key": key, "associations": [], "status": "not-reached"})
        seed["associations"].append({**row, "source_witness": {k: v for k, v in declaration.items() if k != "calls"}})
    for seed in seeds.values():
        enrich_seed(seed, facts, types)
    return {"version": 1, "experimental": True, "eligible_associations": len(rows), "included_associations": sum(len(s["associations"]) for s in seeds.values()), "excluded_associations": exclusions, "excluded_cases": excluded_cases, "receiver_seeds": [seeds[k] for k in sorted(seeds)], "scope_files": facts["files"], "limits": facts["limits"]}


def main():
    parser = argparse.ArgumentParser(description="Experimental read-only cooperating-method navigation")
    for name in ("database", "facts", "manifest", "binding", "scan-receipt", "output"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    facts, binding = verify_binding(args.database, args.facts, args.manifest, args.binding, args.scan_receipt)
    output = Path(args.output).resolve()
    require(not output.is_relative_to(Path(facts["root"])), "output must be outside source tree")
    rows, exclusions = select(args.database)
    result = bind(rows, exclusions, facts)
    result["binding"] = binding
    result["query_sha256"] = digest((HERE / "select-seeds.sql").read_bytes())
    result["binding_limit"] = "Schema 8 does not store source hashes. External receipt/manifest provenance binds source and scope; a current-observation report hash is not a scan-time database hash or independent proof of report authorship."
    with output.open("x") as stream:
        json.dump(result, stream, indent=2)
        stream.write("\n")


if __name__ == "__main__":
    main()
