package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// Amber CRT palette. A dark, warm background with phosphor-amber foregrounds in
// a few intensities, plus a hotter orange for flagged items and a red for
// errors.
var (
	colBg       = lipgloss.Color("#160d00") // near-black, warm
	colBgAlt    = lipgloss.Color("#2c1b04") // selection / panel background
	colAmber    = lipgloss.Color("#ffb000") // primary phosphor amber
	colAmberHi  = lipgloss.Color("#ffd982") // bright highlight
	colAmberDim = lipgloss.Color("#c08218") // dim amber
	colAmberLow = lipgloss.Color("#7a5410") // faint amber (rules, scrollbars)
	colFlagged  = lipgloss.Color("#ff8a1e") // hotter orange
	colDanger   = lipgloss.Color("#ff5436") // error red
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colBg).Background(colAmber)
	titleSubStyle = lipgloss.NewStyle().Foreground(colBg).Background(colAmber)

	statusStyle = lipgloss.NewStyle().Foreground(colAmberDim).Background(colBgAlt)

	ruleStyle = lipgloss.NewStyle().Foreground(colAmberLow)

	cursorMark = lipgloss.NewStyle().Foreground(colAmberHi).Bold(true)
	itemStyle  = lipgloss.NewStyle().Foreground(colAmber)
	itemDim    = lipgloss.NewStyle().Foreground(colAmberDim)
	itemSel    = lipgloss.NewStyle().Foreground(colAmberHi).Bold(true)

	flagUnread  = lipgloss.NewStyle().Foreground(colAmberHi).Bold(true)
	flagFlagged = lipgloss.NewStyle().Foreground(colFlagged).Bold(true)
	flagAttach  = lipgloss.NewStyle().Foreground(colAmberDim)
	bracketDim  = lipgloss.NewStyle().Foreground(colAmberLow)

	metaLabel  = lipgloss.NewStyle().Foreground(colAmberDim).Bold(true)
	metaValue  = lipgloss.NewStyle().Foreground(colAmber)
	subjectBig = lipgloss.NewStyle().Foreground(colAmberHi).Bold(true)
	bodyStyle  = lipgloss.NewStyle().Foreground(colAmber)

	emptyStyle = lipgloss.NewStyle().Foreground(colAmberDim).Italic(true)
	errStyle   = lipgloss.NewStyle().Foreground(colDanger).Bold(true)
)

// inputStyles builds the amber textinput theme.
func inputStyles() textinput.Styles {
	s := textinput.DefaultDarkStyles()

	prompt := lipgloss.NewStyle().Foreground(colAmberDim).Bold(true)
	text := lipgloss.NewStyle().Foreground(colAmberHi)
	placeholder := lipgloss.NewStyle().Foreground(colAmberLow)

	s.Focused.Prompt = prompt
	s.Focused.Text = text
	s.Focused.Placeholder = placeholder
	s.Blurred.Prompt = prompt.Foreground(colAmberLow)
	s.Blurred.Text = lipgloss.NewStyle().Foreground(colAmber)
	s.Blurred.Placeholder = placeholder
	s.Cursor.Color = colAmberHi

	return s
}
