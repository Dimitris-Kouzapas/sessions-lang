package communication

import (
	"sync"
)

// RecursorBroker coordinates a fixed set of authorised recursors.
//
// Semantics:
//   - Accept(recursor) is the only entry point
//   - if no round is open, the first authorised recursor opens a new round
//   - each recursor may accept at most once per round
//   - the last recursor to join closes the round
//   - after a round closes, the next arriving recursor opens the next round
type RecursorBroker struct {
	mu   sync.Mutex
	cond *sync.Cond

	running bool

	recursorsList []string
	recursors     map[string]struct{}

	open  bool
	round int

	joined map[string]struct{}
}

func NewRecursorBroker(
	recursors []string,
) *RecursorBroker {
	recs := make(map[string]struct{}, len(recursors))
	for _, a := range recursors {
		recs[a] = struct{}{}
	}

	b := &RecursorBroker{
		running:       true,
		recursorsList: append([]string(nil), recursors...),
		recursors:     recs,
		joined:        make(map[string]struct{}, len(recursors)),
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *RecursorBroker) Copy() *RecursorBroker {
	return NewRecursorBroker(b.recursorsList)
}

// Stop prevents new rounds from opening.
// If a round is already open, it is allowed to finish.
func (b *RecursorBroker) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.running = false
	b.cond.Broadcast()
}

// Start allows new rounds again.
func (b *RecursorBroker) Start() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.running = true
	b.cond.Broadcast()
}

func (b *RecursorBroker) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func (b *RecursorBroker) Recurse(recursor string) (bool, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.recursors[recursor]; !ok {
		return b.running, false
	}

	for {
		// If no round is open, this recursor becomes the opener of a new round.
		if !b.open {
			// If stopped and no round is open, terminate.
			if !b.running {
				return false, true
			}

			b.round++
			b.open = true
			b.joined = make(map[string]struct{}, len(b.recursors))
			break
		}

		// If a round is open and this recursor has not yet joined it, proceed.
		if _, already := b.joined[recursor]; !already {
			break
		}

		// This recursor already joined the current round.
		b.cond.Wait()
	}

	b.joined[recursor] = struct{}{}

	// Last recursor closes the round.
	if len(b.joined) == len(b.recursors) {
		b.open = false
	}

	b.cond.Broadcast()
	return true, true
}
