package query

import (
	"fmt"
	"strings"
	"time"
)

// Schema names. The melia full-text index is an external-content FTS5 table
// (messages_fts) whose rowid matches messages.rowid.
const (
	tableMessages = "messages"
	tableFTS      = "messages_fts"
	tableFolders  = "folders"
)

// DefaultColumns is the default SELECT list. Every column is qualified with the
// "m" alias given to the messages table so the same list works whether or not
// the FTS table is joined.
var DefaultColumns = []string{
	"m.id",
	"m.date",
	"m.is_read",
	"m.is_flagged",
	"m.has_attachments",
	"m.from_name",
	"m.from_address",
	"m.subject",
	"m.snippet",
}

// Options controls SQL generation.
type Options struct {
	// Columns is the SELECT list. When empty, DefaultColumns is used.
	Columns []string
	// Limit, when > 0, appends a LIMIT clause.
	Limit int
	// Offset, when > 0, appends an OFFSET clause.
	Offset int
	// Now is the reference time used to resolve relative date filters such as
	// newer:7d. When zero, time.Now() is used.
	Now time.Time
	// Dedup collapses messages that appear in more than one folder (e.g. Gmail/
	// Proton "All Mail" duplicates) to a single row per Message-ID.
	Dedup bool
	// HideSpam excludes every message whose Message-ID appears in a spam-type
	// folder, regardless of which folder a given row belongs to. This hides spam
	// even when a copy also lives in "All Mail".
	HideSpam bool
}

// Compiled is the result of compiling a Query into SQL.
type Compiled struct {
	// SQL is a parameterised statement ready to be passed to database/sql.
	SQL string
	// Args holds the bound arguments, in positional order.
	Args []any
	// FTSMatch is the positive FTS5 MATCH expression embedded in SQL (it may
	// include a trailing "NOT (...)" clause). It is empty when the query has no
	// positive full-text terms, in which case the messages table is queried
	// directly.
	FTSMatch string
}

// FTSMatch returns just the full-text portion of the query as an FTS5 MATCH
// expression, e.g. `subject : "invoice" AND {from_name from_address} : "bob"`.
// It is empty when the query has no positive full-text terms (a pure-negative
// or flag-only query). This is primarily useful for inspection and debugging.
func (q *Query) FTSMatch() (string, error) {
	p, err := q.collectParts(time.Now())
	if err != nil {
		return "", err
	}
	return p.matchExpr(), nil
}

