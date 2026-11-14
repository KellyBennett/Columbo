#!/usr/bin/env python3
"""Comment on the Columbo cases a pull request introduced or made worse.

Reviewers react to each comment. Advisory only: this never changes the check's
conclusion or the job's exit status. Standard library only.
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass, field
import enum
import json
import os
from pathlib import Path, PurePosixPath
import re
import sqlite3
import sys
import time
from typing import Optional, Union
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from columbo_snapshot import limited, number_text, open_snapshot

REACTION_PROMPT = "React 👍 if this finding is right, 👎 if it's a nitpick."
WRITE_SPACING_SECONDS = 1.0


@dataclass(frozen=True)
class Span:
    path: str
    start: int
    end: int


@dataclass(frozen=True)
class Limit:
    kind: str
    subject: str
    value: Union[int, float]
    limit: Union[int, float]
    operator: str

    @property
    def excess(self):
        if self.operator in (">", ">="):
            return self.value - self.limit
        if self.operator in ("<", "<="):
            return self.limit - self.value
        return None


@dataclass(frozen=True)
class Case:
    id: str
    smell: str
    verdict: str
    symbol: str
    suppressed: bool
    primary: Span
    why: str
    diagnosis: str
    metrics: tuple = ()
    guidance: tuple = ()
    evidence: tuple = ()
    first_limit: Optional[Limit] = None
    receipts: tuple = ()


class Change(enum.Enum):
    INTRODUCED = "introduced"
    WORSENED = "made worse"


@dataclass(frozen=True)
class Finding:
    case: Case
    change: Change
    base_limit: Optional[Limit] = None


def compare(base, head):
    """Unsuppressed head cases that are new at head or moved further past their first limit."""
    findings = []
    for case in head.values():
        if case.suppressed:
            continue
        before = base.get(case.id)
        if before is None:
            findings.append(Finding(case, Change.INTRODUCED))
        elif worsened(before.first_limit, case.first_limit):
            findings.append(Finding(case, Change.WORSENED, before.first_limit))
    return findings


def worsened(before, after):
    if before is None or after is None or before.kind != after.kind:
        return False
    if before.excess is None or after.excess is None:
        return False
    return after.excess > before.excess


@dataclass(frozen=True)
class LineAnchor:
    path: str
    line: int


@dataclass(frozen=True)
class FileAnchor:
    path: str


@dataclass(frozen=True)
class ConversationAnchor:
    pass


Anchor = Union[LineAnchor, FileAnchor, ConversationAnchor]

HUNK = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@")


def added_lines(patch):
    """Right-side line numbers of the lines a unified diff patch adds."""
    added = set()
    line = None
    for text in (patch or "").split("\n"):
        header = HUNK.match(text)
        if header:
            line = int(header.group(1))
        elif line is None or text.startswith("\\"):
            continue
        elif text.startswith("+"):
            added.add(line)
            line += 1
        elif not text.startswith("-"):
            line += 1
    return added


def pull_request_diff(files):
    """Map each path in the diff to its added lines. A file with no patch has none."""
    return {item["filename"]: frozenset(added_lines(item.get("patch"))) for item in files}


def anchor(case, diff):
    for span in (*case.receipts, case.primary):
        changed = sorted(line for line in diff.get(span.path, ()) if span.start <= line <= span.end)
        if changed:
            return LineAnchor(span.path, changed[0])
    for span in (case.primary, *case.receipts):
        if span.path in diff:
            return FileAnchor(span.path)
    return ConversationAnchor()


def repository_path(working_directory, path):
    """Snapshot paths are relative to the analyzed module; the diff's are relative to the repository."""
    joined = PurePosixPath(os.path.normpath(PurePosixPath(working_directory or ".") / path))
    return joined.as_posix()


class State(enum.Enum):
    OPEN = "open"
    CLEARED = "cleared"


MARKER = re.compile(r"\A<!-- columbo-feedback case=(\S+) role=finding state=(open|cleared) -->")


def marker(case_id, state):
    return f"<!-- columbo-feedback case={case_id} role=finding state={state.value} -->"


def parse_marker(body):
    match = MARKER.match(body or "")
    return (match.group(1), State(match.group(2))) if match else None


def with_state(body, state):
    case_id, _ = parse_marker(body)
    return MARKER.sub(marker(case_id, state), body, count=1)


