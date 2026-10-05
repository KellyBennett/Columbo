#!/usr/bin/env python3
"""Executable negative cases for the database boundary, using the real linters."""
import json
import os
import re
from pathlib import Path
import shlex
import shutil
import subprocess
import tempfile

root = Path(__file__).resolve().parents[2]
here = Path(__file__).resolve().parent
tools = Path(os.environ.get("COLUMBO_TOOLS_BIN", root / ".tools/bin"))
lint = os.environ.get("GOLANGCI_LINT", str(tools / "golangci-lint"))
sqlc = os.environ.get("SQLC", str(tools / "sqlc"))
lint = shutil.which(lint) or str(Path(lint).resolve())
sqlc = shutil.which(sqlc) or str(Path(sqlc).resolve())
env = {**os.environ, "GOTOOLCHAIN": "local", "GOTELEMETRY": "off", "GOWORK": "off",
       "GOFLAGS": os.environ.get("GOFLAGS", "") + " -buildvcs=false -p=1", "GOMAXPROCS": "2"}
manifest = json.loads((here / "manifest.json").read_text())

def run(command, cwd, extra_env=None):
    return subprocess.run(command, cwd=cwd, env={**env, **(extra_env or {})}, text=True, stdout=subprocess.PIPE,
                          stderr=subprocess.STDOUT)

def require(condition, text, output=""):
    if not condition:
        raise SystemExit(f"FAIL: {text}\n{output}")

# The fixture components cannot prove that CI actually invokes the aggregate
# gate. Pin this job's reviewed direct commands, without parsing arbitrary YAML.
workflow = (root / ".github/workflows/test.yml").read_text()
job = re.search(r"(?ms)^  database-boundary:\n(.*?)(?=^  [\w-]+:|\Z)", workflow)
require(job is not None, "CI database-boundary job exists")
job = job[1]
job_fields = [line for line in job.splitlines() if re.match(r"^    \S", line)]
require([line.split(":", 1)[0].strip() for line in job_fields] == ["name", "runs-on", "env", "steps"]
        and all(re.match(r"^    (?:name|runs-on|env|steps):", line) for line in job_fields),
        "CI database-boundary job uses only reviewed unconditional job fields")
steps = re.split(r"^      - ", job, flags=re.M)[1:]
for command, conditional in [("./scripts/check-database-boundary.sh", False),
                             ("python3 scripts/database-boundary/test-guards.py", True)]:
    matches = [step for step in steps if re.search(r"^        run: " + re.escape(command) + r"$", step, re.M)]
    require(len(matches) == 1, f"CI invokes {command} exactly once")
    step = matches[0]
    lines = [line for line in step.splitlines() if line.strip() and not line.lstrip().startswith("#")]
    expected_fields = ["name", "if", "run"] if conditional else ["name", "run"]
    fields = [match[1] if (match := re.match(r"^(?:|        )([\w-]+):", line)) else None for line in lines]
    require(fields == expected_fields, f"CI preserves {command} reviewed step fields", step)
    conditions = re.findall(r"^        if:\s*(.*)$", step, re.M)
    require(conditions == (["always()"] if conditional else []),
            f"CI preserves {command} execution condition")
print("PASS: CI invokes both database checks and propagates their failures")

