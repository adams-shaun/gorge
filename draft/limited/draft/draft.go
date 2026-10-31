// Package draft runs an N-seat (2..8) booster-draft state machine — the
// pure pod half of spec §4.4; the hosted pod is 4 seats × 3 rounds. Packs
// are supplied by the caller (draft/limited/booster generates them from the event
// seed), so this package depends only on the limited vocabulary.
//
// Packs 1 and 3 pass left — a seat passes to (seat+1)%n — and pack 2 passes
// right, to (seat+n-1)%n. Each seat holds a FIFO queue of packs; a seat
// picks from the pack at the head of its queue, and the remainder moves on.
// The next round's packs open only when every seat has emptied the current
// round.
//
// The pod is deterministic regardless of interleaving: a pack visits seats
// in a fixed order and a pick depends only on the pack's contents, so any
// order of Pick calls that respects the queues yields identical pools and
// record. A single goroutine drives the pod, per the event architecture's
// one-driver rule; the methods take no locks.
package draft

import (
	"errors"

	"github.com/adams-shaun/gorge/draft/limited"
)

// Seats and Rounds are the hosted pod's shape (eventhost, New); NewPod
// takes any shape in [MinSeats, MaxSeats] × [1, …].
const (
	Seats  = 4
	Rounds = 3
)

// Pick errors. ErrStale means pickNo is not the seat's next expected pick
// number; ErrNotInPack means the ID is not in the pack at the seat's queue
// head; ErrNotYourTurn means the seat's queue is empty (the round barrier
// has not opened its next pack, or the draft is done).
var (
	ErrStale       = errors.New("draft: stale pick number")
	ErrNotInPack   = errors.New("draft: card not in pack")
	ErrNotYourTurn = errors.New("draft: no pack waiting for seat")
)

// PickView is one seat's view at a pick decision, with the field names spec
// §4.6 gives so draft/limited/pickbot can reuse the type as-is. Every slice is a
// copy: mutating a view never touches the pod.
type PickView struct {
	// Set is the printing set of the pack on offer (all of a pack's cards
	// share one set); empty when the pack is somehow empty.
	Set string
	// Round is the 1-based draft round the pack belongs to.
	Round int
	// PickNo is the seat's own 1-based pick count within the round.
	PickNo int
	// Pack is the pack on offer, minus what earlier seats took.
	Pack []limited.CardRef
	// Picks is the seat's own pool so far.
	Picks []limited.CardRef
	// Seen is every pack the seat has already picked from this draft, in
	// pick order, each as it stood when the seat saw it.
	Seen [][]limited.CardRef
	// Seats is the pod size: a public table fact every seat knows before
	// its first pick (it is what makes a wheel recognisable).
	Seats int
}

// Picked is one entry of the pod's append-only record.
type Picked struct {
	// Round is the 1-based draft round of the pick.
	Round int
	// PickNo is the seat's 1-based pick count within that round.
	PickNo int
	// Seat is the picking seat.
	Seat int
	// ID is the CardRef ID taken.
	ID string
}

// packState is one travelling pack: where it opened and what is left in it.
type packState struct {
	originSeat int
	round      int
	remaining  []limited.CardRef
}

// Pod is one draft's state machine.
type Pod struct {
	seed   uint64                // the event seed, carried for the event log layer
	packs  [][][]limited.CardRef // packs[seat][round-1] is what the seat opens that round
	seats  int
	rounds int

	round  int // the round currently in progress, 1-based
	queues [][]packState
	pools  [][]limited.CardRef
	seen   [][][]limited.CardRef
	pickNo []int // picks made by each seat in the current round

	record []Picked
}

// New starts a pod of the hosted 4×3 shape. packs[s][r-1] is the pack seat
// s opens in round r; the caller generates it (booster, seeded from the
// event seed) so the pod never depends on set data. The seed is accepted
// for symmetry with the event layer's API and carried on the pod; the state
// machine itself uses no randomness, because a pick's outcome depends only
// on the pack contents and the seat's own history.
//
// A [4][3] array is always in shape, so NewPod cannot fail here; an
// impossible error panics.
func New(seed uint64, packs [Seats][Rounds][]limited.CardRef) *Pod {
	sliced := make([][][]limited.CardRef, Seats)
	for s := range packs {
		sliced[s] = packs[s][:]
	}
	p, err := NewPod(seed, sliced)
	if err != nil {
		panic("draft: New: a 4×3 array is always in shape, got: " + err.Error())
	}
	return p
}

