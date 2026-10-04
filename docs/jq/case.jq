# --arg id C-...; keep the complete case and reject a stale or mistyped ID.
first(.cases[] | select(.id == $id))
// error("Unknown case id: \($id)")
