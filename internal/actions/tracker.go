package actions

import "github.com/rubiojr/meliafts/internal/store"

// Tracker remembers which message ids have already been seen, so a caller can
// tell newly-arrived messages from an existing baseline. It is the shared
// new-message detector behind both the `ms watch` Poller and the TUI reload
// loop. A Tracker is not safe for concurrent use; ids are never evicted, so a
// message that scrolls out of and back into a result set is not reported twice.
type Tracker struct {
	seen map[string]bool
}

// NewTracker returns an empty Tracker.
func NewTracker() *Tracker {
	return &Tracker{seen: make(map[string]bool)}
}

// Seen records every message as already known and returns nothing. Use it to
// establish or extend the baseline — the first load, a fresh query, or an
// appended page — none of which should fire actions.
func (t *Tracker) Seen(msgs []store.Message) {
	for _, m := range msgs {
		t.seen[m.ID] = true
	}
}

// Fresh returns the messages whose ids have not been seen before, in input
// order, and records them as seen.
func (t *Tracker) Fresh(msgs []store.Message) []store.Message {
	var out []store.Message
	for _, m := range msgs {
		if !t.seen[m.ID] {
			t.seen[m.ID] = true
			out = append(out, m)
		}
	}
	return out
}