// Seed returns the event seed the pod was built with.
func (p *Pod) Seed() uint64 { return p.seed }

// Next reports whether seat has a pack to pick from, and what it looks like.
// It never mutates the pod.
func (p *Pod) Next(seat int) (PickView, bool) {
	if seat < 0 || seat >= p.seats {
		return PickView{}, false
	}
	q := p.queues[seat]
	if len(q) == 0 {
		return PickView{}, false
	}
	head := q[0]
	v := PickView{
		Round:  p.round,
		PickNo: p.pickNo[seat] + 1,
		Seats:  p.seats,
	}
	if len(head.remaining) > 0 {
		v.Set = head.remaining[0].Set
	}
	v.Pack = append([]limited.CardRef(nil), head.remaining...)
	v.Picks = append([]limited.CardRef(nil), p.pools[seat]...)
	v.Seen = copySeen(p.seen[seat])
	return v, true
}

// Pick takes the card id from the pack at the head of seat's queue. pickNo
// must be the seat's next expected pick number within the round; anything
// else is ErrStale. The remainder passes to the next seat in the round's
// direction; an emptied pack is dropped. When every queue is empty and
// rounds remain, the next round's packs open.
func (p *Pod) Pick(seat int, pickNo int, id string) error {
	if seat < 0 || seat >= p.seats {
		return ErrNotYourTurn
	}
	q := p.queues[seat]
	if len(q) == 0 {
		return ErrNotYourTurn
	}
	if pickNo != p.pickNo[seat]+1 {
		return ErrStale
	}
	head := &q[0]
	idx := -1
	for i, c := range head.remaining {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotInPack
	}
	card := head.remaining[idx]

	p.seen[seat] = append(p.seen[seat], append([]limited.CardRef(nil), head.remaining...))
	head.remaining = append(head.remaining[:idx], head.remaining[idx+1:]...)
	p.pools[seat] = append(p.pools[seat], card)
	p.pickNo[seat]++
	p.record = append(p.record, Picked{Round: p.round, PickNo: p.pickNo[seat], Seat: seat, ID: id})

	passed := *head
	p.queues[seat] = q[1:]
	if len(passed.remaining) > 0 {
		next := (seat + passOffset(p.round, p.seats)) % p.seats
		p.queues[next] = append(p.queues[next], passed)
	}
	p.openNextRound()
	return nil
}

// Done reports whether the draft is finished: every pack of every round has
// been emptied.
func (p *Pod) Done() bool {
	if p.round < p.rounds {
		return false
	}
	for s := range p.queues {
		if len(p.queues[s]) != 0 {
			return false
		}
	}
	return true
}

// Pool returns a copy of seat's drafted cards so far; an out-of-range seat
// gets nil.
func (p *Pod) Pool(seat int) []limited.CardRef {
	if seat < 0 || seat >= p.seats {
		return nil
	}
	return append([]limited.CardRef(nil), p.pools[seat]...)
}

// QueueLen returns how many packs are waiting in seat's queue; an
// out-of-range seat gets 0.
func (p *Pod) QueueLen(seat int) int {
	if seat < 0 || seat >= p.seats {
		return 0
	}
	return len(p.queues[seat])
}

// Record returns a copy of the append-only pick record.
func (p *Pod) Record() []Picked {
	return append([]Picked(nil), p.record...)
}

// passOffset is how far a pack travels in a round's direction in an n-seat
// pod: left for odd rounds, right for even rounds.
func passOffset(round int, n int) int {
	if round%2 == 0 {
		return n - 1
	}
	return 1
}

// openNextRound starts the next round when the round barrier is reached:
// every seat's queue empty and rounds left to play.
func (p *Pod) openNextRound() {
	if p.round >= p.rounds {
		return
	}
	for s := range p.queues {
		if len(p.queues[s]) != 0 {
			return
		}
	}
	p.round++
	for s := range p.queues {
		p.queues[s] = []packState{{originSeat: s, round: p.round, remaining: append([]limited.CardRef(nil), p.packs[s][p.round-1]...)}}
		p.pickNo[s] = 0
	}
}

// copySeen deep-copies a seat's seen-pack list.
func copySeen(seen [][]limited.CardRef) [][]limited.CardRef {
	out := make([][]limited.CardRef, len(seen))
	for i, pack := range seen {
		out[i] = append([]limited.CardRef(nil), pack...)
	}
	return out
}
