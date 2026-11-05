package event

import "github.com/adams-shaun/gorge/draft/limited"

// The sealed-mode supply: a sealed event's packs are its seats' pools, so
// the DRAFT supply phase is only the pack-opening round and the draft pick
// machinery never engages.

// sealedSupplyDone reports whether every seat's pack is opened, the point
// where a sealed event's phase becomes BUILD.
func (e *Event) sealedSupplyDone() bool {
	return e.mode == ModeSealed && e.packCount == e.NumSeats()*e.supplyRounds()
}

// poolSealed is a sealed seat's pool: its packs concatenated in pack order
// (pack 1 first), as a copy.
func (e *Event) poolSealed(seat int) []limited.CardRef {
	if seat < 0 || seat >= e.NumSeats() {
		return nil
	}
	var out []limited.CardRef
	for p := 0; p < e.supplyRounds(); p++ {
		out = append(out, e.packs[p][seat]...)
	}
	return out
}

// supplyRounds is the pack rounds a seat opens: its boosters, then its promos.
func (e *Event) supplyRounds() int { return e.packsPer + e.promos }

// PromoCards is a copy of the seat's promo cards (pack rounds packsPer+1..),
// nil for an out-of-range seat or an event with no promos.
func (e *Event) PromoCards(seat int) []limited.CardRef {
	if seat < 0 || seat >= e.NumSeats() || e.promos == 0 {
		return nil
	}
	var out []limited.CardRef
	for p := e.packsPer; p < e.supplyRounds(); p++ {
		out = append(out, e.packs[p][seat]...)
	}
	return out
}
