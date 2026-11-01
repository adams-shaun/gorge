package event

import "github.com/adams-shaun/gorge/draft/limited"

// The sealed-mode supply: a sealed event's packs are its seats' pools, so
// the DRAFT supply phase is only the pack-opening round and the draft pick
// machinery never engages.

// sealedSupplyDone reports whether every seat's pack is opened, the point
// where a sealed event's phase becomes BUILD.
func (e *Event) sealedSupplyDone() bool {
	return e.mode == ModeSealed && e.packCount == e.NumSeats()*e.packsPer
}

// poolSealed is a sealed seat's pool: its packs concatenated in pack order
// (pack 1 first), as a copy.
func (e *Event) poolSealed(seat int) []limited.CardRef {
	if seat < 0 || seat >= e.NumSeats() {
		return nil
	}
	var out []limited.CardRef
	for p := 0; p < e.packsPer; p++ {
		out = append(out, e.packs[p][seat]...)
	}
	return out
}
