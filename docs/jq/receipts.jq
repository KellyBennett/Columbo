# --arg id C-... --arg kind metric-contribution; preserve complete receipt records.
(first(.cases[] | select(.id == $id)) // error("Unknown case id: \($id)"))
| {
    id, smell, verdict, suppressed, symbol, clues, clusters, policy_reviews,
    receipts: [.receipts[] | select(.kind == $kind)]
  }
