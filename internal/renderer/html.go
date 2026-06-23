package renderer

import (
	"html"
	"strings"
)

// HTMLToText converts an HTML message body to readable plain text. It removes
// comments (including Microsoft Outlook conditional comments), script/style/head
// metadata and tags; collapses HTML whitespace; separates block-level elements,
// table cells and list items sensibly; decodes entities; and strips the
// invisible padding characters favoured by marketing templates.
func HTMLToText(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	s = normalizeNewlines(s)
	s = stripComments(s)
	s = stripBlock(s, "script")
	s = stripBlock(s, "style")
	s = stripBlock(s, "title")

	var out sink
	walkHTML(s, &out)
	return collapseBlankLines(out.String())
}

// walkHTML scans the HTML, feeding text runs and tag breaks into the sink.
func walkHTML(s string, out *sink) {
	for i := 0; i < len(s); {
		if s[i] != '<' {
			j := strings.IndexByte(s[i:], '<')
			if j < 0 {
				out.text(html.UnescapeString(s[i:]))
				return
			}
			out.text(html.UnescapeString(s[i : i+j]))
			i += j
			continue
		}
		end := strings.IndexByte(s[i:], '>')
		if end < 0 {
			return // unterminated tag: drop the rest
		}
		name, closing := tagName(s[i+1 : i+end])
		applyTag(out, name, closing)
		i += end + 1
	}
}

// applyTag translates an element into spacing in the output stream.
func applyTag(out *sink, name string, closing bool) {
	switch {
	case name == "br" || name == "hr":
		out.newline(1)
	case name == "li":
		if closing {
			out.newline(1)
		} else {
			out.bullet()
		}
	case name == "td" || name == "th":
		out.separator() // keep adjacent cells from running together
	case isParagraphTag(name):
		out.newline(2)
	case isLineTag(name):
		out.newline(1)
	}
}

// isParagraphTag reports whether the element should be set off by a blank line.
func isParagraphTag(name string) bool {
	switch name {
	case "p", "h1", "h2", "h3", "h4", "h5", "h6",
		"blockquote", "pre", "ul", "ol", "table":
		return true
	}
	return false
}

// isLineTag reports whether the element should start a new (non-blank) line.
func isLineTag(name string) bool {
	switch name {
	case "div", "tr", "section", "article", "header", "footer", "aside",
		"main", "nav", "figure", "figcaption", "caption", "address",
		"dl", "dt", "dd", "fieldset", "form":
		return true
	}
	return false
}

// tagName extracts the lower-cased element name and whether it is a closing tag
// from the raw text between '<' and '>'.
func tagName(raw string) (name string, closing bool) {
	raw = strings.TrimSpace(raw)
	closing = strings.HasPrefix(raw, "/")
	raw = strings.TrimPrefix(raw, "/")
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case ' ', '\t', '\n', '/', '>':
			return strings.ToLower(raw[:i]), closing
		}
	}
	return strings.ToLower(raw), closing
}

// stripComments removes every HTML comment, including Outlook conditional
// comments. Downlevel-revealed comments (<!--[if !mso]><!--> ... <!--<![endif]-->)
// are pairs of complete comments around visible content, so removing each
// <!-- ... --> span correctly keeps that content while dropping mso-only blocks.
func stripComments(s string) string {
	for {
		start := strings.Index(s, "<!--")
		if start < 0 {
			return s
		}
		rel := strings.Index(s[start+4:], "-->")
		if rel < 0 {
			return s[:start] // unterminated comment: drop the rest
		}
		s = s[:start] + s[start+4+rel+3:]
	}
}

// stripBlock removes every <tag>...</tag> block (case-insensitive), including
// its text content. An unterminated block drops everything from the open tag.
func stripBlock(s, tag string) string {
	for {
		lower := strings.ToLower(s)
		start := strings.Index(lower, "<"+tag)
		if start < 0 {
			return s
		}
		closeTag := "</" + tag + ">"
		rel := strings.Index(lower[start:], closeTag)
		if rel < 0 {
			return s[:start]
		}
		s = s[:start] + s[start+rel+len(closeTag):]
	}
}
