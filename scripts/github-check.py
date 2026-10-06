#!/usr/bin/env python3
"""Read an immutable Columbo snapshot and publish all findings as a check.

This is a GitHub integration adapter, not a second analyzer or report format.
It requires only Python's standard library and a checks:write workflow token.
"""

import argparse
import contextlib
import json
import os
from pathlib import Path
import re
import sqlite3
import sys
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


def limited(text, maximum=65536):
    encoded = text.encode("utf-8")
    if len(encoded) <= maximum:
        return text
    suffix = "\n[Truncated; full evidence is in the SQLite artifact.]"
    return encoded[:maximum - len(suffix.encode())].decode("utf-8", errors="ignore") + suffix


def metric_text(row):
    def number(value):
        if row["kind"] in ("parameter-overlap", "dependency-overlap", "foreign-own-ratio"):
            return f"{value:.6f}"
        return str(value)
    text = f'{row["kind"]} {row["subject"]}: {number(row["numeric_value"])}'
    if row["limit_value"] is not None:
        text += f' (limit {row["operator"]} {number(row["limit_value"])})'
    return text


class Snapshot:
    def __init__(self, connection):
        self.db = connection
        self.db.row_factory = sqlite3.Row

    def validate(self):
        if self.db.execute("PRAGMA application_id").fetchone()[0] != 0x434C4D42:
            raise ValueError("not a Columbo snapshot")
        if self.db.execute("PRAGMA user_version").fetchone()[0] != 4:
            raise ValueError("unsupported Columbo snapshot schema")
        reports = self.db.execute("SELECT id,schema_version FROM report").fetchall()
        if [tuple(row) for row in reports] != [(1, 4)]:
            raise ValueError("invalid Columbo report metadata")
        if [tuple(row) for row in self.db.execute("PRAGMA integrity_check")] != [("ok",)]:
            raise ValueError("snapshot integrity check failed")
        if self.db.execute("PRAGMA foreign_key_check").fetchall():
            raise ValueError("snapshot foreign key check failed")

    def annotations(self):
        cases = self.db.execute("""
            SELECT c.*,d.symbol,f.path FROM cases c
            JOIN declarations d ON d.id=c.primary_declaration_id
            JOIN files f ON f.id=d.file_id
            WHERE c.suppressed=0 ORDER BY c.ordinal
        """).fetchall()
        result = [self.case_annotation(row) for row in cases]
        for row in self.db.execute("""
            SELECT w.*,f.path FROM warnings w LEFT JOIN files f ON f.id=w.file_id
            WHERE w.file_id IS NOT NULL AND w.line>0 ORDER BY w.ordinal
        """):
            result.append(dict(path=row["path"], start_line=row["line"], end_line=row["line"],
                               annotation_level="warning", title=limited("Columbo: " + row["code"], 255),
                               message=limited(row["message"])))
        return result

    def case_annotation(self, row):
        metrics = [metric_text(metric) for metric in self.db.execute("""
            SELECT kind,subject,numeric_value,limit_value,operator FROM clues
            WHERE case_id=? AND numeric_value IS NOT NULL ORDER BY ordinal
        """, (row["id"],))]
        policy = []
        for item in self.db.execute("""
            SELECT p.* FROM case_policy_reviews cp JOIN policy_reviews p ON p.id=cp.policy_id
            WHERE cp.case_id=? ORDER BY cp.ordinal
        """, (row["id"],)):
            policy.extend([f'Policy {item["id"]}: {item["note"]}', f'Review: {item["review_prompt"]}'])
        guidance = [f'{item["kind"]}: {item["item"]}' for item in self.db.execute(
            "SELECT kind,item FROM case_guidance WHERE case_id=? ORDER BY kind,ordinal", (row["id"],))]
        message = "\n".join([f'CASE {row["id"]} {row["symbol"]}: {row["verdict"]}',
                              row["why"], row["diagnosis"], *metrics, *self.dependency_uses(row["id"]), *self.duplicate_fragments(row["id"]), *self.variant_evidence(row["id"]), *self.selection_evidence(row["id"]), *policy, *guidance])
        return dict(path=row["path"], start_line=row["start_line"], end_line=row["end_line"],
                    annotation_level="failure" if row["verdict"] == "FAIL" else "warning",
                    title=limited("Columbo: " + row["smell"], 255), message=limited(message))

    def duplicate_fragments(self, case_id):
        return [f'Duplicate fragment: {row["path"]}:{row["start_line"]}-{row["end_line"]} (bytes {row["start_offset"]}-{row["end_offset"]})'
                for row in self.db.execute("""
                    SELECT f.path,r.start_line,r.end_line,r.start_offset,r.end_offset
                    FROM source_receipts r JOIN files f ON f.id=r.file_id
                    WHERE r.case_id=? AND r.kind='duplicate-fragment'
                    ORDER BY f.path,r.start_offset,r.end_offset
                """, (case_id,))]

    def selection_evidence(self, case_id):
        evidence = []
        for clue in self.db.execute("SELECT id,kind,subject FROM clues WHERE case_id=? AND kind IN ('selected-role','selected-implementation-set','selected-message-set') ORDER BY ordinal", (case_id,)):
            values = [row[0] for row in self.db.execute("SELECT value FROM clue_values WHERE clue_id=? ORDER BY ordinal", (clue["id"],))]
            evidence.append(f'{clue["kind"]} {clue["subject"]}: ' + ", ".join(values))
        for row in self.db.execute("""
            SELECT r.kind,r.subject,f.path,r.start_line,r.end_line,r.start_offset,r.end_offset
            FROM source_receipts r JOIN files f ON f.id=r.file_id
            WHERE r.case_id=? AND r.kind IN ('selection-decision','selection-origin','selection-flow','selected-message')
            ORDER BY f.path,r.start_offset,r.kind
        """, (case_id,)):
            evidence.append(f'{row["kind"]} {row["path"]}:{row["start_line"]}-{row["end_line"]} (bytes {row["start_offset"]}-{row["end_offset"]}): {row["subject"]}')
        return evidence

    def variant_evidence(self, case_id):
        evidence = []
        for clue in self.db.execute("SELECT id,kind,subject FROM clues WHERE case_id=? AND kind IN ('variant-set','repeated-variant-set') ORDER BY ordinal", (case_id,)):
            values = [row[0] for row in self.db.execute("SELECT value FROM clue_values WHERE clue_id=? ORDER BY ordinal", (clue["id"],))]
            evidence.append(f'{clue["kind"]} {clue["subject"]}: ' + ", ".join(values))
        for row in self.db.execute("""
            SELECT f.path,r.start_line,r.end_line,r.start_offset,r.end_offset,d.symbol
            FROM source_receipts r JOIN files f ON f.id=r.file_id
            JOIN declarations d ON d.id=r.source_declaration_id
            WHERE r.case_id=? AND r.kind='variant-decision' ORDER BY f.path,r.start_offset
        """, (case_id,)):
            evidence.append(f'Variant decision: {row["path"]}:{row["start_line"]}-{row["end_line"]} (bytes {row["start_offset"]}-{row["end_offset"]}) in {row["symbol"]}')
        return evidence

    def dependency_uses(self, case_id):
        labels = {"declared": "Declared or constructed types", "signature": "Callable signatures",
                  "signature-only": "Signature only", "supplied": "Supplied arguments",
                  "consumed": "Consumed results", "discarded": "Discarded results",
                  "receiver": "Receivers", "value": "Selected values", "package": "Package references"}
        groups = []
        for clue in self.db.execute("SELECT id,kind FROM clues WHERE case_id=? AND kind LIKE 'dependency-use-%' ORDER BY ordinal", (case_id,)):
            values = [item[0] for item in self.db.execute("SELECT value FROM clue_values WHERE clue_id=? ORDER BY ordinal", (clue["id"],))]
            origin = clue["kind"].removeprefix("dependency-use-")
            groups.append(labels.get(origin, origin) + ": " + ", ".join(values))
        if not groups:
            return []
        return ["Dependency origins (lists overlap; each identity counts once):", *groups]

    def summary(self, exit_code):
        totals = tuple(self.db.execute("SELECT failed,warned,suppressed FROM summary").fetchone())
        expected = 1 if totals[0] else 0
        if exit_code != expected:
            raise ValueError("analysis exit code disagrees with stored verdicts")
        text = f"{totals[0]} failed, {totals[1]} warned, {totals[2]} suppressed."
        warnings = [row[0] for row in self.db.execute("SELECT message FROM warnings ORDER BY ordinal")]
        if warnings:
            text += "\n\nAnalysis warnings:\n" + "\n".join(warnings)
        return limited(text)


