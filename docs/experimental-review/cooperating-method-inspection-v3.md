# Cooperating-method inspection candidate v3

Status: opt-in, experimental local candidate. The [v2 responsibility revision](cooperating-method-inspection-v2.md) and [v1 protocol](cooperating-method-inspection-v1.md) remain preserved. JSON/SQLite contracts and default analyzer behavior do not change.

The inspection entrypoint now captures decoded configuration and its reported SHA-256 from one read. An implicitly absent default file produces defaults and an empty hash. Explicit absence, read errors and decoding errors retain ordinary configuration semantics and fail before package loading. Existing root/path validation precedes that read.

This is snapshot semantics: a later mutation, creation or removal does not change the already captured configuration/hash. The removed later reread no longer introduces a later filesystem-read error. The external source manifest, unchanged-source receipt and binder still must reject later drift; no retry, stability guarantee or automatic invariant inference is added.

A controlled scratch-only load-boundary characterization is kept outside production sources. It is instrumented interleaving evidence, not a reproduced occurrence in an unmodified process. The production candidate has no callback, global test hook or replacement loader framework.