with tempfile.TemporaryDirectory(prefix="columbo-db-negative-") as temporary:
    base = Path(temporary) / "base"
    (base / "internal/snapshotdb").mkdir(parents=True)
    # Match the actual application's import path and verified dependency versions.
    for name in ["go.mod", "go.sum", ".golangci.database.yml"]:
        shutil.copyfile(root / name, base / name)
    # Copy production entrypoint/checkers, including any mutation under test.
    # Aggregate scenarios must not silently use a hardcoded test-only gate.
    for name in ["scripts/check-database-boundary.sh", *[
        "scripts/database-boundary/" + name for name in
        ["check-manifest.py", "check-package-inventory.py", "check-generated.py", "manifest.json", "tools.env"]]]:
        target = base / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(root / name, target)
    (base / "schema.sql").write_text("CREATE TABLE entries (id INTEGER PRIMARY KEY, value TEXT NOT NULL);\n")
    (base / "queries.sql").write_text("-- name: Values :many\nSELECT id, value FROM entries ORDER BY id;\n")
    (base / "sqlc.yaml").write_text('''version: "2"
sql:
  - engine: sqlite
    schema: schema.sql
    queries: queries.sql
    gen:
      go:
        package: snapshotdb
        out: internal/snapshotdb
        emit_interface: true
''')
    # An intentionally leaky synthetic exempt provider verifies that even a passed
    # handle with no consumer import cannot evade the off-the-shelf call guard.
    (base / manifest["lifecycle"]).write_text('''package snapshotdb
import "database/sql"
func RawForFixture() *sql.DB { return nil }
''')
    generated = run([sqlc, "generate"], base)
    require(generated.returncode == 0, "fixture sqlc generation", generated.stdout)

    def copy_case(name, fixture, target="internal/consumer/client.go"):
        case = Path(temporary) / name
        shutil.copytree(base, case)
        path = case / target
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text((here / "fixtures" / fixture).read_text())
        return case

    def lint_case(case):
        result = run([lint, "run", "--config", ".golangci.database.yml",
                      "--output.json.path", "lint.json", "./..."], case)
        require((case / "lint.json").is_file(), "linter emitted JSON", result.stdout)
        issues = json.loads((case / "lint.json").read_text()).get("Issues") or []
        require(not any(i["FromLinter"] == "typecheck" for i in issues),
                "fixture must compile; compiler errors are not guard success", result.stdout)
        return result, issues

    def aggregate(case):
        schema = os.environ.get("GOLANGCI_LINT_SCHEMA", str(Path(lint).parent / "golangci.database.schema.json"))
        return run(["bash", "scripts/check-database-boundary.sh"], case, {
            "GOLANGCI_LINT": lint, "SQLC": sqlc,
            "GOLANGCI_LINT_SCHEMA": str(Path(schema).resolve()),
            "BOUNDARY_REPORT_DIR": str(case / "boundary-report")})

    positive = copy_case("positive", "positive.go.txt")
    result, issues = lint_case(positive)
    require(result.returncode == 0 and not issues, "typed Querier positive control", result.stdout)
    for checker in ["check-manifest.py", "check-package-inventory.py", "check-generated.py"]:
        result = run(["python3", str(here / checker), "--root", str(positive)] +
                     (["--sqlc", sqlc] if checker == "check-generated.py" else []), positive)
        require(result.returncode == 0, f"positive {checker}", result.stdout)
    print("PASS: typed Querier and exact generated/lifecycle files")
    result = aggregate(positive)
    require(result.returncode == 0, "positive production aggregate gate", result.stdout)
    print("PASS: production aggregate gate accepts typed Querier")

    fixtures = {
        "direct_import.go.txt": ("depguard", ["database/sql"]),
        "dot_import.go.txt": ("forbidigo", ["Open"]),
        "database_open_apis.go.txt": ("forbidigo", ["OpenDB", "Register"]),
        "sqlite_driver_import.go.txt": ("depguard", ["modernc.org/sqlite"]),
        "concrete_promoted.go.txt": ("forbidigo", ["PrepareContext"]),
        "raw_transaction_api.go.txt": ("forbidigo", ["WithTx"]),
        "driver_import.go.txt": ("depguard", ["database/sql/driver"]),
        "alias_import.go.txt": ("forbidigo", ["Exec"]),
        "passed_handle.go.txt": ("forbidigo", ["Query"]),
        "local_interface.go.txt": ("forbidigo", ["Exec", "Query", "QueryRow", "Prepare",
            "ExecContext", "QueryContext", "QueryRowContext", "PrepareContext", "Raw"]),
        "embedded_method_value.go.txt": ("forbidigo", ["Query"]),
        "method_expression.go.txt": ("forbidigo", ["PrepareContext"]),
        "fake_generated.go.txt": ("forbidigo", ["Query"]),
        "raw_generated_api.go.txt": ("forbidigo", ["DBTX", "Queries", "New"]),
    }
    for fixture, (linter, words) in fixtures.items():
        case = copy_case(fixture, fixture)
        result, issues = lint_case(case)
        checked = [i for i in issues if i["FromLinter"] == linter]
        require(result.returncode != 0 and checked, f"{fixture} rejected by {linter}", result.stdout)
        for word in words:
            if linter == "forbidigo":
                # Match the actual rejected symbol, never explanatory prose.
                # Every Raw-API message itself contains the word "Raw".
                symbols = [match[1].rsplit(".", 1)[-1] for i in checked
                           if (match := re.search(r"use of `([^`]+)` forbidden", i["Text"]))]
                found = word in symbols
            else:
                found = any(re.search(r"\b" + re.escape(word) + r"\b", i["Text"]) for i in checked)
            require(found, f"{fixture}: {word} rejected", result.stdout)
        print(f"PASS: {fixture} rejected by {linter}")

    # Lightweight external stub modules exercise import policy, not driver
    # behavior. Their source lives outside each fixture root and is not a hidden
    # repository package; no third-party downloads or application deps change.
    for name, package in [
        ("mattn", "github.com/mattn/go-sqlite3"),
        ("ncruces", "github.com/ncruces/go-sqlite3"),
        ("tools", "github.com/KellyBennett/Columbo/.tools/helper"),
        ("git", "github.com/KellyBennett/Columbo/.git/helper"),
        # A terminating vendor element is a valid Go package. A descendant such
        # as vendor/helper is compiler-rejected and would not prove lint policy.
        ("vendor", "github.com/KellyBennett/Columbo/vendor"),
    ]:
        external = Path(temporary) / ("external-" + name)
        external.mkdir()
        (external / "go.mod").write_text(f"module {package}\ngo 1.25.1\n")
        (external / "stub.go").write_text("package fixture\n")
        case = Path(temporary) / ("import-" + name)
        shutil.copytree(base, case)
        with (case / "go.mod").open("a") as output:
            output.write(f"\nrequire {package} v0.0.0\nreplace {package} => {external}\n")
        consumer = case / "internal/consumer"
        consumer.mkdir()
        (consumer / "client.go").write_text(f'package consumer\nimport _ "{package}"\n')
        result, issues = lint_case(case)
        require(result.returncode != 0 and any(i["FromLinter"] == "depguard" and package in i["Text"] for i in issues),
                f"{package}: standalone import policy rejects the compiled stub", result.stdout)
        print(f"PASS: {package} import rejected by depguard")

    # A new raw-SQL test does not inherit the six reviewed test exceptions.
    future = copy_case("future-test", "direct_import.go.txt", "internal/columbo/new_test.go")
    result, issues = lint_case(future)
    require(result.returncode != 0 and any(i["FromLinter"] == "depguard" for i in issues),
            "future test files are not exempt", result.stdout)
    print("PASS: new_test.go rejected by depguard")
    for reviewed in manifest["test_exceptions"]:
        target = reviewed.removesuffix("_test.go") + ".go"
        case = copy_case("runtime-" + Path(target).stem, "direct_import.go.txt", target)
        result, issues = lint_case(case)
        require(result.returncode != 0 and any(i["FromLinter"] == "depguard" for i in issues),
                f"{target}: reviewed test exception cannot include runtime counterpart", result.stdout)
        print(f"PASS: {target} is not a runtime exception")

    for fixture in ["nolint.go.txt", "slash_nolint.go.txt", "tab_permit.go.txt", "permit.go.txt", "extra_same_package.go.txt"]:
        target = "internal/snapshotdb/extra.go" if fixture == "extra_same_package.go.txt" else "internal/consumer/client.go"
        case = copy_case(fixture, fixture, target)
        result = run(["python3", str(here / "check-manifest.py"), "--root", str(case)], case)
        require(result.returncode != 0 and target in result.stdout,
                f"{fixture} rejected by exact manifest/directive guard", result.stdout)
        print(f"PASS: {fixture} rejected by manifest")

    for name, target, reason in [
        ("missing-lifecycle", manifest["lifecycle"], "Required regular boundary file is missing"),
        ("missing-header", "internal/snapshotdb/models.go", "lacks the exact generator header"),
        ("source-symlink", "internal/consumer/client.go", "Source symlinks cannot expand"),
    ]:
        case = Path(temporary) / name
        shutil.copytree(base, case)
        path = case / target
        if name == "missing-lifecycle":
            path.unlink()
        elif name == "missing-header":
            path.write_text(path.read_text().removeprefix("// Code generated by sqlc. DO NOT EDIT.\n"))
        else:
            path.parent.mkdir(parents=True)
            path.symlink_to(here / "fixtures/positive.go.txt")
        result = run(["python3", str(here / "check-manifest.py"), "--root", str(case)], case)
        require(result.returncode != 0 and target in result.stdout and reason in result.stdout,
                f"{name}: explicit manifest convention is enforced", result.stdout)
        print(f"PASS: {name} rejected by manifest")

    for name, fixture, target, build_args, build_env in [
        ("windows-source", "direct_import.go.txt", "internal/consumer/client_windows.go", [], {"GOOS": "windows", "GOARCH": "amd64", "CGO_ENABLED": "0"}),
        ("legacy-tagged-source", "legacy_tagged_source.go.txt", "internal/consumer/client.go", ["-tags=future_sql"], {}),
        ("tagged-source", "tagged_source.go.txt", "internal/consumer/client.go", ["-tags=future_sql"], {}),
    ]:
        case = copy_case(name, fixture, target)
        compiled = run(["go", "build", *build_args, "./..."], case, build_env)
        require(compiled.returncode == 0, f"{name} compiles in relevant variant", compiled.stdout)
        result = run(["python3", str(here / "check-manifest.py"), "--root", str(case)], case)
        require(result.returncode != 0 and "Unreviewed source" in result.stdout,
                f"{name} rejected by explicit source inventory convention", result.stdout)
        print(f"PASS: {name} compiles and is rejected by manifest")

    for name, fixture, target, args, extra_env in [
        ("windows-test-source", "direct_import.go.txt", "internal/consumer/client_windows_test.go",
         ["go", "test", "-c", "-o", str(Path(temporary) / "windows-test.exe"), "./internal/consumer"],
         {"GOOS": "windows", "GOARCH": "amd64", "CGO_ENABLED": "0"}),
        ("tagged-test-source", "tagged_source.go.txt", "internal/consumer/client_test.go",
         ["go", "test", "-run", "^$", "-tags=future_sql", "./..."], {}),
    ]:
        case = copy_case(name, fixture, target)
        compiled = run(args, case, extra_env)
        require(compiled.returncode == 0, f"{name} compiles in relevant test variant", compiled.stdout)
        result = run(["python3", str(here / "check-manifest.py"), "--root", str(case)], case)
        require(result.returncode != 0 and "Unreviewed source" in result.stdout,
                f"{name}: future tests cannot hide raw APIs", result.stdout)
        print(f"PASS: {name} compiles and is rejected by manifest")

    for name, helper_dir, import_path in [
        ("active-testdata", "internal/columbo/testdata/hiddenraw", "github.com/KellyBennett/Columbo/internal/columbo/testdata/hiddenraw"),
        ("active-new-testdata", "internal/new/testdata/hiddenraw", "github.com/KellyBennett/Columbo/internal/new/testdata/hiddenraw"),
        ("active-underscore", "internal/_hiddenraw", "github.com/KellyBennett/Columbo/internal/_hiddenraw"),
        ("active-dot", "internal/.hiddenraw", "github.com/KellyBennett/Columbo/internal/.hiddenraw"),
        ("active-nested-module", "internal/nestedraw", "example.com/hiddenraw"),
        ("active-directory-symlink", "internal/columbo/testdata/hiddenraw", "github.com/KellyBennett/Columbo/internal/rawlink"),
    ]:
        case = Path(temporary) / name
        shutil.copytree(base, case)
        helper = case / helper_dir
        helper.mkdir(parents=True)
        (helper / "helper.go").write_text((here / "fixtures/hidden_helper.go.txt").read_text())
        consumer = case / "internal/consumer"
        consumer.mkdir()
        (consumer / "client.go").write_text(f'package consumer\nimport "{import_path}"\nfunc Bad() {{ hiddenraw.Do("SELECT 1") }}\n')
        if name == "active-nested-module":
            (helper / "go.mod").write_text("module example.com/hiddenraw\ngo 1.25.1\n")
            with (case / "go.mod").open("a") as output:
                output.write("\nrequire example.com/hiddenraw v0.0.0\nreplace example.com/hiddenraw => ./internal/nestedraw\n")
        if name == "active-directory-symlink":
            (case / "internal/rawlink").symlink_to(helper, target_is_directory=True)
        compiled = run(["go", "build", "./..."], case)
        require(compiled.returncode == 0, f"{name}: active helper compiles", compiled.stdout)
        result = run(["python3", str(here / "check-package-inventory.py"), "--root", str(case)], case)
        require(result.returncode != 0 and "absent from ./... lint targets" in result.stdout,
                f"{name}: reachable hidden helper rejected by inventory", result.stdout)
        if name == "active-testdata":
            result, issues = lint_case(case)
            require(result.returncode != 0 and any(i["FromLinter"] == "depguard" and "testdata" in i["Text"] for i in issues),
                    "canonical testdata dependency rejected by depguard", result.stdout)
        if name == "active-new-testdata":
            result = run(["python3", str(here / "check-manifest.py"), "--root", str(case)], case)
            require(result.returncode != 0 and "Unreviewed testdata source tree" in result.stdout,
                    "new corpus root rejected by manifest", result.stdout)
        if name == "active-directory-symlink":
            result = run(["python3", str(here / "check-manifest.py"), "--root", str(case)], case)
            require(result.returncode != 0 and "directory symlinks" in result.stdout,
                    "directory symlink rejected by manifest", result.stdout)
        print(f"PASS: {name} compiles and is rejected by package inventory")

    for name, regular in [("implicit-cgo-mixed", True), ("implicit-cgo-only", False)]:
        case = copy_case(name, "implicit_cgo.go.txt", "internal/consumer/raw.go")
        if regular:
            (case / "internal/consumer/regular.go").write_text((here / "fixtures/positive.go.txt").read_text())
        compiled = run(["go", "build", "./..."], case, {"CGO_ENABLED": "1"})
        require(compiled.returncode == 0, f"{name} compiles with CGO_ENABLED=1", compiled.stdout)
        result = run(["python3", str(here / "check-package-inventory.py"), "--root", str(case)], case,
                     {"CGO_ENABLED": "0"})
        require(result.returncode != 0 and "IgnoredGoFiles" in result.stdout and "internal/consumer/raw.go" in result.stdout,
                f"{name}: implicit inactive source rejected by official metadata", result.stdout)
        print(f"PASS: {name} compiles and is rejected by ignored-source inventory")
        active = run(["python3", str(here / "check-package-inventory.py"), "--root", str(case)], case,
                     {"CGO_ENABLED": "1"})
        require(active.returncode != 0 and "CgoFiles" in active.stdout and "internal/consumer/raw.go" in active.stdout,
                f"{name}: active cgo source is an unreviewed build variant", active.stdout)
        print(f"PASS: {name} active CgoFiles rejected by package inventory")

    for name in manifest["generated"]:
        case = Path(temporary) / ("edited-" + Path(name).name)
        shutil.copytree(base, case)
        with (case / name).open("a") as output:
            output.write("\n// Handwritten edit retaining the authentic sqlc generated header.\n")
        result = run(["python3", str(here / "check-generated.py"), "--root", str(case), "--sqlc", sqlc], case)
        require(result.returncode != 0 and name in result.stdout,
                f"{name}: authentic generated header does not excuse generated edits", result.stdout)
        print(f"PASS: {name} edit rejected by regeneration byte comparison")
    for name, before, after in [
        ("redirected-output", "out: internal/snapshotdb", "out: internal/otherdb"),
        ("missing-interface", "emit_interface: true", "emit_interface: false"),
    ]:
        case = Path(temporary) / name
        shutil.copytree(base, case)
        config = case / "sqlc.yaml"
        config.write_text(config.read_text().replace(before, after))
        result = run(["python3", str(here / "check-generated.py"), "--root", str(case), "--sqlc", sqlc], case)
        require(result.returncode != 0 and "sqlc output differs" in result.stdout,
                f"{name}: stale generated files cannot pass", result.stdout)
        print(f"PASS: {name} rejected despite stale original outputs")

    extra_generated = Path(temporary) / "unexpected-generated-file"
    shutil.copytree(base, extra_generated)
    (extra_generated / "internal/snapshotdb/extra.go").write_text("package snapshotdb\n")
    result = run(["python3", str(here / "check-generated.py"), "--root", str(extra_generated), "--sqlc", sqlc], extra_generated)
    require(result.returncode != 0 and "internal/snapshotdb/extra.go: sqlc output differs" in result.stdout,
            "regeneration rejects an unexpected same-package output independently of manifest", result.stdout)
    print("PASS: regeneration rejects unexpected same-package output")

    wrong_version = Path(temporary) / "wrong-version-sqlc"
    wrong_version.write_text('#!/bin/sh\nif [ "$1" = version ]; then echo v0.0.0; else exec ' + shlex.quote(sqlc) + ' "$@"; fi\n')
    wrong_version.chmod(0o700)
    result = run(["python3", str(here / "check-generated.py"), "--root", str(positive), "--sqlc", str(wrong_version)], positive)
    require(result.returncode != 0 and "sqlc version mismatch" in result.stdout,
            "generator provenance rejects wrong version even if generation bytes would match", result.stdout)
    print("PASS: generator rejects wrong reported version")

    test_only = Path(temporary) / "test-only-hidden"
    shutil.copytree(base, test_only)
    helper = test_only / "internal/columbo/testdata/hiddenraw"
    helper.mkdir(parents=True)
    (helper / "helper.go").write_text((here / "fixtures/hidden_helper.go.txt").read_text())
    (test_only / "internal/columbo/acceptance_test.go").write_text('''package columbo
import "github.com/KellyBennett/Columbo/internal/columbo/testdata/hiddenraw"
func testOnlyDependency() { hiddenraw.Do("SELECT 1") }
''')
    compiled = run(["go", "test", "-run", "^$", "./..."], test_only)
    require(compiled.returncode == 0, "test-only hidden dependency compiles", compiled.stdout)
    result = run(["python3", str(here / "check-package-inventory.py"), "--root", str(test_only)], test_only)
    require(result.returncode != 0 and "absent from ./... lint targets" in result.stdout,
            "test-only dependency is included in package inventory", result.stdout)
    print("PASS: test-only hidden dependency is rejected by package inventory")

    # Each input violates exactly one component. If the entrypoint drops a check
    # or swallows its exit status, the aggregate result must fail this test.
    aggregate_manifest = copy_case("aggregate-manifest", "nolint.go.txt")
    aggregate_lint = copy_case("aggregate-lint", "passed_handle.go.txt")
    aggregate_generated = Path(temporary) / "aggregate-generated"
    shutil.copytree(base, aggregate_generated)
    with (aggregate_generated / "internal/snapshotdb/queries.sql.go").open("a") as output:
        output.write("\n// Deliberate audit edit retaining the generated header.\n")
    for case, component, reason in [
        (aggregate_manifest, "manifest", "Inline lint/permit exclusions are forbidden"),
        (test_only, "inventory", "absent from ./... lint targets"),
        (aggregate_lint, "lint", "use of `db.Query` forbidden"),
        (aggregate_generated, "generated", "internal/snapshotdb/queries.sql.go: sqlc output differs"),
    ]:
        result = aggregate(case)
        require(result.returncode != 0 and reason in result.stdout,
                f"production aggregate propagates {component} rejection", result.stdout)
        # Attribution matters: an unrelated failure must not make a swallowed
        # component exit status look like a successful regression test.
        reports = case / "boundary-report"
        for other, filename in [("manifest", "manifest.json"), ("inventory", "packages.json"), ("lint", "lint.json")]:
            report = reports / filename
            require(report.is_file(), f"aggregate emitted {filename}", result.stdout)
            findings = json.loads(report.read_text())
            if other == "lint":
                findings = findings.get("Issues") or []
                require(not any(i["FromLinter"] == "typecheck" for i in findings),
                        "aggregate fixtures compile", result.stdout)
            if other != component:
                require(not findings, f"aggregate {component} fixture leaves {other} clean", result.stdout)
        if component != "generated":
            require("generated output: clean" in result.stdout,
                    f"aggregate {component} fixture leaves generation clean", result.stdout)
        print(f"PASS: production aggregate propagates {component} rejection")
print("Database boundary regression fixtures: all passed")