def read_snapshot(path, exit_code):
    if exit_code == 2:
        return [], "Analysis failed (exit 2). No finding annotations were published; see the workflow log."
    uri = Path(path).resolve(strict=True).as_uri() + "?mode=ro"
    with contextlib.closing(sqlite3.connect(uri, uri=True)) as db:
        db.execute("PRAGMA query_only=ON")
        snapshot = Snapshot(db)
        snapshot.validate()
        return snapshot.annotations(), snapshot.summary(exit_code)


class ChecksAPI:
    def __init__(self, repository, token):
        if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
            raise ValueError("invalid GitHub repository")
        if not token:
            raise ValueError("GH_TOKEN is required with checks:write permission")
        self.base = "https://api.github.com/repos/" + repository + "/check-runs"
        self.token = token

    def request(self, method, suffix, payload):
        request = Request(self.base + suffix, data=json.dumps(payload).encode(), method=method,
                          headers={"Authorization": "Bearer " + self.token,
                                   "Accept": "application/vnd.github+json",
                                   "Content-Type": "application/json",
                                   "X-GitHub-Api-Version": "2022-11-28",
                                   "User-Agent": "Columbo-check-publisher"})
        try:
            with urlopen(request, timeout=30) as response:
                return json.load(response)
        except HTTPError as error:
            # Do not print response headers, request objects, or credentials.
            raise RuntimeError(f"Checks API returned HTTP {error.code}; verify checks:write permission") from None
        except URLError:
            # Never blindly retry an append: an ambiguous response may already
            # have added annotations, and retrying would duplicate them.
            raise RuntimeError("Checks API connection failed; publication may be partial") from None


