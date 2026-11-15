"""Pull request feedback against real snapshots and an in-memory pull request."""

import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shutil
import sqlite3
import sys
import tempfile
import unittest
from unittest.mock import patch
from urllib.error import HTTPError

spec = importlib.util.spec_from_file_location("github_feedback", Path(__file__).with_name("github-feedback.py"))
feedback = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = feedback
spec.loader.exec_module(feedback)

HEAD_SHA = "c" * 40
PARAMETERS = "C-ad03ea5957"
LONG_FUNCTION = "C-b10bccb46b"
FEATURE_ENVY = "C-4063b2b57c"


def rename_case(path, old, new):
    with contextlib.closing(sqlite3.connect(path)) as db, db:
        tables = [row[0] for row in db.execute(
            "SELECT m.name FROM sqlite_master m, pragma_table_info(m.name) p WHERE m.type='table' AND p.name='case_id'")]
        for table in tables:
            db.execute(f"UPDATE {table} SET case_id=? WHERE case_id=?", (new, old))
        db.execute("UPDATE cases SET id=? WHERE id=?", (new, old))


def edit(path, *statements):
    with contextlib.closing(sqlite3.connect(path)) as db, db:
        for statement in statements:
            db.execute(*statement) if isinstance(statement, tuple) else db.execute(statement)


class FakePullRequest:
    """Stores comments and threads the way GitHub reports them back."""

    def __init__(self, files, forbid_writes=False, reject_lines=False, forbid_resolve=False):
        self.changed_files = files
        self.forbid_resolve = forbid_resolve
        self.forbid_writes, self.reject_lines = forbid_writes, reject_lines
        self.review, self.issue, self.threads = {}, {}, {}
        self.writes = []
        self.next_id = 1000

    def write(self, *record):
        if self.forbid_writes:
            raise feedback.PermissionDenied("HTTP 403")
        self.writes.append(record)
        self.next_id += 1
        return self.next_id

    def files(self):
        return self.changed_files

    def review_comments(self):
        return [dict(id=key, body=value["body"]) for key, value in self.review.items()]

    def issue_comments(self):
        return [dict(id=key, body=value["body"]) for key, value in self.issue.items()]

    def review_threads(self):
        return [dict(id=key, isResolved=value["resolved"], comments=dict(nodes=[dict(databaseId=value["comments"][0])]))
                for key, value in self.threads.items()]

    def create_review_comment(self, body, path, line=None):
        if line and self.reject_lines:
            raise feedback.Unprocessable("HTTP 422")
        comment = self.write("create-review", path, line)
        self.review[comment] = dict(body=body, path=path, line=line)
        self.threads[f"T{comment}"] = dict(resolved=False, comments=[comment])
        return dict(id=comment)

    def create_issue_comment(self, body):
        comment = self.write("create-issue")
        self.issue[comment] = dict(body=body)
        return dict(id=comment)

    def edit_comment(self, kind, comment_id, body):
        self.write("edit", kind.value, comment_id)
        (self.review if kind is feedback.Kind.REVIEW else self.issue)[comment_id]["body"] = body

    def reply(self, comment_id, body):
        reply = self.write("reply", comment_id, body)
        self.review[reply] = dict(body=body, in_reply_to=comment_id)
        next(thread for thread in self.threads.values() if comment_id in thread["comments"])["comments"].append(reply)

    def set_thread_resolved(self, thread_id, resolved):
        if self.forbid_resolve:
            raise feedback.PermissionDenied("GraphQL request was forbidden")
        self.write("resolve" if resolved else "unresolve", thread_id)
        self.threads[thread_id]["resolved"] = resolved

    def finding(self, case_id):
        for kind, store in ((feedback.Kind.REVIEW, self.review), (feedback.Kind.ISSUE, self.issue)):
            for key, value in store.items():
                parsed = feedback.parse_marker(value["body"])
                if parsed and parsed[0] == case_id:
                    return kind, key, value, parsed[1]
        return None

    def thread_of(self, comment_id):
        return next(thread for thread in self.threads.values() if comment_id in thread["comments"])

    def replies_to(self, comment_id):
        return [value["body"] for value in self.review.values() if value.get("in_reply_to") == comment_id]


def patch_adding(*lines):
    return "\n".join(f"@@ -{line},0 +{line} @@\n+added" for line in lines)


class FeedbackTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        fixture = Path(os.environ["COLUMBO_TEST_SNAPSHOT"])
        self.head = self.root / "head.sqlite"
        self.base = self.root / "base.sqlite"
        shutil.copyfile(fixture, self.head)
        shutil.copyfile(fixture, self.base)
        rename_case(self.base, PARAMETERS, "C-0000000001")
        edit(self.base, f"UPDATE clues SET numeric_value=10 WHERE case_id='{LONG_FUNCTION}' AND ordinal=0")
        event = self.root / "event.json"
        event.write_text(json.dumps(dict(pull_request=dict(
            number=7, head=dict(sha=HEAD_SHA, repo=dict(full_name="example/repo"))))))
        self.environment = dict(GITHUB_EVENT_PATH=str(event), GITHUB_REPOSITORY="example/repo",
                                GITHUB_STEP_SUMMARY=str(self.root / "summary.md"))

    def tearDown(self):
        self.directory.cleanup()

    def run_feedback(self, pull_request, head=None, sha=HEAD_SHA, extra=(), working_directory="."):
        if sha != HEAD_SHA:
            event = json.loads(Path(self.environment["GITHUB_EVENT_PATH"]).read_text())
            event["pull_request"]["head"]["sha"] = sha
            Path(self.environment["GITHUB_EVENT_PATH"]).write_text(json.dumps(event))
        argv = ["--head-snapshot", str(head or self.head), "--head-exit-code", "1",
                "--base-snapshot", str(self.base), "--working-directory", working_directory,
                "--evidence", str(self.root / "feedback.json"), *extra]
        opened = []

        def open_pull_request(repository, event):
            opened.append((repository, event.number))
            return pull_request
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            self.assertEqual(0, feedback.main(argv, self.environment, open_pull_request))
        self.opened = opened
        self.evidence = json.loads((self.root / "feedback.json").read_text())
        return output.getvalue()

    def cases(self, path=None):
        return feedback.read_cases(path or self.head, ".")

    # What gets reported

    def test_only_introduced_and_worsened_cases_are_reported(self):
        findings = feedback.compare(self.cases(self.base), self.cases())
        self.assertEqual({PARAMETERS: feedback.Change.INTRODUCED, LONG_FUNCTION: feedback.Change.WORSENED},
                         {finding.case.id: finding.change for finding in findings})

    def test_suppressed_head_cases_are_never_reported(self):
        edit(self.head, f"UPDATE cases SET suppressed=1 WHERE id='{PARAMETERS}'")
        findings = feedback.compare(self.cases(self.base), self.cases())
        self.assertEqual([LONG_FUNCTION], [finding.case.id for finding in findings])

    def test_a_case_is_worse_only_when_its_first_limit_moves_further_past(self):
        def case(limit):
            return feedback.Case("C-1", "long-function", "FAIL", "f", False, feedback.Span("a.go", 1, 2),
                                 "why", "diagnosis", first_limit=limit)

        def worse(before, after):
            return bool(feedback.compare({"C-1": case(before)}, {"C-1": case(after)}))
        Limit = feedback.Limit
        self.assertTrue(worse(Limit("lines", "f", 11, 10, ">"), Limit("lines", "f", 12, 10, ">")))
        self.assertFalse(worse(Limit("lines", "f", 12, 10, ">"), Limit("lines", "f", 12, 10, ">")))
        self.assertFalse(worse(Limit("lines", "f", 12, 10, ">"), Limit("lines", "f", 11, 10, ">")))
        self.assertTrue(worse(Limit("lines", "f", 12, 10, ">"), Limit("lines", "f", 12, 9, ">")))
        self.assertTrue(worse(Limit("ratio", "f", 0.5, 0.7, "<="), Limit("ratio", "f", 0.4, 0.7, "<=")))
        self.assertFalse(worse(Limit("lines", "f", 1, 10, ">"), Limit("other", "f", 50, 10, ">")))
        self.assertFalse(worse(Limit("count", "f", 1, 2, "!="), Limit("count", "f", 5, 2, "!=")))
        self.assertFalse(worse(None, Limit("lines", "f", 50, 10, ">")))

    def test_worsened_comment_names_the_moved_limit(self):
        pull_request = FakePullRequest([])
        self.run_feedback(pull_request)
        _, _, comment, _ = pull_request.finding(LONG_FUNCTION)
        self.assertIn("**Columbo: long-function** in `fixture.LongFunction` (made worse: function-lines fixture.LongFunction 10 → 11, limit > 10)",
                      comment["body"])

    # Reading the diff

    def test_added_lines_follow_the_right_side_of_each_hunk(self):
        patch_text = ("@@ -1,3 +1,4 @@\n context\n-removed\n+first\n+second\n context\n"
                      "@@ -40 +41,2 @@\n+third\n context\n\\ No newline at end of file")
        self.assertEqual({2, 3, 41}, feedback.added_lines(patch_text))
        self.assertEqual(set(), feedback.added_lines(None))
        diff = feedback.pull_request_diff([dict(filename="binary.png"), dict(filename="a.go", patch="@@ -0,0 +1 @@\n+x")])
        self.assertEqual({"binary.png": frozenset(), "a.go": frozenset({1})}, diff)

    # Where comments land

    def test_comment_lands_on_the_first_added_line_inside_a_receipt(self):
        case = self.cases()[LONG_FUNCTION]
        self.assertEqual(feedback.LineAnchor("all.go", 18), feedback.anchor(case, {"all.go": frozenset({5, 18, 20})}))

    def test_comment_falls_back_to_the_file_when_no_receipt_line_changed(self):
        case = self.cases()[LONG_FUNCTION]
        self.assertEqual(feedback.FileAnchor("all.go"), feedback.anchor(case, {"all.go": frozenset({200}), "other.go": frozenset({1})}))

    def test_comment_falls_back_to_a_receipt_file_in_the_diff(self):
        case = feedback.Case("C-1", "duplicate-code", "WARN", "f", False, feedback.Span("a.go", 1, 5), "why", "diagnosis",
                             receipts=(feedback.Span("a.go", 1, 5), feedback.Span("b.go", 10, 12)))
        self.assertEqual(feedback.LineAnchor("b.go", 11), feedback.anchor(case, {"b.go": frozenset({11})}))
        self.assertEqual(feedback.FileAnchor("b.go"), feedback.anchor(case, {"b.go": frozenset()}))

    def test_comment_falls_back_to_the_conversation_when_no_file_is_in_the_diff(self):
        pull_request = FakePullRequest([dict(filename="README.md", patch=patch_adding(1))])
        self.run_feedback(pull_request)
        self.assertEqual([("create-issue",), ("create-issue",)], pull_request.writes)

    def test_working_directory_prefixes_snapshot_paths(self):
        cases = feedback.read_cases(self.head, "./services/api/")
        self.assertEqual(feedback.Span("services/api/all.go", 13, 25), cases[LONG_FUNCTION].primary)
        pull_request = FakePullRequest([dict(filename="services/api/all.go", patch=patch_adding(14, 26))])
        self.run_feedback(pull_request, working_directory="services/api")
        self.assertEqual({("create-review", "services/api/all.go", 26), ("create-review", "services/api/all.go", 14)},
                         set(pull_request.writes))

    def test_a_line_github_refuses_falls_back_to_a_file_comment(self):
        pull_request = FakePullRequest([dict(filename="all.go", patch=patch_adding(26))], reject_lines=True)
        self.run_feedback(pull_request)
        self.assertIn(("create-review", "all.go", None), pull_request.writes)

    # The comment

    def test_finding_comment_carries_marker_evidence_and_reaction_prompt(self):
        pull_request = FakePullRequest([dict(filename="all.go", patch=patch_adding(26))])
        output = self.run_feedback(pull_request)
        kind, _, comment, state = pull_request.finding(PARAMETERS)
        body = comment["body"]
        self.assertEqual((feedback.Kind.REVIEW, feedback.State.OPEN, 26), (kind, state, comment["line"]))
        self.assertTrue(body.startswith(f"<!-- columbo-feedback case={PARAMETERS} role=finding state=open -->\n"))
        self.assertIn("**Columbo: long-parameter-list** in `fixture.Parameters` (introduced)", body)
        self.assertIn("parameters fixture.Parameters: 5 (limit > 4)", body)
        self.assertIn("**Leads**", body)
        self.assertTrue(body.endswith("React 👍 if this finding is right, 👎 if it's a nitpick."))
        self.assertEqual({"head_sha": HEAD_SHA, "skipped": None, "findings": 2,
                          "outcomes": {"created": 2, "updated": 0, "reopened": 0, "cleared": 0}}, self.evidence)
        self.assertIn("Columbo feedback: 2 findings introduced or made worse; 2 created", output)
        self.assertIn("2 created", (self.root / "summary.md").read_text())

    def test_long_evidence_is_truncated_inside_the_comment_limit(self):
        edit(self.head, (f"UPDATE cases SET why=? WHERE id='{PARAMETERS}'", ("🙂" * 40000,)))
        pull_request = FakePullRequest([])
        self.run_feedback(pull_request)
        body = pull_request.finding(PARAMETERS)[2]["body"]
        self.assertLessEqual(len(body), 65536)
        self.assertIn("Truncated", body)
        self.assertTrue(body.endswith("nitpick."))

    # Across runs and pushes

    def test_running_twice_on_the_same_commit_writes_nothing_the_second_time(self):
        pull_request = FakePullRequest([dict(filename="all.go", patch=patch_adding(26))])
        self.run_feedback(pull_request)
        self.assertEqual(2, len(pull_request.writes))
        pull_request.writes.clear()
        self.run_feedback(pull_request)
        self.assertEqual([], pull_request.writes)
        self.assertEqual({"created": 0, "updated": 0, "reopened": 0, "cleared": 0}, self.evidence["outcomes"])

    def test_a_changed_finding_edits_its_comment_in_place(self):
        pull_request = FakePullRequest([dict(filename="all.go", patch=patch_adding(26))])
        self.run_feedback(pull_request)
        _, comment_id, _, _ = pull_request.finding(PARAMETERS)
        pull_request.writes.clear()
        edit(self.head, f"UPDATE cases SET verdict='WARN' WHERE id='{PARAMETERS}'")
        self.run_feedback(pull_request, sha="d" * 40)
        self.assertEqual([("edit", "review", comment_id)], pull_request.writes)
        self.assertIn("WARN, case", pull_request.finding(PARAMETERS)[2]["body"])

    def test_a_vanished_case_is_cleared_and_resolved_once(self):
        pull_request = FakePullRequest([dict(filename="all.go", patch=patch_adding(26))])
        self.run_feedback(pull_request)
        _, comment_id, _, _ = pull_request.finding(PARAMETERS)
        later = self.root / "later.sqlite"
        shutil.copyfile(self.head, later)
        rename_case(later, PARAMETERS, "C-0000000002")
        self.run_feedback(pull_request, head=later, sha="d" * 40)
        self.assertEqual(feedback.State.CLEARED, pull_request.finding(PARAMETERS)[3])
        self.assertEqual([f"Cleared in {'d' * 40}."], pull_request.replies_to(comment_id))
        self.assertTrue(pull_request.thread_of(comment_id)["resolved"])
        self.assertEqual(1, self.evidence["outcomes"]["cleared"])
        pull_request.writes.clear()
        self.run_feedback(pull_request, head=later, sha="d" * 40)
        self.assertEqual([], pull_request.writes)

    def test_a_token_that_cannot_resolve_threads_still_clears_once(self):
        pull_request = FakePullRequest([dict(filename="all.go", patch=patch_adding(26))], forbid_resolve=True)
        self.run_feedback(pull_request)
        _, comment_id, _, _ = pull_request.finding(PARAMETERS)
        later = self.root / "later.sqlite"
        shutil.copyfile(self.head, later)
        rename_case(later, PARAMETERS, "C-0000000002")
        output = self.run_feedback(pull_request, head=later, sha="d" * 40)
        self.assertEqual(feedback.State.CLEARED, pull_request.finding(PARAMETERS)[3])
        self.assertEqual([f"Cleared in {'d' * 40}."], pull_request.replies_to(comment_id))
        self.assertFalse(pull_request.thread_of(comment_id)["resolved"])
        self.assertIn("contents: write", output)
        pull_request.writes.clear()
        self.run_feedback(pull_request, head=later, sha="d" * 40)
        self.assertEqual([], pull_request.writes)

    def test_a_case_no_longer_made_worse_says_so(self):
        pull_request = FakePullRequest([])
        self.run_feedback(pull_request)
        shutil.copyfile(self.head, self.base)
        _, comment_id, _, _ = pull_request.finding(PARAMETERS)
        self.run_feedback(pull_request, sha="e" * 40)
        self.assertTrue(pull_request.issue[comment_id]["body"].endswith(
            f"No longer made worse by this pull request as of {'e' * 40}."))
        self.assertEqual(feedback.State.CLEARED, pull_request.finding(PARAMETERS)[3])

    def test_a_suppressed_case_is_cleared_as_suppressed(self):
        pull_request = FakePullRequest([dict(filename="all.go", patch=patch_adding(26))])
        self.run_feedback(pull_request)
        _, comment_id, _, _ = pull_request.finding(PARAMETERS)
        edit(self.head, f"UPDATE cases SET suppressed=1 WHERE id='{PARAMETERS}'")
        self.run_feedback(pull_request, sha="f" * 40)
        self.assertEqual([f"Suppressed in {'f' * 40}."], pull_request.replies_to(comment_id))

    def test_a_case_that_returns_reopens_its_one_comment(self):
        pull_request = FakePullRequest([dict(filename="all.go", patch=patch_adding(26))])
        self.run_feedback(pull_request)
        _, comment_id, _, _ = pull_request.finding(PARAMETERS)
        later = self.root / "later.sqlite"
        shutil.copyfile(self.head, later)
        rename_case(later, PARAMETERS, "C-0000000002")
        self.run_feedback(pull_request, head=later, sha="d" * 40)
        self.run_feedback(pull_request, sha="e" * 40)
        kind, reopened_id, _, state = pull_request.finding(PARAMETERS)
        self.assertEqual((comment_id, feedback.State.OPEN), (reopened_id, state))
        self.assertFalse(pull_request.thread_of(comment_id)["resolved"])
        self.assertEqual(f"Reported again in {'e' * 40}.", pull_request.replies_to(comment_id)[-1])
        self.assertEqual(1, self.evidence["outcomes"]["reopened"])
        self.assertEqual(1, sum(1 for value in pull_request.review.values()
                                if (feedback.parse_marker(value["body"]) or ("",))[0] == PARAMETERS))
        pull_request.writes.clear()
        self.run_feedback(pull_request, sha="e" * 40)
        self.assertEqual([], pull_request.writes)

    # When nothing is posted

    def test_unavailable_base_posts_nothing(self):
        pull_request = FakePullRequest([])
        self.run_feedback(pull_request, extra=["--base-status", "unavailable"])
        self.assertEqual(([], []), (self.opened, pull_request.writes))
        self.assertEqual("the base commit could not be analyzed.", self.evidence["skipped"])

    def test_failed_head_analysis_posts_nothing(self):
        pull_request = FakePullRequest([])
        self.run_feedback(pull_request, extra=["--head-exit-code", "2"])
        self.assertEqual(([], []), (self.opened, pull_request.writes))
        self.assertEqual("head analysis failed.", self.evidence["skipped"])

    def test_fork_pull_requests_are_skipped_with_a_warning(self):
        event = json.loads(Path(self.environment["GITHUB_EVENT_PATH"]).read_text())
        event["pull_request"]["head"]["repo"]["full_name"] = "someone/fork"
        Path(self.environment["GITHUB_EVENT_PATH"]).write_text(json.dumps(event))
        pull_request = FakePullRequest([])
        output = self.run_feedback(pull_request)
        self.assertEqual(([], []), (self.opened, pull_request.writes))
        self.assertIn("::warning::Columbo feedback skipped: pull requests from forks", output)

    def test_missing_write_permission_warns_and_succeeds(self):
        pull_request = FakePullRequest([], forbid_writes=True)
        output = self.run_feedback(pull_request)
        self.assertIn("::warning::Columbo feedback needs the pull-requests: write permission", output)
        self.assertEqual("the token cannot write pull request comments.", self.evidence["skipped"])


