package events

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// EntryCounterNotice marks a logged CounterChange whose placement was already
// folded atomically in the preceding MoveZone. The event still notifies triggers.
const EntryCounterNotice = "__entry_counter_notice"

var entryCounterKinds = [...]string{"", "LOYALTY", "P1P1", "LORE", "DEFENSE"}

// EntryCounterPair encodes a replacement-adjusted entry grant in MoveZone's
// previously-unused Pairs payload. Both values use a high-bit tag so generic
// event-reference walkers never mistake counter metadata for an object ID.
func EntryCounterPair(g EntryCounterGrant) [2]state.ObjID {
	for i := 1; i < len(entryCounterKinds); i++ {
		if g.Kind == entryCounterKinds[i] {
			return [2]state.ObjID{state.ObjID(i) | 0x80000000, state.ObjID(g.Amount) | 0x80000000}
		}
	}
	return [2]state.ObjID{}
}

func applyEntryCounterPairs(o *state.Object, pairs [][2]state.ObjID) {
	for _, pair := range pairs {
		kind := pair[0] &^ state.ObjID(0x80000000)
		amount := pair[1] &^ state.ObjID(0x80000000)
		if pair[0]&0x80000000 != 0 && pair[1]&0x80000000 != 0 &&
			kind > 0 && int(kind) < len(entryCounterKinds) && amount > 0 {
			o.AddCounter(entryCounterKinds[kind], int32(amount))
		}
	}
}

// EntryCounterKindEncodable reports whether a counter kind can travel in the
// MoveZone EntryCounterPairs payload. The payload is a fixed-index tag (the
// hash-chained event shape is frozen), so only the kinds the table names can
// be folded; a body-defined grant of any other kind must be placed by its
// own body instead, or it would be silently dropped. rules/entry_counters.go
// gates body-grant absorption on this predicate.
func EntryCounterKindEncodable(kind string) bool {
	for i := 1; i < len(entryCounterKinds); i++ {
		if kind == entryCounterKinds[i] {
			return true
		}
	}
	return false
}

// EntryCounterGrant is one counter kind a permanent enters the battlefield
// with as an entry characteristic, and the amount of that kind.
type EntryCounterGrant struct {
	Kind   string
	Amount int32
}

// EntryCounterGrants returns the counters a permanent entering the
// battlefield grants ITSELF as part of that entry, in the order the entry
// fold used to place them: CR 306.5b starting loyalty, CR 702.54 Riot's
// counter election, CR 702.86 Unleash's, CR 702.151a a Saga's lore counter,
// CR 310.6 a Battle's defense counters. It is a pure function of the object
// as it stands in its ORIGIN zone (the logged elections and the cast-time
// CompleatedLifePaid are read here), plus whether this specific entry puts
// the card onto the battlefield face down -- a face-down entry is a 2/2
// creature with no abilities (CR 708.5), so it grants none of these.
//
// This is the ONE home for the entry-characteristic counter arithmetic.
// events.Move no longer folds them itself: rules snapshots this list just
// before the entry move folds and then places each grant through a real
// CounterChange event, so the CR 614 AddCounter replacement class and the
// CantPutCounter prohibition see an entry counter exactly like any other
// placement (task addcounter1/2). A log-only replay re-derives the same
// counters from those logged CounterChange events.
//
// The Riot "haste" election is deliberately NOT here: it grants a keyword,
// not a counter, and stays in events.Move where it always was.
func EntryCounterGrants(o *state.Object, faceDown bool) []EntryCounterGrant {
	if o == nil || faceDown {
		return nil
	}
	var grants []EntryCounterGrant
	// CR 306.5b: a planeswalker enters with loyalty counters equal to its
	// starting loyalty. A face whose starting loyalty this engine cannot read
	// (absent, non-numeric, or the Loyalty:X dynamic form) grants nothing,
	// the same total-fail-closed stance the fold took. The Phyrexian
	// compleated payment (CR 702.143) lowers it by the life paid.
	if f := o.Face(); f != nil && f.IsPlaneswalker() {
		if n, err := strconv.Atoi(strings.TrimSpace(f.Loyalty)); err == nil && n > 0 {
			loyalty := int32(n) - o.CompleatedLifePaid
			if loyalty < 0 {
				loyalty = 0
			}
			grants = append(grants, EntryCounterGrant{Kind: "LOYALTY", Amount: loyalty})
		}
	}
	// CR 702.54 (Riot): the logged "counter" election enters with a +1/+1
	// counter; the "haste" election grants a keyword and is handled in Move.
	if o.RiotChoice == "counter" {
		grants = append(grants, EntryCounterGrant{Kind: "P1P1", Amount: 1})
	}
	// CR 702.86 (Unleash): "counter" enters with a +1/+1 counter.
	if o.UnleashChoice == "counter" {
		grants = append(grants, EntryCounterGrant{Kind: "P1P1", Amount: 1})
	}
	// CR 702.151a (Sagas, kw:Chapter): "As this Saga enters ... add a lore
	// counter."
	if _, names := cards.SagaChapters(o.Face()); len(names) > 0 {
		grants = append(grants, EntryCounterGrant{Kind: "LORE", Amount: 1})
	}
	// CR 310.6/310.8: a Battle enters with defense counters equal to its
	// printed Defense; an absent/X/non-numeric value fails closed.
	if f := o.Face(); f != nil && f.IsBattle() {
		if n, err := strconv.Atoi(strings.TrimSpace(f.Defense)); err == nil && n > 0 {
			grants = append(grants, EntryCounterGrant{Kind: "DEFENSE", Amount: int32(n)})
		}
	}
	return grants
}
