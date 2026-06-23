package actions

import (
	"context"

	"github.com/rubiojr/meliafts/internal/store"
)

// Poller watches a query for new messages and fires actions for them. It is the
// headless counterpart of the TUI reload loop: both detect new messages with a
// [Tracker] and fire them through [Runner.FireNew].
//
// The first Tick primes a silent baseline of whatever already matches the query,
// so a freshly started watcher does not replay the existing mailbox — unless
// FireExisting is set.
type Poller struct {
	Store        *store.Store
	Runner       *Runner
	Query        string
	Limit        int
	FireExisting bool

	tracker *Tracker
	primed  bool
}

// Tick runs the query once, fires actions for any newly-seen messages, and
// returns how many were fired. The first Tick primes the baseline (firing
// nothing unless FireExisting is set).
func (p *Poller) Tick(ctx context.Context) (int, error) {
	if p.tracker == nil {
		p.tracker = NewTracker()
	}

	results, err := p.Store.Search(p.Query, p.Limit, 0)
	if err != nil {
		return 0, err
	}

	// The first poll establishes the baseline silently by default.
	if !p.primed {
		p.primed = true
		if !p.FireExisting {
			p.tracker.Seen(results)
			return 0, nil
		}
	}

	return p.Runner.FireNew(ctx, p.Query, p.tracker.Fresh(results))
}
