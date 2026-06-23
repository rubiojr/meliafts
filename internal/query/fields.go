package query

// This file defines the mapping between Gmail-style field keywords used in a
// query string and the underlying melia schema (the messages_fts virtual table
// and the messages table). Keeping the mapping in one place makes it easy to
// add new operators or adapt to schema changes.

// ftsField maps a user-facing field keyword to one or more FTS5 columns of the
// messages_fts virtual table. A keyword that maps to multiple columns (such as
// "sender") is compiled into a `{col_a col_b} : "value"` column-filter.
type ftsField struct {
	columns []string
}

// flagField maps a user-facing boolean keyword to a column of the messages
// table and the column value that represents the flag being "present" (true).
//
// For example "unread" maps to is_read with presentValue 0, because a message
// is unread when is_read = 0.
type flagField struct {
	column       string
	presentValue int
}

// ftsFields are the full-text search operators. The canonical Gmail-style names
// requested are subject, sender, recipient, snippet and body; a few intuitive
// aliases are included as well.
var ftsFields = map[string]ftsField{
	"subject":   {columns: []string{"subject"}},
	"sender":    {columns: []string{"from_name", "from_address"}},
	"from":      {columns: []string{"from_name", "from_address"}},
	"recipient": {columns: []string{"to_text"}},
	"to":        {columns: []string{"to_text"}},
	"snippet":   {columns: []string{"snippet"}},
	"body":      {columns: []string{"body_text"}},
}

// flagFields are the boolean operators. They are true when present, e.g.
// `unread:` filters to unread messages. An explicit value may be supplied to
// override (`unread:false`) and the operator may be negated (`-unread:`).
var flagFields = map[string]flagField{
	"unread":      {column: "is_read", presentValue: 0},
	"read":        {column: "is_read", presentValue: 1},
	"flagged":     {column: "is_flagged", presentValue: 1},
	"starred":     {column: "is_flagged", presentValue: 1},
	"attachments": {column: "has_attachments", presentValue: 1},
	"attachment":  {column: "has_attachments", presentValue: 1},
}

// dateOps are the date operators. Each maps to the SQL comparison applied to
// messages.date. "Newer/after/since" are lower bounds (>=); "older/before/until"
// are upper bounds (<). The value is either a relative duration (7d, 1week,
// 1month) or an absolute date (2024-01-31, 2024/01/31[ HH:MM]).
var dateOps = map[string]string{
	"after":      ">=",
	"since":      ">=",
	"newer":      ">=",
	"newer_than": ">=",
	"before":     "<",
	"until":      "<",
	"older":      "<",
	"older_than": "<",
}

// flipDateOp returns the complementary comparison, used when a date operator is
// negated (e.g. -newer:7d means "not newer than 7 days" = older than 7 days).
func flipDateOp(op string) string {
	if op == ">=" {
		return "<"
	}
	return ">="
}
