package booster

import (
	"fmt"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// OpenPod opens a draft pod's packs (they feed
// draft/limited/draft): out[seat][pack] is that seat's copy of that pack, seat
// major and pack index second. Every pack is opened from its own
// rand.NewPCG(seed, salt) stream salted with the seat and pack index, so two
// pods sharing a seed see identical packs, and growing the pod (more seats
// or more packs) never shifts a pack another seat already holds: pack
// (seat, index) always draws from the same stream.
//
// ids must be one event-wide allocator (booster.IDs works): the pod's cards
// get sequential IDs in seat-major, pack-second, slot order.
//
// The pod deals spec.Unplayable substitutes as Open does; OpenPodLog also
// returns the record of each one.
func OpenPod(spec setdata.Spec, seed uint64, seats, packs int, ids IDSource, colourOf ColourOf) ([][][]limited.CardRef, error) {
	out, _, err := OpenPodLog(spec, seed, seats, packs, ids, colourOf)
	return out, err
}

// PodSubstitution is one Substitution a pod's pack made, with the seat and
// pack index that opened it.
type PodSubstitution struct {
	Seat int `json:"seat"`
	Pack int `json:"pack"`
	Substitution
}

// OpenPodLog is OpenPod returning every substitution the pod's packs made,
// in seat-major, pack-second, fill order: the same packs, from the same
// draws.
func OpenPodLog(spec setdata.Spec, seed uint64, seats, packs int, ids IDSource, colourOf ColourOf) ([][][]limited.CardRef, []PodSubstitution, error) {
	if seats <= 0 || packs <= 0 {
		return nil, nil, fmt.Errorf("booster: OpenPod needs seats and packs > 0, got %d and %d", seats, packs)
	}
	out := make([][][]limited.CardRef, seats)
	var log []PodSubstitution
	for seat := 0; seat < seats; seat++ {
		out[seat] = make([][]limited.CardRef, packs)
		for p := 0; p < packs; p++ {
			rng := rand.New(rand.NewPCG(seed, podSalt(seat, p)))
			pack, subs, err := OpenLog(spec, rng, ids, colourOf)
			if err != nil {
				return nil, nil, fmt.Errorf("booster: pod seat %d pack %d: %w", seat, p, err)
			}
			for _, s := range subs {
				log = append(log, PodSubstitution{Seat: seat, Pack: p, Substitution: s})
			}
			out[seat][p] = pack
		}
	}
	return out, log, nil
}

// podSalt is the per-pack stream salt: the seat in the high half and the
// pack index in the low half, so the pair is injective for realistic pod
// sizes and neither component's growth shifts the other's streams.
func podSalt(seat, pack int) uint64 {
	return uint64(seat)<<32 | uint64(uint32(pack))
}