class RecordingClient:
    def __init__(self):
        self.sent = []

    def rest(self, method, path, payload=None):
        self.sent.append((method, path, payload))
        return {"id": 1}

    def graphql(self, query, variables, write=False):
        self.sent.append((re.search(r"\{\s*(\w+)\(", query).group(1) if write else "query", variables, write))
        return {"repository": {"pullRequest": {"reviewThreads": {
            "pageInfo": {"hasNextPage": False, "endCursor": None}, "nodes": []}}}}


class PullRequestRequestTests(unittest.TestCase):
    def test_requests_match_the_github_api(self):
        client = RecordingClient()
        pull_request = feedback.PullRequest(client, "example/repo", 7, HEAD_SHA)
        pull_request.create_review_comment("line", "a.go", 12)
        pull_request.create_review_comment("file", "a.go")
        pull_request.create_issue_comment("conversation")
        pull_request.edit_comment(feedback.Kind.REVIEW, 5, "edited")
        pull_request.edit_comment(feedback.Kind.ISSUE, 6, "edited")
        pull_request.reply(5, "reply")
        pull_request.set_thread_resolved("T1", True)
        pull_request.set_thread_resolved("T1", False)
        self.assertEqual([
            ("POST", "/repos/example/repo/pulls/7/comments",
             {"body": "line", "commit_id": HEAD_SHA, "path": "a.go", "line": 12, "side": "RIGHT"}),
            ("POST", "/repos/example/repo/pulls/7/comments",
             {"body": "file", "commit_id": HEAD_SHA, "path": "a.go", "subject_type": "file"}),
            ("POST", "/repos/example/repo/issues/7/comments", {"body": "conversation"}),
            ("PATCH", "/repos/example/repo/pulls/comments/5", {"body": "edited"}),
            ("PATCH", "/repos/example/repo/issues/comments/6", {"body": "edited"}),
            ("POST", "/repos/example/repo/pulls/7/comments/5/replies", {"body": "reply"}),
            ("resolveReviewThread", {"id": "T1"}, True),
            ("unresolveReviewThread", {"id": "T1"}, True),
        ], client.sent)

    def test_review_threads_are_read_from_graphql(self):
        client = RecordingClient()
        self.assertEqual([], feedback.PullRequest(client, "example/repo", 7, HEAD_SHA).review_threads())
        self.assertEqual(("query", {"owner": "example", "name": "repo", "number": 7, "cursor": None}, False), client.sent[0])