def render(finding):
    case = finding.case
    change = finding.change.value
    if finding.change is Change.WORSENED:
        before, after = finding.base_limit, case.first_limit
        change += (f": {after.kind} {after.subject} {number_text(after.kind, before.value)} → "
                   f"{number_text(after.kind, after.value)}, limit {after.operator} {number_text(after.kind, after.limit)}")
    parts = [f"{case.verdict}, case `{case.id}`.", case.why, case.diagnosis]
    if case.metrics:
        parts.append("\n".join(f"- {metric}" for metric in case.metrics))
    for kind, title in (("lead", "Leads"), ("avoid", "Avoid")):
        items = [item for item_kind, item in case.guidance if item_kind == kind]
        if items:
            parts.append(f"**{title}**\n" + "\n".join(f"- {item}" for item in items))
    if case.evidence:
        parts.append("<details><summary>Evidence</summary>\n\n```text\n" + "\n".join(case.evidence) + "\n```\n</details>")
    # GitHub caps a comment at 65536 characters; the marker and prompt must survive truncation.
    middle = limited("\n\n".join(part for part in parts if part), 60000)
    return "\n".join([marker(case.id, State.OPEN),
                      f"**Columbo: {case.smell}** in `{case.symbol}` ({change})",
                      "", middle, "", REACTION_PROMPT])


class Kind(enum.Enum):
    REVIEW = "review"
    ISSUE = "issue"


@dataclass(frozen=True)
class Thread:
    id: str
    resolved: bool


@dataclass(frozen=True)
class Posted:
    kind: Kind
    id: int
    body: str
    case_id: str
    state: State
    thread: Optional[Thread] = None


def collect_posted(review_comments, issue_comments, threads):
    """Columbo's finding comments keyed by case ID. The oldest wins if a case has several."""
    thread_of = {}
    for thread in threads:
        for comment in thread["comments"]["nodes"]:
            thread_of[comment["databaseId"]] = Thread(thread["id"], thread["isResolved"])
    posted = {}
    for kind, comments in ((Kind.REVIEW, review_comments), (Kind.ISSUE, issue_comments)):
        for comment in sorted(comments, key=lambda item: item["id"]):
            parsed = parse_marker(comment.get("body"))
            if parsed and parsed[0] not in posted:
                posted[parsed[0]] = Posted(kind, comment["id"], comment["body"], parsed[0], parsed[1],
                                           thread_of.get(comment["id"]) if kind is Kind.REVIEW else None)
    return posted


@dataclass(frozen=True)
class Create:
    case_id: str
    body: str
    anchor: Anchor


@dataclass(frozen=True)
class Edit:
    comment: Posted
    body: str


@dataclass(frozen=True)
class Reply:
    comment: Posted
    body: str


@dataclass(frozen=True)
class Resolve:
    thread: Thread


@dataclass(frozen=True)
class Unresolve:
    thread: Thread


class Outcome(enum.Enum):
    CREATED = "created"
    UPDATED = "updated"
    REOPENED = "reopened"
    CLEARED = "cleared"


@dataclass
class Plan:
    actions: list = field(default_factory=list)
    outcomes: dict = field(default_factory=dict)

    def counts(self):
        return {outcome.value: sum(1 for value in self.outcomes.values() if value is outcome) for outcome in Outcome}


def same_text(left, right):
    return left.replace("\r\n", "\n").rstrip() == right.replace("\r\n", "\n").rstrip()


def clearing_note(case, sha):
    if case is None:
        return f"Cleared in {sha}."
    if case.suppressed:
        return f"Suppressed in {sha}."
    return f"No longer made worse by this pull request as of {sha}."


