package actions

import (
	"testing"

	"github.com/rubiojr/meliafts/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func msgs(ids ...string) []store.Message {
	out := make([]store.Message, len(ids))
	for i, id := range ids {
		out[i] = store.Message{ID: id}
	}
	return out
}

func TestTrackerSeenAndFresh(t *testing.T) {
	tr := NewTracker()

	// Establish a baseline: nothing is "fresh" relative to it.
	tr.Seen(msgs("a", "b"))

	fresh := tr.Fresh(msgs("c", "a", "b"))
	require.Len(t, fresh, 1)
	assert.Equal(t, "c", fresh[0].ID)

	// "c" is now recorded; nothing fresh remains.
	assert.Empty(t, tr.Fresh(msgs("c", "a")))
}

func TestTrackerFreshPreservesOrder(t *testing.T) {
	tr := NewTracker()
	fresh := tr.Fresh(msgs("x", "y", "z"))
	assert.Equal(t, []string{"x", "y", "z"}, ids(fresh))
}

func ids(ms []store.Message) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.ID
	}
	return out
}
