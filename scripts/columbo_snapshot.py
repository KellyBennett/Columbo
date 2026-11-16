"""Read-only access to a Columbo SQLite snapshot, shared by the GitHub publishers.

Standard library only. The publishers are adapters over the stored evidence;
this module is the one place that knows how to open, validate and render it.
"""

import contextlib
from pathlib import Path
import sqlite3


def limited(text, maximum=65536):
    encoded = text.encode("utf-8")
    if len(encoded) <= maximum:
        return text
    suffix = "\n[Truncated; full evidence is in the SQLite artifact.]"
    return encoded[:maximum - len(suffix.encode())].decode("utf-8", errors="ignore") + suffix


def number_text(kind, value):
    if kind in ("parameter-overlap", "dependency-overlap", "foreign-own-ratio"):
        return f"{value:.6f}"
    return str(value)


def metric_text(row):
    text = f'{row["kind"]} {row["subject"]}: {number_text(row["kind"], row["numeric_value"])}'
    if row["limit_value"] is not None:
        text += f' (limit {row["operator"]} {number_text(row["kind"], row["limit_value"])})'
    return text


@contextlib.contextmanager
def open_snapshot(path, reader=None):
    uri = Path(path).resolve(strict=True).as_uri() + "?mode=ro"
    with contextlib.closing(sqlite3.connect(uri, uri=True)) as db:
        db.execute("PRAGMA query_only=ON")
        snapshot = (reader or Snapshot)(db)
        snapshot.validate()
        yield snapshot


class Snapshot:
    def __init__(self, connection):
        self.db = connection
        self.db.row_factory = sqlite3.Row

    def validate(self):
        if self.db.execute("PRAGMA application_id").fetchone()[0] != 0x434C4D42:
            raise ValueError("not a Columbo snapshot")
        if self.db.execute("PRAGMA user_version").fetchone()[0] != 9:
            raise ValueError("unsupported Columbo snapshot schema")
        reports = self.db.execute("SELECT id,schema_version FROM report").fetchall()
        if [tuple(row) for row in reports] != [(1, 9)]:
            raise ValueError("invalid Columbo report metadata")
        if [tuple(row) for row in self.db.execute("PRAGMA integrity_check")] != [("ok",)]:
            raise ValueError("snapshot integrity check failed")
        if self.db.execute("PRAGMA foreign_key_check").fetchall():
            raise ValueError("snapshot foreign key check failed")

    def metrics(self, case_id):
        return [metric_text(metric) for metric in self.db.execute("""
            SELECT kind,subject,numeric_value,limit_value,operator FROM clues
            WHERE case_id=? AND numeric_value IS NOT NULL ORDER BY ordinal
        """, (case_id,))]

    def guidance(self, case_id):
        return [f"{kind}: {item}" for kind, item in self.guidance_items(case_id)]

    def guidance_items(self, case_id):
        return [(row["kind"], row["item"]) for row in self.db.execute(
            "SELECT kind,item FROM case_guidance WHERE case_id=? ORDER BY kind,ordinal", (case_id,))]

    def supporting_evidence(self, case_id):
        return [*self.dependency_uses(case_id), *self.duplicate_fragments(case_id),
                *self.variant_evidence(case_id), *self.selection_evidence(case_id),
                *self.policy(case_id)]

    def policy(self, case_id):
        lines = []
        for item in self.db.execute("""
            SELECT p.* FROM case_policy_reviews cp JOIN policy_reviews p ON p.id=cp.policy_id
            WHERE cp.case_id=? ORDER BY cp.ordinal
        """, (case_id,)):
            lines.extend([f'Policy {item["id"]}: {item["note"]}', f'Review: {item["review_prompt"]}'])
        return lines

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
        for clue in self.db.execute("SELECT id,kind,subject FROM clues WHERE case_id=? AND kind IN ('variant-set','repeated-variant-set','variant-shared-write','variant-write-read','variant-state-overlap') ORDER BY ordinal", (case_id,)):
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
