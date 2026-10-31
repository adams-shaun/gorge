package draft

import (
	"errors"
	"fmt"

	"github.com/adams-shaun/gorge/draft/limited"
)

// MinSeats and MaxSeats bound a pod built with NewPod.
const (
	MinSeats = 2
	MaxSeats = 8
)

// ErrPodShape reports packs that do not form a pod: fewer than MinSeats or
// more than MaxSeats seats, no rounds, or seats holding different numbers
// of rounds.
var ErrPodShape = errors.New("draft: bad pod shape")

// NewPod starts a pod of len(packs) seats and len(packs[0]) rounds:
// packs[s][r-1] is the pack seat s opens in round r. Odd rounds pass left,
// to (seat+1)%n; even rounds pass right, to (seat+n-1)%n. For 4 seats and 3
// rounds this is exactly New. The packs are copied; the caller may reuse
// its slices.
func NewPod(seed uint64, packs [][][]limited.CardRef) (*Pod, error) {
	seats := len(packs)
	if seats < MinSeats || seats > MaxSeats {
		return nil, fmt.Errorf("%w: %d seats, want %d..%d", ErrPodShape, seats, MinSeats, MaxSeats)
	}
	rounds := len(packs[0])
	if rounds < 1 {
		return nil, fmt.Errorf("%w: %d rounds, want at least 1", ErrPodShape, rounds)
	}
	for s, packRow := range packs {
		if len(packRow) != rounds {
			return nil, fmt.Errorf("%w: seat %d holds %d rounds, want %d", ErrPodShape, s, len(packRow), rounds)
		}
	}
	p := &Pod{
		seed:   seed,
		packs:  make([][][]limited.CardRef, seats),
		round:  1,
		queues: make([][]packState, seats),
		pools:  make([][]limited.CardRef, seats),
		seen:   make([][][]limited.CardRef, seats),
		pickNo: make([]int, seats),
		seats:  seats,
		rounds: rounds,
	}
	for s := 0; s < seats; s++ {
		p.packs[s] = make([][]limited.CardRef, rounds)
		for r := 0; r < rounds; r++ {
			p.packs[s][r] = append([]limited.CardRef(nil), packs[s][r]...)
		}
		p.queues[s] = []packState{{originSeat: s, round: 1, remaining: append([]limited.CardRef(nil), packs[s][0]...)}}
	}
	return p, nil
}

// Seats reports the pod size.
func (p *Pod) Seats() int { return p.seats }

// Rounds reports the number of pack rounds.
func (p *Pod) Rounds() int { return p.rounds }
