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

// The MoveZone Pairs payload tags every entry-counter word with the top bit
// so generic event-reference walkers (trigMustVisit, trigZonesCatchUp, the
// combat matchers) never mistake counter metadata for an object ID: a real
// object id can never carry the bit (state/ids.go's NextID range), so G.Obj
// returns nil for every tagged word and a tagged word never equals a real
// referent. The table kinds (LOYALTY/P1P1/LORE/DEFENSE) keep their historic
// fixed-index tag -- the one committed shape -- while every other kind rides
// a second tag bit plus a length-prefixed UTF-8 byte payload in the pairs
// that follow it. Both forms stay inside the existing Pairs field, so the
// frozen events.Event struct and the log encoding are untouched.
const (
	entryCounterMarker       = state.ObjID(1) << 31
	entryCounterStringMarker = state.ObjID(1) << 30
	// entryCounterKindBytes bounds a decoded UTF-8 payload. The longest real
	// counter kind is a short word; the cap only rejects a corrupt log.
	entryCounterKindBytes = 64
	// entryCounterPayloadMask is the bit field a string payload pair may use
	// for its three packed bytes. Three bytes (not four) leave the two marker
	// bits clear, so a byte's own high bits can never be clipped by the
	// marker OR at encode or decode time.
	entryCounterPayloadMask = state.ObjID(0x00FFFFFF)
)

// EntryCounterPairs encodes one replacement-adjusted entry grant into the
// MoveZone Pairs payload. A table kind is one pair; any other non-empty kind
// is a header pair (tag + byte length, tag + amount) followed by as many
// three-byte little-endian payload pairs as the kind needs. A grant with no
// amount encodes nothing (a fully blocked or zero placement).
func EntryCounterPairs(g EntryCounterGrant) [][2]state.ObjID {
	if g.Amount <= 0 || g.Kind == "" {
		return nil
	}
	for i := 1; i < len(entryCounterKinds); i++ {
		if g.Kind == entryCounterKinds[i] {
			return [][2]state.ObjID{{state.ObjID(i) | entryCounterMarker, state.ObjID(g.Amount) | entryCounterMarker}}
		}
	}
	if len(g.Kind) > entryCounterKindBytes {
		return nil
	}
	n := len(g.Kind)
	out := [][2]state.ObjID{{
		entryCounterMarker | entryCounterStringMarker | state.ObjID(n),
		entryCounterMarker | state.ObjID(uint32(g.Amount)),
	}}
	for off := 0; off < n; off += 3 {
		var w state.ObjID
		for b := 0; b < 3 && off+b < n; b++ {
			w |= state.ObjID(g.Kind[off+b]) << (8 * b)
		}
		out = append(out, [2]state.ObjID{entryCounterMarker | entryCounterStringMarker | w, entryCounterMarker})
	}
	return out
}

// applyEntryCounterPairs installs the encoded grants on the entering object
// during its MoveZone/TokenCreate/CardToken fold. It is a sequential decode:
// a table pair is one grant, a string header consumes exactly the payload
// pairs its length names, so the two forms cannot be confused. Malformed or
// truncated payloads are skipped whole, never half-decoded.
func applyEntryCounterPairs(o *state.Object, pairs [][2]state.ObjID) {
	for i := 0; i < len(pairs); i++ {
		pair := pairs[i]
		if pair[0]&entryCounterMarker == 0 {
			continue
		}
		if pair[0]&entryCounterStringMarker != 0 {
			n := int(pair[0] &^ (entryCounterMarker | entryCounterStringMarker))
			amount := int32(pair[1] &^ entryCounterMarker)
			if n <= 0 || n > entryCounterKindBytes {
				continue
			}
			buf := make([]byte, 0, (n+2)/3*3)
			for c := 0; c < (n+2)/3 && i+1 < len(pairs); c++ {
				i++
				if pairs[i][0]&(entryCounterMarker|entryCounterStringMarker) != (entryCounterMarker | entryCounterStringMarker) {
					buf = buf[:0]
					break
				}
				w := pairs[i][0] & entryCounterPayloadMask
				buf = append(buf, byte(w), byte(w>>8), byte(w>>16))
			}
			if len(buf) < n {
				continue
			}
			if amount > 0 {
				o.AddCounter(string(buf[:n]), amount)
			}
			continue
		}
		kind := pair[0] &^ entryCounterMarker
		amount := pair[1] &^ entryCounterMarker
		if kind > 0 && int(kind) < len(entryCounterKinds) && amount > 0 {
			o.AddCounter(entryCounterKinds[kind], int32(amount))
		}
	}
}

// EntryCounterKindEncodable reports whether a counter kind can travel in the
// MoveZone EntryCounterPairs payload. Every non-empty kind up to the payload's
// length bound is encodable -- the table kinds by their fixed index, every
// other kind by the UTF-8 payload form -- so rules/entry_counters.go may fold
// any body-defined entry grant without silently dropping it.
func EntryCounterKindEncodable(kind string) bool {
	return kind != "" && len(kind) <= entryCounterKindBytes
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
		// CR 306.5b: a planeswalker enters with loyalty counters equal to its
		// starting loyalty. A CopySpellAbility SetLoyalty$ rider (Ob Nixilis,
		// the Adversary's Casualty:X copy) names the starting loyalty
		// directly, replacing the printed face value; a 0 value is real (the
		// copy enters with no loyalty counters). A face whose printed
		// starting loyalty this engine cannot read (absent, non-numeric) or
		// the dynamic Loyalty:X form grants nothing, the same total
		// fail-closed stance the fold took. The Phyrexian compleated payment
		// (CR 702.143) lowers it by the life paid.
		if o.CopyLoyaltySet {
			loyalty := o.CopyLoyalty - o.CompleatedLifePaid
			if loyalty < 0 {
				loyalty = 0
			}
			grants = append(grants, EntryCounterGrant{Kind: "LOYALTY", Amount: loyalty})
		} else if n, err := strconv.Atoi(strings.TrimSpace(f.Loyalty)); err == nil && n > 0 {
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
