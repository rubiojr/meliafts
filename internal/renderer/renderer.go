// Package renderer turns a stored mail message body into readable plain text
// for terminal display. It deliberately knows nothing about styling, layout or
// the database: callers pass the raw stored fields and receive clean, unstyled
// text that they can then wrap and colour however they like.
//
// The HTML converter targets the messy real-world email that mail clients
// actually receive — marketing/MJML templates full of conditional comments,
// nested layout tables, inline CSS and invisible "preheader" padding — and
// distils it down to the text a human wants to read.
package renderer

import (
	"strings"
	"unicode"
)

// Body returns the best plain-text rendering of a message body. It prefers the
// plain-text part, falls back to converting the HTML part, and finally to the
// stored snippet. It returns an empty string when there is nothing to show, so
// callers can decide how to present an empty body.
func Body(bodyText, bodyHTML, snippet string) string {
	if t := cleanPlain(bodyText); t != "" {
		return t
	}
	if t := HTMLToText(bodyHTML); t != "" {
		return t
	}
	return cleanPlain(snippet)
}

// cleanPlain normalises a plain-text body: it unifies line endings, removes
// invisible/control characters, turns non-breaking and exotic spaces into
// ordinary spaces, trims trailing whitespace and collapses long runs of blank
// lines. Unlike the HTML path it preserves leading indentation and internal
// spacing, which can be meaningful in a plain-text mail.
func cleanPlain(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	s = normalizeNewlines(s)
	s = sanitizeKeepLayout(s)

	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return collapseBlankLines(strings.Join(lines, "\n"))
}

// normalizeNewlines converts CRLF and lone CR line endings to LF.
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// sanitizeKeepLayout drops invisible characters and maps Unicode spaces to a
// plain space while keeping newlines and tabs, so the original layout survives.
func sanitizeKeepLayout(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case unicode.Is(unicode.Zs, r):
			b.WriteByte(' ')
		case isInvisible(r):
			// drop zero-width / control / formatting characters
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// collapseBlankLines reduces three or more consecutive newlines to a single
// blank line and trims surrounding whitespace.
func collapseBlankLines(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

// isSpaceRune reports whether r is whitespace that should collapse in HTML flow:
// ASCII spaces/tabs/newlines plus any Unicode space separator (e.g. NBSP).
func isSpaceRune(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

// isInvisible reports whether r is a zero-width, formatting or control character
// that carries no visible meaning in a terminal. These are heavily abused in
// marketing mail to pad the inbox preview line, so stripping them is essential.
// Callers must handle wanted whitespace (space, tab, newline) before calling
// this, as ASCII control whitespace would otherwise be reported invisible.
func isInvisible(r rune) bool {
	switch r {
	case '\t', '\n', '\r', '\f', '\v':
		return false
	case 0x034F, // combining grapheme joiner (a Mark, so not caught by Cf)
		0x00AD: // soft hyphen
		return true
	}
	return unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Cc, r)
}
