# Prose comments

`prose-comment` fails by default when an analyzed production Go file contains any
non-directive comment. Existing comments are included; there is no Git baseline,
new-code filter, or positive allowance. This is an explicit source policy, not a
claim that every comment proves poor architecture.

The rule covers package documentation, declaration documentation, standalone and
inline comments, block comments, TODOs, empty comments, and commented-out code.
It uses parsed comment tokens, so strings containing URLs or comment delimiters
are not comments. Each line-comment token counts once; each block-comment token
counts once. One case per file contains a zero-limit count and a receipt for every
prohibited token. Locations use physical file coordinates, including with `//line`
and CRLF source. The case is anchored to the file's actual package clause, whose
explicit symbol is `<package import path>.<package-clause>`; it does not pretend
that file-level prose belongs to a function. Identity uses smell, file, package
clause symbol, and an empty key, so edits to comment text or line numbers do not
change the case ID.

The normal analysis scope applies: selected, build-active production source in the
invocation module. Test files, generated files, vendor, configured exclusions, and
inactive build variants remain outside that scope. Markdown documentation and
non-Go files are outside this Go rule.

## Machine input

The recognized spellings are defined in `internal/columbo/comment_directives.go`:

- Go build constraints (`//go:build`, legacy `// +build`), generation, embedding,
  linking, WebAssembly import/export, `//go:debug`, and the named compiler/cgo
  pragmas in the allowlist.
- Line directives (`//line` and `/*line ...*/`) and cgo `//export` directives.
- `//nolint` with optional linter names and its trailing justification syntax.
- Staticcheck `//lint:ignore` and `//lint:file-ignore`, including their reasons.
- Columbo `//columbo:ignore` directives, including their required justifications.
  Existing parsing, attachment, and justification checks still apply.
- The comment group attached to `import "C"`, which is executable cgo preamble
  input rather than ordinary Go documentation.

Recognition is syntactic. The consuming tool remains responsible for validating
arguments and placement. Unknown directive names are prohibited; there is no
blanket `//tool:anything` or `//go:anything` exemption. Additional tooling syntax
needs an explicit reviewed recognizer. A directive exempts only its token, never
adjacent prose (except the cgo preamble group, which cgo consumes as a unit).

## Guidance to agents

If a comment preserves an intentional omission or guards against a future “fix,”
write a regression test that fails when the unwanted behavior is introduced.
Comments are not a reliable enforcement mechanism. Test the actual behavior,
including negative behavior such as not sending an event or not retrying an
operation; do not substitute a test that searches for a particular comment.

Express ordinary intent through clear names, cohesive structure, and executable
contracts. Preserve useful design rationale and attribution in external
documentation. Remove obsolete explanations and commented-out code. Do not delete
an untested invariant without first protecting it, disguise prose as a directive,
mark handwritten files as generated, or move it into unused strings.

## Policy and storage

```yaml
severity:
  prose-comment: fail
```

The normal `warn` and `off` configuration values are supported. There is no count
threshold to relax. Inline suppression of this file-wide rule is rejected, like
inline suppression of `data-clump`; directives suppressing other rules remain
valid machine input.

SQLite schema version 4 adds `prose-comment` to the permitted smell values. The
existing declaration relation also carries physical package-clause anchors for
file-wide findings. Those anchors have no function dependency inventory. Clue
`prose-comments` has `value > 0`, with one explicitly linked `prose-comment`
receipt per token. The existing summary, check annotations, severity handling,
and exit codes apply. Earlier snapshots remain untouched and require the matching
older reader, as with previous schema changes.
