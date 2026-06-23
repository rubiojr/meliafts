package themes

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamesSortedAndComplete(t *testing.T) {
	names := Names()
	assert.Subset(t, names, []string{"amber", "green", "ice", "paper", "synthwave"})
	assert.True(t, sort.StringsAreSorted(names), "Names() must be sorted")
}

func TestGet(t *testing.T) {
	p, ok := Get(Default)
	require.True(t, ok)
	assert.Equal(t, Default, p.Name)

	_, ok = Get("nope")
	assert.False(t, ok)
}

func TestDefaultRegistered(t *testing.T) {
	_, ok := Get(Default)
	assert.True(t, ok, "default theme %q must be registered", Default)
}

// TestPalettesHaveBaseRoles guards against a theme file forgetting a required
// base color.
func TestPalettesHaveBaseRoles(t *testing.T) {
	for _, name := range Names() {
		p, _ := Get(name)
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, p.Bg, "Bg")
			assert.NotEmpty(t, p.BgAlt, "BgAlt")
			assert.NotEmpty(t, p.Fg, "Fg")
			assert.NotEmpty(t, p.Hi, "Hi")
			assert.NotEmpty(t, p.Dim, "Dim")
			assert.NotEmpty(t, p.Low, "Low")
			assert.NotEmpty(t, p.Accent, "Accent")
			assert.NotEmpty(t, p.Danger, "Danger")
		})
	}
}