// Compile walks the AST and produces a runnable SQL statement.
//
// The full-text terms are folded into a single FTS5 MATCH expression; the
// boolean flag terms become equality conditions on the messages table, ANDed at
// the top level. There are three shapes:
//
//   - Positive full-text terms present: join messages_fts to messages and
//     constrain with `messages_fts MATCH ?`, ordered by FTS rank.
//   - No positive full-text terms but negative ones present: query messages and
//     exclude rows via `rowid NOT IN (SELECT ... MATCH ?)`.
//   - Flag-only or empty query: query messages directly.
func (q *Query) Compile(opts Options) (*Compiled, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	p, err := q.collectParts(now)
	if err != nil {
		return nil, err
	}

	cols := opts.Columns
	if len(cols) == 0 {
		cols = DefaultColumns
	}
	selectList := strings.Join(cols, ", ")

	posExpr := strings.Join(p.pos, " AND ")
	negExpr := strings.Join(p.neg, " OR ")

	// A positive folder filter (e.g. in:spam) already scopes the result to a
	// single folder, where no cross-folder duplicates exist, so dedup is both
	// unnecessary and harmful (its global representative may live in another
	// folder). An explicit in:spam likewise overrides the default spam hiding.
	dedup := opts.Dedup && !p.folderRequested
	hideSpam := opts.HideSpam && !p.spamRequested
	extra := viewConditions(dedup, hideSpam)

	var b strings.Builder
	var args []any

	if posExpr != "" {
		// Full-text search path: join the FTS table to messages.
		matchExpr := posExpr
		if negExpr != "" {
			matchExpr = posExpr + " NOT (" + negExpr + ")"
		}
		fmt.Fprintf(&b, "SELECT %s FROM %s JOIN %s m ON m.rowid = %s.rowid WHERE %s MATCH ?",
			selectList, tableFTS, tableMessages, tableFTS, tableFTS)
		args = append(args, matchExpr)
		for i, c := range p.conds {
			b.WriteString(" AND ")
			b.WriteString(c)
			args = append(args, p.condArgs[i])
		}
		for _, ec := range extra {
			b.WriteString(" AND ")
			b.WriteString(ec)
		}
		// m.id is a stable tiebreaker so LIMIT/OFFSET pagination doesn't skip or
		// duplicate rows when ranks are equal.
		b.WriteString(" ORDER BY rank, m.id")
		applyLimit(&b, &args, opts)
		return &Compiled{SQL: b.String(), Args: args, FTSMatch: matchExpr}, nil
	}

	// No positive full-text terms: query the messages table directly.
	fmt.Fprintf(&b, "SELECT %s FROM %s m", selectList, tableMessages)

	conds := append([]string(nil), p.conds...)
	condArgs := append([]any(nil), p.condArgs...)
	if negExpr != "" {
		conds = append(conds, fmt.Sprintf("m.rowid NOT IN (SELECT rowid FROM %s WHERE %s MATCH ?)", tableFTS, tableFTS))
		condArgs = append(condArgs, negExpr)
	}
	conds = append(conds, extra...)
	if len(conds) > 0 {
		b.WriteString(" WHERE ")
		b.WriteString(strings.Join(conds, " AND "))
		args = append(args, condArgs...)
	}
	b.WriteString(" ORDER BY m.date DESC, m.id DESC")
	applyLimit(&b, &args, opts)
	return &Compiled{SQL: b.String(), Args: args, FTSMatch: ""}, nil
}

// viewConditions returns extra WHERE predicates for the "view" options (spam
// hiding and deduplication). They are self-contained SQL with no bound
// parameters, so they slot into either query shape without disturbing the order
// of the positional arguments. Rows with no Message-ID are always kept (they
// cannot be matched to a duplicate or a spam copy by identity).
func viewConditions(dedup, hideSpam bool) []string {
	var ex []string
	if hideSpam {
		ex = append(ex, fmt.Sprintf(
			"(m.message_id IS NULL OR m.message_id = '' OR m.message_id NOT IN "+
				"(SELECT message_id FROM %s WHERE message_id IS NOT NULL AND message_id <> '' "+
				"AND folder_id IN (SELECT id FROM %s WHERE type = 'spam')))",
			tableMessages, tableFolders))
	}
	if dedup {
		ex = append(ex, fmt.Sprintf(
			"(m.message_id IS NULL OR m.message_id = '' OR m.rowid IN "+
				"(SELECT MIN(rowid) FROM %s WHERE message_id IS NOT NULL AND message_id <> '' GROUP BY message_id))",
			tableMessages))
	}
	return ex
}

// parts holds the pieces collected from the AST before assembling SQL.
type parts struct {
	pos             []string  // positive FTS sub-expressions
	neg             []string  // negated FTS sub-expressions
	conds           []string  // non-FTS WHERE conditions, e.g. "m.is_read = ?"
	condArgs        []any     // bound args matching conds
	now             time.Time // reference time for relative date filters
	folderRequested bool      // a positive in:<folder> term is present
	spamRequested   bool      // a positive in:spam term is present
}

// matchExpr combines the positive and negative FTS parts into a single MATCH
// expression. It returns "" when there is no positive part (a pure-negative or
// flag-only query), since FTS5 cannot express a stand-alone NOT.
func (p *parts) matchExpr() string {
	posExpr := strings.Join(p.pos, " AND ")
	if posExpr == "" {
		return ""
	}
	if negExpr := strings.Join(p.neg, " OR "); negExpr != "" {
		return posExpr + " NOT (" + negExpr + ")"
	}
	return posExpr
}