def plan(findings, diff, posted, head, sha):
    """The writes that make the pull request show exactly one current comment per case.

    The marker's state is written last, so an interrupted run repeats its
    replies rather than skipping them."""
    result = Plan()
    reported = set()
    for finding in findings:
        case_id = finding.case.id
        reported.add(case_id)
        body = render(finding)
        existing = posted.get(case_id)
        if existing is None:
            result.actions.append(Create(case_id, body, anchor(finding.case, diff)))
            result.outcomes[case_id] = Outcome.CREATED
        elif existing.state is State.CLEARED:
            if existing.thread and existing.thread.resolved:
                result.actions.append(Unresolve(existing.thread))
            if existing.kind is Kind.REVIEW:
                result.actions.append(Reply(existing, f"Reported again in {sha}."))
            result.actions.append(Edit(existing, body))
            result.outcomes[case_id] = Outcome.REOPENED
        elif not same_text(existing.body, body):
            result.actions.append(Edit(existing, body))
            result.outcomes[case_id] = Outcome.UPDATED
    for case_id, existing in posted.items():
        if case_id in reported or existing.state is State.CLEARED:
            continue
        note = clearing_note(head.get(case_id), sha)
        cleared = with_state(existing.body, State.CLEARED)
        if existing.kind is Kind.REVIEW:
            result.actions.append(Reply(existing, note))
            if existing.thread and not existing.thread.resolved:
                result.actions.append(Resolve(existing.thread))
            result.actions.append(Edit(existing, cleared))
        else:
            result.actions.append(Edit(existing, cleared.rstrip() + "\n\n" + note))
        result.outcomes[case_id] = Outcome.CLEARED
    return result


def read_cases(path, working_directory):
    with open_snapshot(path) as snapshot:
        db = snapshot.db
        cases = {}
        for row in db.execute("""
            SELECT c.*,d.symbol,f.path FROM cases c
            JOIN declarations d ON d.id=c.primary_declaration_id
            JOIN files f ON f.id=d.file_id ORDER BY c.ordinal
        """):
            receipts = tuple(Span(repository_path(working_directory, item["path"]), item["start_line"], item["end_line"])
                             for item in db.execute("""
                                 SELECT f.path,r.start_line,r.end_line FROM source_receipts r
                                 JOIN files f ON f.id=r.file_id WHERE r.case_id=? ORDER BY r.ordinal
                             """, (row["id"],)))
            limit = db.execute("""
                SELECT kind,subject,numeric_value,limit_value,operator FROM clues
                WHERE case_id=? AND limit_value IS NOT NULL ORDER BY ordinal LIMIT 1
            """, (row["id"],)).fetchone()
            cases[row["id"]] = Case(
                id=row["id"], smell=row["smell"], verdict=row["verdict"], symbol=row["symbol"],
                suppressed=bool(row["suppressed"]),
                primary=Span(repository_path(working_directory, row["path"]), row["start_line"], row["end_line"]),
                why=row["why"], diagnosis=row["diagnosis"],
                metrics=tuple(snapshot.metrics(row["id"])), guidance=tuple(snapshot.guidance_items(row["id"])),
                evidence=tuple(snapshot.supporting_evidence(row["id"])),
                first_limit=Limit(*limit) if limit else None, receipts=receipts)
        return cases


class GitHubError(Exception):
    pass


class PermissionDenied(GitHubError):
    pass


class Unprocessable(GitHubError):
    pass


THREADS_QUERY = """
query($owner: String!, $name: String!, $number: Int!, $cursor: String) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      reviewThreads(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
        nodes { id isResolved comments(first: 1) { nodes { databaseId } } }
      }
    }
  }
}"""
RESOLVE = "mutation($id: ID!) { resolveReviewThread(input: {threadId: $id}) { thread { id } } }"
UNRESOLVE = "mutation($id: ID!) { unresolveReviewThread(input: {threadId: $id}) { thread { id } } }"