class Publication:
    def __init__(self, api, name, head_sha, details_url, exit_code, summary, annotations):
        if not re.fullmatch(r"[0-9a-f]{40}", head_sha):
            raise ValueError("head-sha must be a full GitHub commit SHA")
        self.api, self.name, self.head_sha = api, name, head_sha
        self.details_url, self.exit_code, self.summary = details_url, exit_code, summary
        self.annotations = annotations
        self.check = None
        self.published = 0

    def output(self, batch):
        summary = self.summary + f"\n\nAnnotations published: {self.published + len(batch)}/{len(self.annotations)}."
        return dict(title=self.name, summary=limited(summary, 65535), annotations=batch)

    def publish(self):
        batches = [self.annotations[i:i + 50] for i in range(0, len(self.annotations), 50)] or [[]]
        for index, batch in enumerate(batches):
            complete = index == len(batches) - 1
            payload = dict(status="completed" if complete else "in_progress", output=self.output(batch))
            if complete:
                payload["conclusion"] = "failure" if self.exit_code else "success"
            if index == 0:
                payload.update(name=self.name, head_sha=self.head_sha, details_url=self.details_url,
                               external_id=f'{os.getenv("GITHUB_RUN_ID", "local")}:{os.getenv("GITHUB_RUN_ATTEMPT", "1")}')
            try:
                self.check = self.api.request("POST" if index == 0 else "PATCH",
                                              "" if index == 0 else "/" + str(self.check["id"]), payload)
            except RuntimeError:
                self.mark_incomplete()
                raise
            self.published += len(batch)

    def mark_incomplete(self):
        if self.check is None:
            return
        try:
            self.api.request("PATCH", "/" + str(self.check["id"]),
                             dict(status="completed", conclusion="failure",
                                  output=dict(title="Columbo publication incomplete",
                                              summary=f"Only {self.published}/{len(self.annotations)} annotations were confirmed. See workflow evidence.")))
        except RuntimeError:
            pass  # Preserve the original publishing error; never claim success.

    def evidence(self):
        return dict(head_sha=self.head_sha, analyzer_exit_code=self.exit_code,
                    expected_annotations=len(self.annotations), published_annotations=self.published,
                    check_id=self.check["id"] if self.check else None,
                    check_url=self.check["html_url"] if self.check else None)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--snapshot", required=True)
    parser.add_argument("--exit-code", required=True, type=int, choices=(0, 1, 2))
    parser.add_argument("--head-sha", required=True)
    parser.add_argument("--name", default="Columbo findings")
    parser.add_argument("--details-url", required=True)
    parser.add_argument("--publication-status", default="")
    args = parser.parse_args()
    publication = None
    try:
        annotations, summary = read_snapshot(args.snapshot, args.exit_code)
        api = ChecksAPI(os.environ.get("GITHUB_REPOSITORY", ""), os.environ.get("GH_TOKEN", ""))
        publication = Publication(api, args.name, args.head_sha, args.details_url, args.exit_code, summary, annotations)
        publication.publish()
        evidence = publication.evidence()
        print(f'Published {publication.published} Columbo annotations: {evidence["check_url"]}')
        if os.environ.get("GITHUB_OUTPUT"):
            with open(os.environ["GITHUB_OUTPUT"], "a") as output:
                output.write(f'check-url={evidence["check_url"]}\nannotation-count={publication.published}\n')
        return 0
    except (ValueError, OSError, sqlite3.Error, RuntimeError) as error:
        print(f"Columbo check publication failed: {error}", file=sys.stderr)
        return 1
    finally:
        if args.publication_status and publication is not None:
            path = Path(args.publication_status)
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(json.dumps(publication.evidence(), indent=2) + "\n")


if __name__ == "__main__":
    sys.exit(main())
