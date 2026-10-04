# Keep the report's counts and warnings; index only unsuppressed FAIL cases.
{
  version,
  summary,
  warnings,
  cases: [
    .cases[]
    | select(.verdict == "FAIL" and (.suppressed | not))
    | {id, smell, verdict, symbol, file, start_line, end_line, clues, policy_reviews}
  ]
}