class GitHubClient:
    def __init__(self, token, api_url="https://api.github.com", graphql_url="https://api.github.com/graphql",
                 sleep=time.sleep, clock=time.monotonic):
        if not token:
            raise ValueError("GH_TOKEN is required with pull-requests: write permission")
        self.token, self.api_url, self.graphql_url = token, api_url.rstrip("/"), graphql_url
        self.sleep, self.clock = sleep, clock
        self.last_write = None

    def send(self, method, url, payload=None, write=None):
        write = method != "GET" if write is None else write
        if write and self.last_write is not None:
            # GitHub asks integrations to space content-creating requests at least a second apart.
            self.sleep(max(0.0, WRITE_SPACING_SECONDS - (self.clock() - self.last_write)))
        request = Request(url, data=None if payload is None else json.dumps(payload).encode(), method=method,
                          headers={"Authorization": "Bearer " + self.token,
                                   "Accept": "application/vnd.github+json",
                                   "Content-Type": "application/json",
                                   "X-GitHub-Api-Version": "2022-11-28",
                                   "User-Agent": "Columbo-feedback-publisher"})
        try:
            with urlopen(request, timeout=30) as response:
                body = response.read()
                links = response.headers.get("Link", "")
        except HTTPError as error:
            # Never echo the request or response headers: they carry the credential.
            # GitHub also answers 403 when a rate limit is hit; only a plain 403 means a missing permission.
            if error.code == 403 and not (error.headers and (error.headers.get("Retry-After")
                                                           or error.headers.get("X-RateLimit-Remaining") == "0")):
                raise PermissionDenied(f"{method} returned HTTP 403") from None
            if error.code == 422:
                raise Unprocessable(f"{method} returned HTTP 422") from None
            raise GitHubError(f"{method} returned HTTP {error.code}") from None
        except URLError:
            raise GitHubError(f"{method} connection failed") from None
        finally:
            if write:
                self.last_write = self.clock()
        return (json.loads(body) if body else None), links

    def rest(self, method, path, payload=None):
        return self.send(method, self.api_url + path, payload)[0]

    def pages(self, path):
        url = self.api_url + path + "?per_page=100"
        items = []
        while url:
            page, links = self.send("GET", url)
            items.extend(page)
            following = re.search(r'<([^>]+)>;\s*rel="next"', links)
            url = following.group(1) if following else None
        return items

    def graphql(self, query, variables, write=False):
        result, _ = self.send("POST", self.graphql_url, {"query": query, "variables": variables}, write)
        errors = result.get("errors") or []
        if any(error.get("type") == "FORBIDDEN" for error in errors):
            raise PermissionDenied("GraphQL request was forbidden")
        if errors:
            raise GitHubError("GraphQL request failed: " + "; ".join(error.get("message", "") for error in errors))
        return result["data"]


class PullRequest:
    """The pull request operations feedback needs, over GitHub's REST and GraphQL APIs."""

    def __init__(self, client, repository, number, head_sha):
        if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
            raise ValueError("invalid GitHub repository")
        self.client, self.repository, self.number, self.head_sha = client, repository, int(number), head_sha
        self.prefix = f"/repos/{repository}"

    def files(self):
        return self.client.pages(f"{self.prefix}/pulls/{self.number}/files")

    def review_comments(self):
        return self.client.pages(f"{self.prefix}/pulls/{self.number}/comments")

    def issue_comments(self):
        return self.client.pages(f"{self.prefix}/issues/{self.number}/comments")

    def review_threads(self):
        owner, name = self.repository.split("/")
        threads, cursor = [], None
        while True:
            data = self.client.graphql(THREADS_QUERY, {"owner": owner, "name": name, "number": self.number, "cursor": cursor})
            page = data["repository"]["pullRequest"]["reviewThreads"]
            threads.extend(page["nodes"])
            if not page["pageInfo"]["hasNextPage"]:
                return threads
            cursor = page["pageInfo"]["endCursor"]

    def create_review_comment(self, body, path, line=None):
        payload = {"body": body, "commit_id": self.head_sha, "path": path}
        payload.update({"line": line, "side": "RIGHT"} if line else {"subject_type": "file"})
        return self.client.rest("POST", f"{self.prefix}/pulls/{self.number}/comments", payload)

    def create_issue_comment(self, body):
        return self.client.rest("POST", f"{self.prefix}/issues/{self.number}/comments", {"body": body})

    def edit_comment(self, kind, comment_id, body):
        area = "pulls" if kind is Kind.REVIEW else "issues"
        return self.client.rest("PATCH", f"{self.prefix}/{area}/comments/{int(comment_id)}", {"body": body})

    def reply(self, comment_id, body):
        return self.client.rest("POST", f"{self.prefix}/pulls/{self.number}/comments/{int(comment_id)}/replies", {"body": body})

    def set_thread_resolved(self, thread_id, resolved):
        self.client.graphql(RESOLVE if resolved else UNRESOLVE, {"id": thread_id}, write=True)


def execute(actions, pull_request):
    for action in actions:
        if isinstance(action, Create):
            create(action, pull_request)
        elif isinstance(action, Edit):
            pull_request.edit_comment(action.comment.kind, action.comment.id, action.body)
        elif isinstance(action, Reply):
            pull_request.reply(action.comment.id, action.body)
        elif isinstance(action, (Resolve, Unresolve)):
            pull_request.set_thread_resolved(action.thread.id, isinstance(action, Resolve))


