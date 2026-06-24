package version

import "testing"

func TestFormat(t *testing.T) {
	cases := []struct {
		name         string
		version, rev string
		dirty        bool
		want         string
	}{
		{"no vcs", "0.7.2", "", false, "0.7.2"},
		{"no vcs ignores dirty", "0.7.2", "", true, "0.7.2"},
		{"clean", "0.7.2", "abc123def456", false, "0.7.2 (abc123def456)"},
		{"dirty", "0.7.2", "abc123def456", true, "0.7.2 (abc123def456, dirty)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := format(c.version, c.rev, c.dirty); got != c.want {
				t.Fatalf("format(%q,%q,%v) = %q, want %q", c.version, c.rev, c.dirty, got, c.want)
			}
		})
	}
}