class Response:
    def __init__(self, body, link=""):
        self.body, self.headers = json.dumps(body).encode(), {"Link": link}

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def read(self):
        return self.body


class GitHubClientTests(unittest.TestCase):
    def client(self):
        self.now, self.slept = 0.0, []

        def sleep(seconds):
            self.slept.append(seconds)
            self.now += seconds
        return feedback.GitHubClient("secret-token", sleep=sleep, clock=lambda: self.now)

    def test_writes_are_spaced_a_second_apart_and_reads_are_not(self):
        client = self.client()
        with patch.object(feedback, "urlopen", side_effect=lambda request, timeout: Response({"data": {}})):
            client.rest("POST", "/one", {})
            client.rest("GET", "/read")
            self.now += 0.25
            client.rest("PATCH", "/two", {})
            client.graphql("query", {})
            client.graphql("mutation", {}, write=True)
        self.assertEqual([0.75, 1.0], self.slept)

    def test_pages_follow_next_links(self):
        client = self.client()
        pages = [Response([1, 2], '<https://api.github.com/next?page=2>; rel="next", <https://x/last>; rel="last"'),
                 Response([3])]
        requested = []

        def respond(request, timeout):
            requested.append(request.full_url)
            return pages.pop(0)
        with patch.object(feedback, "urlopen", side_effect=respond):
            self.assertEqual([1, 2, 3], client.pages("/repos/example/repo/pulls/7/files"))
        self.assertEqual(["https://api.github.com/repos/example/repo/pulls/7/files?per_page=100",
                          "https://api.github.com/next?page=2"], requested)

    def test_forbidden_is_a_permission_error_without_the_token(self):
        client = self.client()
        error = HTTPError("https://api.github.com/x", 403, "forbidden", {}, None)
        with patch.object(feedback, "urlopen", side_effect=error):
            with self.assertRaises(feedback.PermissionDenied) as caught:
                client.rest("POST", "/x", {})
        self.assertNotIn("secret-token", str(caught.exception))

    def test_a_rate_limited_403_is_not_a_permission_error(self):
        client = self.client()
        error = HTTPError("https://api.github.com/x", 403, "forbidden", {"Retry-After": "60"}, None)
        with patch.object(feedback, "urlopen", side_effect=error):
            with self.assertRaises(feedback.GitHubError) as caught:
                client.rest("POST", "/x", {})
        self.assertNotIsInstance(caught.exception, feedback.PermissionDenied)

    def test_forbidden_graphql_errors_are_permission_errors(self):
        client = self.client()
        with patch.object(feedback, "urlopen", side_effect=lambda request, timeout: Response(
                {"errors": [{"type": "FORBIDDEN", "message": "Resource not accessible by integration"}]})):
            with self.assertRaises(feedback.PermissionDenied):
                client.graphql("mutation", {}, write=True)


if __name__ == "__main__":
    unittest.main()
