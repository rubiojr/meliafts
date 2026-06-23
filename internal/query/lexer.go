package query

import (
	"fmt"
	"unicode"
)

// token is a single lexical unit of a query: one whitespace-separated term,
// already split into its optional negation, optional field name and value.
//
// Examples:
//
//	subject:hello      -> {field:"subject", value:"hello", hasField:true}
//	subject:"hi there" -> {field:"subject", value:"hi there", quoted:true, hasField:true}
//	-from:bob          -> {negated:true, field:"from", value:"bob", hasField:true}
//	unread:            -> {field:"unread", value:"", hasField:true}
//	"quick brown"      -> {value:"quick brown", quoted:true}
//	invoice            -> {value:"invoice"}
type token struct {
	negated  bool   // had a leading '-'
	field    string // lower-cased field name, empty when no "field:" prefix
	hasField bool   // a "field:" prefix was present
	value    string // the (unquoted) value text
	quoted   bool   // the value was supplied as a "quoted phrase"
}

// tokenize splits a raw query string into tokens. Terms are separated by
// whitespace, except that whitespace inside a double-quoted phrase is part of
// the value. An unterminated quote is reported as an error.
func tokenize(input string) ([]token, error) {
	r := []rune(input)
	var toks []token

	for i := 0; i < len(r); {
		if unicode.IsSpace(r[i]) {
			i++
			continue
		}
		tk, next, err := scanTerm(r, i)
		if err != nil {
			return nil, err
		}
		toks = append(toks, tk)
		i = next
	}

	return toks, nil
}

// scanTerm scans a single term starting at r[i] (which must be non-whitespace)
// and returns the token and the index just past it.
func scanTerm(r []rune, i int) (token, int, error) {
	var tk token
	i, tk.negated = scanNegation(r, i)

	// A term that starts with a quote is a bare quoted phrase (no field).
	if r[i] == '"' {
		val, next, err := readQuoted(r, i)
		if err != nil {
			return tk, i, err
		}
		tk.value, tk.quoted = val, true
		return tk, next, nil
	}

	// Read the "head": runes up to whitespace or a ':' field separator. When the
	// head is a valid identifier followed by ':', this is a field:value term.
	head, j := scanHead(r, i)
	if j < len(r) && r[j] == ':' && isIdent(head) {
		tk.field, tk.hasField = toLower(head), true
		val, quoted, next, err := scanValue(r, j+1) // skip ':'
		if err != nil {
			return tk, j, err
		}
		tk.value, tk.quoted = val, quoted
		return tk, next, nil
	}

	// Otherwise the whole run up to whitespace (including any ':') is a bare
	// value term.
	tk.value, j = scanBareWord(r, i)
	return tk, j, nil
}

// scanNegation consumes an optional leading '-' that is followed by more of the
// term, returning the new index and whether a negation was found.
func scanNegation(r []rune, i int) (int, bool) {
	if r[i] == '-' && i+1 < len(r) && !unicode.IsSpace(r[i+1]) {
		return i + 1, true
	}
	return i, false
}

// scanHead reads a run up to the next whitespace or ':'.
func scanHead(r []rune, i int) (string, int) {
	start := i
	for i < len(r) && !unicode.IsSpace(r[i]) && r[i] != ':' {
		i++
	}
	return string(r[start:i]), i
}

// scanBareWord reads a run up to the next whitespace (':' is included).
func scanBareWord(r []rune, i int) (string, int) {
	start := i
	for i < len(r) && !unicode.IsSpace(r[i]) {
		i++
	}
	return string(r[start:i]), i
}

// scanValue reads the value following a "field:" prefix: either a quoted phrase
// or an unquoted run up to the next whitespace.
func scanValue(r []rune, i int) (value string, quoted bool, next int, err error) {
	if i < len(r) && r[i] == '"' {
		val, nx, err := readQuoted(r, i)
		if err != nil {
			return "", false, i, err
		}
		return val, true, nx, nil
	}
	val, nx := scanBareWord(r, i)
	return val, false, nx, nil
}

// readQuoted reads a double-quoted phrase starting at r[i] (which must be '"').
// It returns the unquoted contents and the index just past the closing quote.
func readQuoted(r []rune, i int) (string, int, error) {
	// r[i] == '"'
	i++
	start := i
	for i < len(r) {
		if r[i] == '"' {
			return string(r[start:i]), i + 1, nil
		}
		i++
	}
	return "", i, fmt.Errorf("unterminated quote in query")
}

// isIdent reports whether s is a valid field-name identifier: a non-empty run
// of ASCII letters, digits and underscores starting with a letter or
// underscore. Only such heads are treated as "field:" operators; anything else
// containing a ':' is kept as literal search text.
func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for idx, c := range s {
		switch {
		case c == '_':
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
			if idx == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// toLower lower-cases an ASCII identifier.
func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
