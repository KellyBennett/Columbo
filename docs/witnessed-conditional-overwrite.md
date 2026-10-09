# Witnessed conditional overwrite (advisory)

`witnessed-conditional-overwrite` records a concrete local execution in which
separate sequential `if` statements inspect stable inputs and write different
complete scalar constants to the same resolved local variable. The earlier
assignment must still be that destination's last writer when the later assignment
executes. Both assignments, both guards, typed witness inputs and the evaluated
prefix appear in the ordinary summary and normalized SQLite advisory tables.
The prefix includes initialization, true/false conditions, and executed writes.

This is review evidence for Untangle Behavior, **not a defect diagnosis**. A
legitimate priority/override policy can produce exactly the same evidence. The
control tests intentionally report that policy; names do not exempt or select it.
No stage membership, severity, suppression, threshold, verdict or exit-code rules
change. Staged and ordinary runs retain the same independent advisory evidence.

## Bounded contract

- At most two resolved signed-integer parameters or direct receiver fields.
  Inputs are searched independently over integers -2 through 8 (at most 121
  assignments). Named signed integer types retain their resolved Go type.
- At most 32 candidate direct assignments in top-level `if` bodies, and 256
  statements per function. Multiple statements in each body are permitted.
- Supported guard evaluation uses typed integer comparisons, conjunction,
  subtraction, parentheses and scalar constants. Integer subtraction uses exact
  arithmetic and rejects any witness whose intermediate result cannot fit its
  Go type; `int` is conservatively restricted to signed 32-bit range even on a
  64-bit target. No wraparound witness is asserted.
- Candidate destinations are resolved local variables; result expressions must
  have compile-time integer, string or boolean values. Self-dependent updates,
  compound assignment and accumulation do not qualify. Equal resulting values
  do not qualify, including differently named equal-valued constants.
- The actual prefix through the later decision must evaluate successfully.
  Unsupported executed assignments, expressions, or returns reject that witness.
  An earlier return is never crossed. Local variable reads are not accepted as
  guard inputs. Unexecuted ordinary-score branches can remain unevaluated.
- Calls, closures, address escape, input mutation, indirect/promoted destinations,
  loops, `else`/initializers, branching and other unsupported control flow reject
  the function. Noninput direct receiver fields can be assigned constants on a
  supported path; they cannot supply guard inputs through aliases.

A witness assumes a nonnil receiver and the displayed values at function entry.
It does not establish reachability through constructors or public methods, final
returned output, correctness of precedence, or safe refactoring. It assumes an
ordinary sequential execution without concurrent external mutation. No witness
means **unknown**, never disjoint or clean. Coverage rows count analyzed
production declarations, not proofs of support or exhaustive analyses; individual
unsupported/skipped paths are not enumerated as separate findings.

## Validation

The untouched TennisGame2 source yields witnesses 5–3 and 3–5, showing advantage
then win assignments. TennisGame3 yields no witness under this contract; absence
is not a general quality judgment. Regression tests cover intentional override,
exclusive/equal-result guards, accumulation, intervening writers, multiple body
statements, early returns, calls, mutation, aliases, shadowing, signed types,
overflow, unsupported unsigned inputs, and out-of-domain overlaps. SQLite tests
verify source excerpts, witness values, ordinary zero verdicts and unchanged
staged issue counts.
