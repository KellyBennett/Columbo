# Evidence snapshot

Every analysis writes a fresh SQLite snapshot and prints `Database: <path>` as its last line. Stdout and the exit code are rendered from that snapshot after it is published. The checked-in queries in `docs/sqlite/` are the documented way for an agent to read the evidence. Columbo refuses a destination that already exists, one inside the analyzed module, and stdout (`-`).

## Sub-features

- `snap-default-path` writes `columbo-<26 base32>.sqlite` in the OS temp dir when `--output` is omitted.
- `snap-queries` runs every `docs/sqlite/*.sql` read-only against a real snapshot.
- `snap-no-clobber` exits 2 when the destination exists and leaves it unchanged.
- `snap-outside-module` exits 2 with `inspection output must be outside source root` and writes nothing into the module.
- `snap-no-stdout` exits 2 for `--output -`.

## How to get to it (user POV)

- Read the `Database:` line printed by any run, then open that file with `sqlite3`.
- Pass `--output PATH` to choose the location.

## Driving it with columbo-verify

Preconditions:

- Baseline from the [README](./README.md). Run `$V stage "$RUN" all` and `$V drive "$RUN" all snap --staged=false --no-history`.
- `C="$RUN/bin/columbo"` and `G="$(cat "$RUN/go-bin")"`.

- **Path printed.** The last line of `stdout.txt` is `Database: $RUN/evidence/snap/report.sqlite`, and that file exists.
- **Queries.** Run `./scripts/test-sqlite-queries.sh "$RUN/evidence/snap/report.sqlite"` from the repo root. Exit is `0`. For individual queries, use `$V sql "$RUN" snap <name>.sql`.
- **No clobber.** Run `shasum "$RUN/evidence/snap/report.sqlite"`, then `(cd "$RUN/subjects/all" && PATH="$G:$PATH" "$C" --output "$RUN/evidence/snap/report.sqlite" ./...)`. Exit is `2`, stderr starts with `columbo: snapshot destination already exists`, and the checksum is unchanged.
- **Inside module.** Run `(cd "$RUN/subjects/all" && PATH="$G:$PATH" "$C" --output "$RUN/subjects/all/inside.sqlite" ./...)`. Exit is `2`, and `ls "$RUN/subjects/all"` shows no `inside.sqlite`.
- **Stdout refused.** Run `$C --output -`. Exit is `2`, and stderr matches `columbo:.*stdout`.

## Gotchas

- `.gitignore` ignores `*.sqlite` anywhere. A snapshot written inside the repo would therefore be invisible to `git status`, not just "safe".
- A run that omits `--output` leaves a file in `$TMPDIR` that cleanup does not know about. Delete it yourself, using the path from its `Database:` line.
- `test-sqlite-queries.sh` needs `sqlite3` on `PATH`. The macOS system copy works.
