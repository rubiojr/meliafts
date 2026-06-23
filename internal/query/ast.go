package query

import "time"

// Node is a single node in the parsed query AST.
//
// The tree produced by Parse is intentionally small: the root is always an
// *And whose children are leaf conditions (*Match or *Flag), some of which may
// be wrapped in *Not. The shape is kept flat so the compiler can split it into
// a single FTS5 MATCH expression plus a set of boolean WHERE conditions, which
// is the only structure SQLite FTS5 can express efficiently (MATCH must be a
// single top-level constraint against the virtual table).
//
// The interface is deliberately extensible: additional node kinds (e.g. *Or)
// can be added later without breaking callers that type-switch on Node.
type Node interface {
	isNode()
}

// And is a conjunction of child nodes. The top-level query is always an *And
// (possibly empty, meaning "match everything").
type And struct {
	Children []Node
}

func (*And) isNode() {}

// Not negates its child. The parser only ever wraps a single leaf node.
type Not struct {
	Child Node
}

func (*Not) isNode() {}

// Match is a full-text condition compiled against the FTS5 table.
//
// Columns lists the FTS5 columns the search is restricted to. An empty Columns
// slice means "search every column" (a bare term, e.g. `invoice`). Phrase is
// the raw, un-escaped search text; the compiler is responsible for quoting it
// into a safe FTS5 phrase. When Prefix is true the final token is treated as a
// prefix token (e.g. `inv*`).
type Match struct {
	Columns []string
	Phrase  string
	Prefix  bool
}

func (*Match) isNode() {}

// Flag is a boolean condition compiled against a column of the messages table,
// for example `is_read = 0`. Column is the messages column name and Value is
// the integer (0 or 1) the column is compared against.
type Flag struct {
	Column string
	Value  int
}

func (*Flag) isNode() {}

// Date is a condition on the messages.date column, for example "newer than one
// month" or "before 2024-01-01". Op is the SQL comparison operator (">=" or
// "<"). Exactly one of At or Rel describes the cut-off:
//
//   - At holds an absolute instant (e.g. parsed from after:2024-01-01).
//   - Rel holds a relative amount (e.g. 7d, 1month) that the compiler subtracts
//     from a reference time (Options.Now) to obtain the cut-off.
type Date struct {
	Op  string
	At  time.Time
	Rel *RelSpec
}

func (*Date) isNode() {}

// RelSpec is a relative time amount such as "7d", "1week" or "1month". It is
// subtracted from a reference time to produce an absolute cut-off. Calendar
// units (months, years) use calendar arithmetic so "1month" before 31 Mar is
// 28/29 Feb rather than a fixed number of days.
type RelSpec struct {
	Years  int
	Months int
	Days   int
	Dur    time.Duration
}

// Before returns now shifted back by the relative amount.
func (r RelSpec) Before(now time.Time) time.Time {
	return now.AddDate(-r.Years, -r.Months, -r.Days).Add(-r.Dur)
}