func (q *Query) collectParts(now time.Time) (*parts, error) {
	p := &parts{now: now}
	if q == nil || q.Root == nil {
		return p, nil
	}
	for _, child := range q.Root.Children {
		if err := collect(child, p); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// collect walks a node and accumulates its contribution into p.
func collect(n Node, p *parts) error {
	switch v := n.(type) {
	case *Match:
		p.pos = append(p.pos, ftsPart(v))
	case *Flag:
		p.addFlag(v)
	case *Date:
		p.addDate(v.Op, v.At, v.Rel)
	case *Folder:
		p.addFolder(v.Type, false)
	case *Not:
		return collectNegated(v.Child, p)
	case *And:
		for _, c := range v.Children {
			if err := collect(c, p); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported node %T", n)
	}
	return nil
}

// collectNegated accumulates the contribution of a negated leaf node. The parser
// folds flag/date negation into the leaf itself, so the *Flag and *Date cases
// here are defensive and simply invert the leaf.
func collectNegated(child Node, p *parts) error {
	switch inner := child.(type) {
	case *Match:
		p.neg = append(p.neg, ftsPart(inner))
	case *Flag:
		p.conds = append(p.conds, fmt.Sprintf("m.%s = ?", inner.Column))
		p.condArgs = append(p.condArgs, 1-inner.Value)
	case *Date:
		p.addDate(flipDateOp(inner.Op), inner.At, inner.Rel)
	case *Folder:
		p.addFolder(inner.Type, true)
	default:
		return fmt.Errorf("unsupported negated node %T", child)
	}
	return nil
}

// addFlag appends a boolean column condition.
func (p *parts) addFlag(f *Flag) {
	p.conds = append(p.conds, fmt.Sprintf("m.%s = ?", f.Column))
	p.condArgs = append(p.condArgs, f.Value)
}

// addFolder appends a folder-type condition via a subquery on the folders table.
func (p *parts) addFolder(folderType string, negate bool) {
	op := "IN"
	if negate {
		op = "NOT IN"
	}
	if !negate {
		p.folderRequested = true
		if folderType == "spam" {
			p.spamRequested = true
		}
	}
	p.conds = append(p.conds, fmt.Sprintf("m.folder_id %s (SELECT id FROM %s WHERE type = ?)", op, tableFolders))
	p.condArgs = append(p.condArgs, folderType)
}

// addDate appends a date condition, resolving a relative spec against p.now.
// Both sides are normalised with julianday() so the comparison works regardless
// of the textual datetime format stored in messages.date.
func (p *parts) addDate(op string, at time.Time, rel *RelSpec) {
	cutoff := at
	if rel != nil {
		cutoff = rel.Before(p.now)
	}
	p.conds = append(p.conds, fmt.Sprintf("julianday(m.date) %s julianday(?)", op))
	p.condArgs = append(p.condArgs, cutoff.UTC().Format("2006-01-02 15:04:05"))
}

// ftsPart renders a *Match as an FTS5 sub-expression, applying the column
// filter syntax when the match is restricted to one or more columns.
func ftsPart(m *Match) string {
	term := ftsTerm(m.Phrase, m.Prefix)
	switch len(m.Columns) {
	case 0:
		return term
	case 1:
		return m.Columns[0] + " : " + term
	default:
		return "{" + strings.Join(m.Columns, " ") + "} : " + term
	}
}

// ftsTerm quotes a phrase as an FTS5 string literal, escaping embedded double
// quotes by doubling them. A prefix match appends the prefix token marker.
func ftsTerm(phrase string, prefix bool) string {
	q := `"` + strings.ReplaceAll(phrase, `"`, `""`) + `"`
	if prefix {
		q += " *"
	}
	return q
}

// applyLimit appends LIMIT/OFFSET clauses and their bound arguments.
func applyLimit(b *strings.Builder, args *[]any, opts Options) {
	switch {
	case opts.Limit > 0:
		b.WriteString(" LIMIT ?")
		*args = append(*args, opts.Limit)
		if opts.Offset > 0 {
			b.WriteString(" OFFSET ?")
			*args = append(*args, opts.Offset)
		}
	case opts.Offset > 0:
		b.WriteString(" LIMIT -1 OFFSET ?")
		*args = append(*args, opts.Offset)
	}
}
