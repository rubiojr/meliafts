package renderer

import "strings"

// sink accumulates converted text while collapsing HTML whitespace the way a
// browser would: runs of whitespace become a single space, leading/trailing
// spaces on a line are dropped, and consecutive block boundaries collapse into
// at most a blank line. Breaks and spaces are buffered and only committed when
// real text follows, which keeps the output free of stray edge whitespace.
type sink struct {
	b          strings.Builder
	started    bool // any visible text has been written
	space      bool // a collapsed space is pending before the next text
	breaks     int  // pending newlines before the next text (0, 1 or 2)
	wantBullet bool // a list bullet is pending at the next line start
}

// text appends a run of (already entity-decoded) text, collapsing whitespace and
// dropping invisible characters.
func (s *sink) text(str string) {
	for _, r := range str {
		switch {
		case isSpaceRune(r):
			if s.started {
				s.space = true
			}
		case isInvisible(r):
			// drop zero-width / control / formatting padding
		default:
			s.commit()
			s.b.WriteRune(r)
			s.started = true
		}
	}
}

// newline requests up to n line breaks before the next text. It is a no-op
// before any text (so the output never starts with blank lines) and never
// requests more than two (a single blank line).
func (s *sink) newline(n int) {
	if !s.started {
		return
	}
	if n > 2 {
		n = 2
	}
	if n > s.breaks {
		s.breaks = n
	}
	s.space = false
}

// separator requests a single space between inline pieces (e.g. table cells).
func (s *sink) separator() {
	if s.started {
		s.space = true
	}
}

// bullet starts a new line introduced by a list marker.
func (s *sink) bullet() {
	s.newline(1)
	s.wantBullet = true
}

// commit flushes any pending breaks, bullet and space ahead of real text.
func (s *sink) commit() {
	if s.breaks > 0 {
		s.b.WriteString(strings.Repeat("\n", s.breaks))
		s.breaks = 0
		s.space = false
	}
	if s.wantBullet {
		s.b.WriteString("• ")
		s.wantBullet = false
		s.space = false
	}
	if s.space {
		s.b.WriteByte(' ')
		s.space = false
	}
}

func (s *sink) String() string { return s.b.String() }
