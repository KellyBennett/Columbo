#!/usr/bin/env python3
"""Executable negative cases for the database boundary, using the real linters."""
import json
import os
import re
from pathlib import Path
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

with tempfile.TemporaryDirectory(prefix="columbo-db-negative-") as temporary:
    base = Path(temporary) / "base"
    (base / "internal/snapshotdb").mkdir(parents=True)
    # Match the actual application's import path and verified dependency versions.
    for name in ["go.mod", "go.sum", ".golangci.database.yml"]:
        shutil.copyfile(root / name, base / name)
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

    positive = copy_case("positive", "positive.go.txt")
    result, issues = lint_case(positive)
    require(result.returncode == 0 and not issues, "typed Querier positive control", result.stdout)
    for checker in ["check-manifest.py", "check-package-inventory.py", "check-generated.py"]:
        result = run(["python3", str(here / checker), "--root", str(positive)] +
                     (["--sqlc", sqlc] if checker == "check-generated.py" else []), positive)
        require(result.returncode == 0, f"positive {checker}", result.stdout)
    print("PASS: typed Querier and exact generated/lifecycle files")

    fixtures = {
        "direct_import.go.txt": ("depguard", ["database/sql"]),
        "dot_import.go.txt": ("forbidigo", ["Open"]),
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
            require(any(re.search(r"\b" + re.escape(word) + r"\b", i["Text"]) for i in checked), f"{fixture}: {word} rejected", result.stdout)
        print(f"PASS: {fixture} rejected by {linter}")

    # A new raw-SQL test does not inherit the six reviewed test exceptions.
    future = copy_case("future-test", "direct_import.go.txt", "internal/columbo/new_test.go")
    result, issues = lint_case(future)
    require(result.returncode != 0 and any(i["FromLinter"] == "depguard" for i in issues),
            "future test files are not exempt", result.stdout)
    print("PASS: new_test.go rejected by depguard")

    for fixture in ["nolint.go.txt", "slash_nolint.go.txt", "tab_permit.go.txt", "permit.go.txt", "extra_same_package.go.txt"]:
        target = "internal/snapshotdb/extra.go" if fixture == "extra_same_package.go.txt" else "internal/consumer/client.go"
        case = copy_case(fixture, fixture, target)
        result = run(["python3", str(here / "check-manifest.py"), "--root", str(case)], case)
        require(result.returncode != 0 and target in result.stdout,
                f"{fixture} rejected by exact manifest/directive guard", result.stdout)
        print(f"PASS: {fixture} rejected by manifest")

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

    case = Path(temporary) / "edited-generated"
    shutil.copytree(base, case)
    with (case / "internal/snapshotdb/db.go").open("a") as output:
        output.write("\n// Handwritten edit retaining the authentic sqlc generated header.\n")
    result = run(["python3", str(here / "check-generated.py"), "--root", str(case), "--sqlc", sqlc], case)
    require(result.returncode != 0 and "internal/snapshotdb/db.go" in result.stdout,
            "authentic generated header does not excuse generated edits", result.stdout)
    print("PASS: generated edit rejected by regeneration byte comparison")
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
print("Database boundary regression fixtures: all passed")
