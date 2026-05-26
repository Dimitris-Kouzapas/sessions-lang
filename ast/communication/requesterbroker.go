package communication

import (
	"sync"
)

// NamedBroker coordinates:
// - exactly one authorised requester identity
// - a fixed set of authorised acceptor identities
//
// Semantics:
//   - Request(requester) opens a round, but only if requester is the authorised one
//   - Accept(acceptor) blocks until a round is open
//   - each acceptor may accept at most once per round
//   - the next request blocks until all named acceptors have accepted


type RequesterBroker[T any] struct {
	mu   sync.Mutex
	cond *sync.Cond

	requester string

	running bool

	acceptorsList []string
	acceptors map[string]struct{}

	open  bool
	round int

	newChannels func() map[string]map[string]Channel[T]
	channels    map[string]map[string]Channel[T]

	joined map[string]struct{}
}

func NewRequesterBroker[T any](
	requester string,
	acceptors []string,
	newChannels func() map[string]map[string]Channel[T],
) *RequesterBroker[T] {
	accs := make(map[string]struct{}, len(acceptors))
	for _, a := range acceptors {
		accs[a] = struct{}{}
	}

	b := &RequesterBroker[T]{
		requester:   requester,
		running:     true,
		acceptorsList: acceptors,
		acceptors:   accs,
		newChannels: newChannels,
		joined:      make(map[string]struct{}, len(acceptors)),
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *RequesterBroker[T]) Copy() *RequesterBroker[T] {
	return NewRequesterBroker[T](b.requester, b.acceptorsList, b.newChannels)
}

// Stop prevents new rounds from opening.
// If a round is already open, it is allowed to finish.
func (b *RequesterBroker[T]) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.running = false
	b.cond.Broadcast()
}

// Start allows new rounds again.
func (b *RequesterBroker[T]) Start() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.running = true
	b.cond.Broadcast()
}

// Optional helper.
func (b *RequesterBroker[T]) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func (b *RequesterBroker[T]) Request(requester string) (map[string]map[string]Channel[T], bool, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if requester != b.requester {
		return nil, b.running, false
	}

	// Wait until the previous round finishes.
	for b.open {
		b.cond.Wait()
	}

	// If stopped, do not start a new round.
	if !b.running {
		return nil, false, true
	}

	b.round++
	b.open = true
	b.joined = make(map[string]struct{}, len(b.acceptors))
	b.channels = b.newChannels()

	b.cond.Broadcast()
	return b.channels, true, true
}

func (b *RequesterBroker[T]) Accept(acceptor string) (map[string]map[string]Channel[T], bool, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.acceptors[acceptor]; !ok {
		return nil, b.running, false
	}

	for {
		// No open round.
		for !b.open {
			// If stopped and no round is open, nothing more to accept.
			if !b.running {
				return nil, false, true
			}
			b.cond.Wait()
		}

		// Open round exists. If this acceptor has not yet joined, proceed.
		if _, already := b.joined[acceptor]; !already {
			break
		}

		// This acceptor already joined this round.
		// Wait either for next round or for stop.
		b.cond.Wait()
	}

	b.joined[acceptor] = struct{}{}

	if len(b.joined) == len(b.acceptors) {
		b.open = false
	}

	b.cond.Broadcast()
	return b.channels, true, true
}
