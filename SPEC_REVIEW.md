# Specification readiness review

This review covers the original 41 implementation questions against the revised SPEC.md. It is specification review, not implementation validation. No Columbo code was designed or implemented.

| Original questions | Resolution location | Decision |
|---|---|---|
| 1–2 | Configuration | Fieldwise defaults, replacing exclusions, strict YAML typing/validation, inclusive overlap range. |
| 3–5 | CLI and exit codes; Configuration | Single module, physical source paths, excluded files omitted as subjects but retained for typing/reference evidence. |
| 6–7 | CLI and exit codes; Metric definitions | Named declarations only, signature-only bodyless analysis, token-based physical line counting. |
| 8–10 | Metric definitions | Canonical package/type dependency units, explicit double counting, recursive type handling, precise private-type exemption. |
| 11–13 | Feature Envy | Lexical variable/field-path identities, one selector increment, one method case with every qualifying group. |
| 14–16 | Data Clump; Metric definitions | Declaration occurrences, alias/type normalization, closed frequent multiset support. |
| 17–19 | Cosmetic Extraction; CLI and exit codes | Named helper scope, static call/reference universe, structural known-interface exclusion. |
| 20–21 | Cosmetic Extraction | Immediate-caller variable sets and unchanged forwarding, deduplicated values, no transitive parameter substitution. |
| 22–25 | Cosmetic Extraction | Statement-list boundaries, exact exception symbols, maximal sequences only, all qualifying clusters in one parent case. |
| 26–29 | Virtual inlining; Cosmetic Extraction | All reachable candidate sites, severity-independent thresholds, token buckets, pinned visitor interception, retained cycle-cutoff calls. |
| 30–31 | Git history | Bounded changed-file provenance, first-parent comparison, committer timestamp/hash order, no verdict effects. |
| 32–33 | Case identity and ordering; Data Clump | Earliest supporting declaration anchor, qualified symbol/type encodings, escaped canonical identity. |
| 34–36 | Suppressions | Go doc-group attachment, invalid/duplicate/Data Clump handling, retained suppressed cases and explicit counting. |
| 37–39 | Output | Typed records and ordering, seven literal guidance templates, mandatory per-smell clues and contributing receipts. |
| 40–41 | Output; CLI and exit codes; Git history | Defined warning channels, one buffered output, fatal errors leave stdout empty and exit 2. |

The acceptance section supplies 31 named scenarios plus seven deterministic guidance templates. It explicitly distinguishes exclusive metric limits from inclusive structural minima.

## Dogfooding status

CE-001 remains provisional by design. Its enforcement is fully specified; only the desirability of the strict policy is being evaluated. It does not block implementation, but public v1 release must resolve it under the first-class policy register.

## Readiness result

READY — The specification is sufficiently complete to hand to an implementation agent. There are zero material implementation questions.
