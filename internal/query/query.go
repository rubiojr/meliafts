// Package query implements a Gmail-style search query language for the melia
// mail database and compiles it into SQLite FTS5 queries.
//
// A query string is a whitespace-separated list of terms that are combined with
// an implicit AND. Each term is one of:
//
//	field:value         a full-text field filter (subject, sender, recipient,
//	                    snippet, body, and the aliases from/to)
//	field:"a phrase"    the value may be a double-quoted phrase
//	flag:               a boolean filter (unread, flagged, attachments);
//	                    present means true, e.g. `unread:` or `flagged:`
//	flag:false          a boolean filter may take an explicit value
//	-term               any term may be negated with a leading '-'
//	bare words          free text searched across every full-text column
//
// Parsing produces a small AST (see Node). Compiling walks the AST and produces
// a single FTS5 MATCH expression for the full-text portion plus a set of
// boolean WHERE conditions for the flag portion, joined against the messages
// table. See Query.Compile.
package query

// Query is a parsed search query. The Root is always an *And whose children are
// the top-level conditions.
type Query struct {
	Root *And
}

// Parse parses a Gmail-style query string into a Query. An empty (or
// whitespace-only) input yields a valid Query that matches every message.
func Parse(input string) (*Query, error) {
	toks, err := tokenize(input)
	if err != nil {
		return nil, err
	}

	root := &And{}
	for _, tk := range toks {
		node, err := nodeFromToken(tk)
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue
		}
		root.Children = append(root.Children, node)
	}

	return &Query{Root: root}, nil
}

// IsEmpty reports whether the query has no conditions and therefore matches
// every message.
func (q *Query) IsEmpty() bool {
	return q == nil || q.Root == nil || len(q.Root.Children) == 0
}