def create(action, pull_request):
    target = action.anchor
    # GitHub refuses a line the diff does not show (422), as when the base branch
    # moved since the merge commit was analyzed. Fall back to coarser anchors.
    if isinstance(target, LineAnchor):
        try:
            return pull_request.create_review_comment(action.body, target.path, target.line)
        except Unprocessable:
            target = FileAnchor(target.path)
    if isinstance(target, FileAnchor):
        try:
            return pull_request.create_review_comment(action.body, target.path)
        except Unprocessable:
            pass
    return pull_request.create_issue_comment(action.body)


@dataclass(frozen=True)
class Event:
    number: int
    head_sha: str
    head_repository: str


def read_event(path):
    payload = json.loads(Path(path).read_text())
    pull = payload.get("pull_request")
    if not pull:
        raise ValueError("the workflow event is not a pull request")
    return Event(pull["number"], pull["head"]["sha"], (pull["head"].get("repo") or {}).get("full_name", ""))


@dataclass
class Report:
    head_sha: str = ""
    skipped: Optional[str] = None
    findings: int = 0
    outcomes: dict = field(default_factory=dict)
    warning: Optional[str] = None

    def summary_line(self):
        if self.skipped:
            return f"Columbo feedback skipped: {self.skipped}"
        counts = ", ".join(f"{count} {name}" for name, count in self.outcomes.items())
        return f"Columbo feedback: {self.findings} findings introduced or made worse; {counts}."


def publish(arguments, environment, open_pull_request):
    report = Report()
    if arguments.head_exit_code == 2:
        report.skipped = "head analysis failed."
        return report
    if arguments.base_status == "unavailable" or not arguments.base_snapshot:
        report.skipped = "the base commit could not be analyzed."
        return report
    event = read_event(environment["GITHUB_EVENT_PATH"])
    report.head_sha = event.head_sha
    repository = environment.get("GITHUB_REPOSITORY", "")
    if event.head_repository != repository:
        report.skipped = "pull requests from forks cannot receive comments with the workflow token."
        report.warning = "Columbo feedback skipped: " + report.skipped
        return report
    head = read_cases(arguments.head_snapshot, arguments.working_directory)
    base = read_cases(arguments.base_snapshot, arguments.working_directory)
    findings = compare(base, head)
    report.findings = len(findings)
    pull_request = open_pull_request(repository, event)
    posted = collect_posted(pull_request.review_comments(), pull_request.issue_comments(), pull_request.review_threads())
    result = plan(findings, pull_request_diff(pull_request.files()), posted, head, event.head_sha)
    execute(result.actions, pull_request)
    report.outcomes = result.counts()
    return report


def main(argv=None, environment=None, open_pull_request=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--head-snapshot", required=True)
    parser.add_argument("--head-exit-code", required=True, type=int)
    parser.add_argument("--base-snapshot", default="")
    parser.add_argument("--base-status", default="")
    parser.add_argument("--working-directory", default=".")
    parser.add_argument("--evidence", default="")
    arguments = parser.parse_args(argv)
    environment = os.environ if environment is None else environment
    if open_pull_request is None:
        def open_pull_request(repository, event):
            client = GitHubClient(environment.get("GH_TOKEN", ""),
                                  environment.get("GITHUB_API_URL", "https://api.github.com"),
                                  environment.get("GITHUB_GRAPHQL_URL", "https://api.github.com/graphql"))
            return PullRequest(client, repository, event.number, event.head_sha)
    try:
        report = publish(arguments, environment, open_pull_request)
    except PermissionDenied:
        report = Report(skipped="the token cannot write pull request comments.",
                        warning="Columbo feedback needs the pull-requests: write permission. Grant it to the job to receive comments.")
    except (GitHubError, ValueError, KeyError, TypeError, OSError, sqlite3.Error) as error:
        report = Report(skipped=f"{error}", warning=f"Columbo feedback failed: {error}")
    if report.warning:
        print("::warning::" + report.warning)
    print(report.summary_line())
    if environment.get("GITHUB_STEP_SUMMARY"):
        with open(environment["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write(report.summary_line() + "\n")
    if arguments.evidence:
        Path(arguments.evidence).write_text(json.dumps(dict(
            head_sha=report.head_sha, skipped=report.skipped, findings=report.findings,
            outcomes=report.outcomes), indent=2) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
