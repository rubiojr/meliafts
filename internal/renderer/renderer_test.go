package renderer

import (
	"io"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTMLToTextBasics(t *testing.T) {
	in := `<html><head><title>ignore me</title><style>.x{color:red}</style></head>
		<body><p>Hello <b>world</b></p><br>line two<script>alert(1)</script>
		<div>caf&eacute; &amp; tea</div></body></html>`
	out := HTMLToText(in)

	assert.Contains(t, out, "Hello world")
	assert.Contains(t, out, "line two")
	assert.Contains(t, out, "café & tea")
	assert.NotContains(t, out, "alert(1)")
	assert.NotContains(t, out, "color:red")
	assert.NotContains(t, out, "ignore me")
	assert.NotContains(t, out, "<")
}

func TestHTMLToTextTable(t *testing.T) {
	// Adjacent table cells must not run their text together.
	out := HTMLToText(`<table><tr><td>Mis pedidos</td><td>Mi cuenta</td></tr></table>`)
	assert.Contains(t, out, "Mis pedidos Mi cuenta")
	assert.NotContains(t, out, "pedidosMi")
}

func TestHTMLToTextLists(t *testing.T) {
	out := HTMLToText(`<ul><li>one</li><li>two</li></ul>`)
	assert.Equal(t, "• one\n• two", out)
}

func TestHTMLToTextCollapsesWhitespace(t *testing.T) {
	// Source indentation and newlines inside a block collapse to single spaces.
	out := HTMLToText("<div>a    b\n\n\t  c</div>")
	assert.Equal(t, "a b c", out)
}

func TestHTMLToTextParagraphs(t *testing.T) {
	out := HTMLToText(`<p>First</p><p>Second</p>`)
	assert.Equal(t, "First\n\nSecond", out)
}

func TestHTMLToTextComments(t *testing.T) {
	// Plain comments and Outlook mso-only blocks are dropped entirely...
	out := HTMLToText(`<p>A<!-- hidden -->B</p><!--[if mso]><o:PixelsPerInch>96</o:PixelsPerInch><![endif]-->`)
	assert.Equal(t, "AB", out)
	assert.NotContains(t, out, "96")

	// ...but downlevel-revealed content between paired comments is kept.
	revealed := HTMLToText(`<!--[if !mso]><!--><span>shown</span><!--<![endif]-->`)
	assert.Equal(t, "shown", revealed)
}

func TestHTMLToTextStripsInvisiblePadding(t *testing.T) {
	// Zero-width joiner, soft hyphen, combining grapheme joiner and a bidi mark
	// are the characters marketing mail uses to pad the inbox preview line.
	in := "Hi\u200c\u00ad\u034f\u202bthere\u200d"
	out := HTMLToText("<div>" + in + "</div>")
	assert.Equal(t, "Hithere", out)
	for _, r := range []rune{0x200C, 0x00AD, 0x034F, 0x202B, 0x200D} {
		assert.NotContains(t, out, string(r))
	}
}

func TestHTMLToTextNbsp(t *testing.T) {
	out := HTMLToText("a&nbsp;b\u00a0c")
	assert.Equal(t, "a b c", out)
	assert.NotContains(t, out, "\u00a0")
}

func TestHTMLToTextEmpty(t *testing.T) {
	assert.Equal(t, "", HTMLToText(""))
	assert.Equal(t, "", HTMLToText("   \n  "))
	assert.Equal(t, "", HTMLToText("<div></div>"))
}

func TestBodyPrefersPlainText(t *testing.T) {
	got := Body("  Plain body  ", "<p>HTML body</p>", "snippet")
	assert.Equal(t, "Plain body", got)
}

func TestBodyFallsBackToHTML(t *testing.T) {
	got := Body("", "<p>HTML body</p>", "snippet")
	assert.Equal(t, "HTML body", got)
}

func TestBodyFallsBackToSnippet(t *testing.T) {
	got := Body("", "", "just a snippet")
	assert.Equal(t, "just a snippet", got)
}

func TestBodyEmpty(t *testing.T) {
	assert.Equal(t, "", Body("", "", ""))
	assert.Equal(t, "", Body("  ", "<div> </div>", "\u00ad"))
}

func TestCleanPlainKeepsLayoutButStripsJunk(t *testing.T) {
	// Indentation on inner lines is preserved (it can be meaningful in plain
	// text), while trailing whitespace and invisible characters are removed,
	// CRLF is normalised, and blank runs collapse to a single blank line.
	in := "header\r\n    indented  \r\n\r\n\r\nlast\u200ctext\t\r\n"
	got := cleanPlain(in)
	assert.Equal(t, "header\n    indented\n\nlasttext", got)
}

// decodeEML extracts the decoded HTML body of a quoted-printable email, which is
// what a mail client like melia stores in body_html.
func decodeEML(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	msg, err := mail.ReadMessage(f)
	require.NoError(t, err)

	var body io.Reader = msg.Body
	if strings.EqualFold(msg.Header.Get("Content-Transfer-Encoding"), "quoted-printable") {
		body = quotedprintable.NewReader(msg.Body)
	}
	b, err := io.ReadAll(body)
	require.NoError(t, err)
	return string(b)
}

// TestHTMLToTextSampleEML renders a synthetic marketing email — a
// quoted-printable template stuffed with an Outlook conditional comment, a large
// inline stylesheet, nested layout tables and invisible preheader padding (the
// same shape as the real-world mail that motivated the renderer, but with no
// private data) — and checks that the readable content survives while the noise
// is gone.
func TestHTMLToTextSampleEML(t *testing.T) {
	htmlBody := decodeEML(t, "testdata/sample.eml")
	out := HTMLToText(htmlBody)

	// The meaningful content is present.
	for _, want := range []string{
		"Your order is ready for pickup",
		"Pick up before Friday",
		"Pickup Point - Example Store",
		"Order no. 000-0000000-0000000",
		"Start pickup",
		"Acme Wireless Microphone",
		"My orders",
	} {
		assert.Contains(t, out, want)
	}

	// None of the noise leaks through.
	for _, bad := range []string{
		"PixelsPerInch", // mso conditional comment content
		"mso",           // Outlook-only markup
		"color:",        // inline CSS
		"#FFFFFF",       // CSS colours
		"prefers-color-scheme",
		"!important",
		"{",
		"<",
	} {
		assert.NotContains(t, out, bad, "noise %q leaked into output", bad)
	}

	// The invisible preheader padding is stripped.
	for _, r := range []rune{0x200C, 0x200D, 0x00AD, 0x034F, 0x202B, 0xFEFF} {
		assert.NotContainsf(t, out, string(r), "invisible char U+%04X leaked", r)
	}

	// The distilled text is a tiny fraction of the original markup.
	assert.Less(t, len(out), len(htmlBody)/20)
}
