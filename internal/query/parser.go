package query

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// nodeFromToken converts a single lexer token into an AST node. It returns a
// nil node (and nil error) for tokens that carry no searchable content, such as
// a stray empty value, so callers can simply skip them.
func nodeFromToken(tk token) (Node, error) {
	if tk.hasField {
		return fieldNode(tk)
	}

	// Bare text term: full-text search across all FTS columns.
	value := strings.TrimSpace(tk.value)
	if value == "" {
		return nil, nil
	}
	return textNode(nil, value, tk.quoted, tk.negated), nil
}

// fieldNode builds a node for a `field:value` token, dispatching to the matching
// operator family (full-text, boolean flag, date, folder) or reporting an
// unknown field.
func fieldNode(tk token) (Node, error) {
	if f, ok := ftsFields[tk.field]; ok {
		return ftsFieldNode(tk, f)
	}
	if ff, ok := flagFields[tk.field]; ok {
		return flagFieldNode(tk, ff)
	}
	if op, ok := dateOps[tk.field]; ok {
		return dateFieldNode(tk, op)
	}
	if folderOps[tk.field] {
		return folderNode(tk)
	}
	return nil, fmt.Errorf("unknown field: %q", tk.field)
}

func ftsFieldNode(tk token, f ftsField) (Node, error) {
	value := strings.TrimSpace(tk.value)
	if value == "" {
		return nil, fmt.Errorf("field %q requires a value", tk.field)
	}
	return textNode(f.columns, value, tk.quoted, tk.negated), nil
}

func flagFieldNode(tk token, ff flagField) (Node, error) {
	want, err := parseFlagValue(tk.value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tk.field, err)
	}
	if tk.negated {
		want = !want
	}
	value := ff.presentValue
	if !want {
		// Columns are 0/1, so the "absent" value is the complement.
		value = 1 - ff.presentValue
	}
	return &Flag{Column: ff.column, Value: value}, nil
}

func dateFieldNode(tk token, op string) (Node, error) {
	at, rel, err := parseWhen(tk.value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tk.field, err)
	}
	if tk.negated {
		op = flipDateOp(op)
	}
	return &Date{Op: op, At: at, Rel: rel}, nil
}

// folderNode builds a *Folder (optionally wrapped in *Not) for an in:/folder:
// token, validating the folder type.
func folderNode(tk token) (Node, error) {
	typ := strings.ToLower(strings.TrimSpace(tk.value))
	if typ == "" {
		return nil, fmt.Errorf("%s requires a folder (%s)", tk.field, strings.Join(folderTypeList, ", "))
	}
	if !isFolderType(typ) {
		return nil, fmt.Errorf("unknown folder %q (want %s)", typ, strings.Join(folderTypeList, ", "))
	}
	var n Node = &Folder{Type: typ}
	if tk.negated {
		n = &Not{Child: n}
	}
	return n, nil
}

// textNode builds a *Match (optionally wrapped in *Not) for the given columns
// and value, extracting a trailing prefix marker ("*") from unquoted values.
func textNode(columns []string, value string, quoted, negated bool) Node {
	prefix := false
	if !quoted && strings.HasSuffix(value, "*") && len(value) > 1 {
		prefix = true
		value = strings.TrimSuffix(value, "*")
	}

	var n Node = &Match{Columns: columns, Phrase: value, Prefix: prefix}
	if negated {
		n = &Not{Child: n}
	}
	return n
}

// parseFlagValue interprets the value of a boolean flag operator. An empty
// value means the flag is present (true); otherwise common truthy/falsy spellings
// are accepted.
func parseFlagValue(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", s)
	}
}

// parseWhen interprets the value of a date operator as either a relative
// duration (7d, 1week, 1month) or an absolute date. For a relative value the
// returned rel is non-nil and at is zero; for an absolute value at is set and
// rel is nil.
func parseWhen(s string) (at time.Time, rel *RelSpec, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil, fmt.Errorf("requires a date or duration value")
	}
	if r, ok := parseRelSpec(s); ok {
		return time.Time{}, r, nil
	}
	if t, ok := parseAbsDate(s); ok {
		return t, nil, nil
	}
	return time.Time{}, nil, fmt.Errorf("invalid date or duration %q", s)
}

// relUnits maps every accepted duration unit spelling to a constructor that
// scales the parsed amount into a RelSpec.
var relUnits = map[string]func(int) RelSpec{
	"h": relHours, "hr": relHours, "hrs": relHours, "hour": relHours, "hours": relHours,
	"d": relDays, "day": relDays, "days": relDays,
	"w": relWeeks, "wk": relWeeks, "week": relWeeks, "weeks": relWeeks,
	"m": relMonths, "mo": relMonths, "mon": relMonths, "month": relMonths, "months": relMonths,
	"y": relYears, "yr": relYears, "year": relYears, "years": relYears,
}

func relHours(n int) RelSpec  { return RelSpec{Dur: time.Duration(n) * time.Hour} }
func relDays(n int) RelSpec   { return RelSpec{Days: n} }
func relWeeks(n int) RelSpec  { return RelSpec{Days: 7 * n} }
func relMonths(n int) RelSpec { return RelSpec{Months: n} }
func relYears(n int) RelSpec  { return RelSpec{Years: n} }

// parseRelSpec parses a relative duration written as a number followed by a
// unit, with both short and long spellings: h/hour(s), d/day(s), w/week(s),
// m/mo/month(s), y/year(s). An optional space between the number and unit is
// allowed (e.g. "2 months").
func parseRelSpec(s string) (*RelSpec, bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return nil, false
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil {
		return nil, false
	}

	build, ok := relUnits[strings.ToLower(strings.TrimSpace(s[i:]))]
	if !ok {
		return nil, false
	}
	r := build(n)
	return &r, true
}

// parseAbsDate parses an absolute date in a few common layouts, interpreted in
// UTC. Message dates are stored and compared in UTC (see addDate), and relative
// durations resolve against the current instant, so interpreting absolute dates
// in UTC too keeps results independent of the host's local time zone.
func parseAbsDate(s string) (time.Time, bool) {
	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.UTC); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
