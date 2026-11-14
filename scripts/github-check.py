#!/usr/bin/env python3
"""Read an immutable Columbo snapshot and publish all findings as a check.

This is a GitHub integration adapter, not a second analyzer or report format.
It requires only Python's standard library and a checks:write workflow token.
"""

import argparse
import json
import os
from pathlib import Path
import re
import sqlite3
import sys
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

import columbo_snapshot
from columbo_snapshot import limited


class Snapshot(columbo_snapshot.Snapshot):
    def annotations(self):
        if self.stages():
            return self.stage_annotations()
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
        message = "\n".join([f'CASE {row["id"]} {row["symbol"]}: {row["verdict"]}',
                              row["why"], row["diagnosis"], *self.metrics(row["id"]),
                              *self.supporting_evidence(row["id"]), *self.guidance(row["id"])])
        return dict(path=row["path"], start_line=row["start_line"], end_line=row["end_line"],
                    annotation_level="failure" if row["verdict"] == "FAIL" else "warning",
                    title=limited("Columbo: " + row["smell"], 255), message=limited(message))

    def summary(self, exit_code):
        if self.stages():
            return self.stage_summary(exit_code)
        totals = tuple(self.db.execute("SELECT failed,warned,suppressed FROM summary").fetchone())
        expected = 1 if totals[0] else 0
        if exit_code != expected:
            raise ValueError("analysis exit code disagrees with stored verdicts")
        text = f"{totals[0]} failed, {totals[1]} warned, {totals[2]} suppressed."
        warnings = [row[0] for row in self.db.execute("SELECT message FROM warnings ORDER BY ordinal")]
        if warnings:
            text += "\n\nAnalysis warnings:\n" + "\n".join(warnings)
        text += self.advisory_summary()
        return limited(text)

    def stages(self):
        return self.db.execute("SELECT * FROM refactoring_stages ORDER BY ordinal").fetchall()

    def stage_annotations(self):
        result = []
        for group in self.db.execute("SELECT * FROM active_stage_issues ORDER BY kind,id"):
            for site in self.db.execute("""
                SELECT f.path,min(r.start_line) AS line
                FROM advisory_sites s JOIN declarations d ON d.id=s.declaration_id
                JOIN files f ON f.id=d.file_id
                JOIN advisory_receipts r ON r.group_id=s.group_id AND r.site_ordinal=s.ordinal
                WHERE s.group_id=? GROUP BY s.ordinal ORDER BY s.ordinal
            """, (group["id"],)):
                result.append(dict(path=site["path"], start_line=site["line"], end_line=site["line"],
                                   annotation_level="failure", title=limited("Columbo: " + group["kind"], 255),
                                   message=limited(group["subject"] + "\n" + group["lead"] + "\n" + group["limits"])))
        return result

    def stage_summary(self, exit_code):
        stages = self.stages()
        active = [stage for stage in stages if stage["state"] == "active"]
        if len(active) > 1 or (not active and any(stage["state"] != "cleared" for stage in stages)):
            raise ValueError("invalid active refactoring stage")
        if exit_code != (1 if active and active[0]["issue_count"] else 0):
            raise ValueError("analysis exit code disagrees with stored stage issues")
        lines = ["Refactoring mode: staged. Legacy verdicts remain in SQLite and are not enforced.", "Cleared means only that configured collectors found no issues; it does not prove architectural ownership or correctness."]
        for stage in stages:
            lines.append(f'Stage {stage["name"]}: {stage["state"]}')
            if stage["state"] == "active":
                lines.append(stage["task"])
                if stage["pending_definition"]:
                    lines.append("Pending definition; this is not completion.")
            for member in self.db.execute("SELECT * FROM stage_collectors WHERE stage_id=? ORDER BY ordinal", (stage["id"],)):
                lines.append(f'  Collector {member["collector"]}: {member["issue_count"]} issues')
        for group in self.db.execute("SELECT * FROM active_stage_issues ORDER BY kind,id"):
            lines.append(f'Evidence {group["id"]} {group["kind"]}: {group["subject"]}')
        lines.extend(row[0] for row in self.db.execute("SELECT message FROM warnings ORDER BY ordinal"))
        return limited("\n\n".join(lines))

    def advisory_summary(self):
        lines = []
        for group in self.db.execute("SELECT id,kind,subject,lead FROM advisory_groups ORDER BY kind,id"):
            lines.append(f'Evidence {group["id"]} {group["kind"]}: {group["subject"]}\n{group["lead"]}')
            for site in self.db.execute("""
                SELECT d.symbol,f.path,min(r.start_line) AS line
                FROM advisory_sites s JOIN declarations d ON d.id=s.declaration_id
                JOIN files f ON f.id=d.file_id
                JOIN advisory_receipts r ON r.group_id=s.group_id AND r.site_ordinal=s.ordinal
                WHERE s.group_id=? GROUP BY s.ordinal ORDER BY s.ordinal
            """, (group["id"],)):
                lines.append(f'  {site["path"]}:{site["line"]} {site["symbol"]}')
        return "\n\n" + "\n".join(lines) if lines else ""


def read_snapshot(path, exit_code):
    if exit_code == 2:
        return [], "Analysis failed (exit 2). No finding annotations were published; see the workflow log."
    with columbo_snapshot.open_snapshot(path, Snapshot) as snapshot:
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
