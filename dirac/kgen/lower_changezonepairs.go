package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// per-card ChangesZone lines on a zone pair the
// ETB / Dies / Leaves split does not name. gorge reads Origin$ and
// Destination$ as zone SETS off the move event (rules/trigmatch/zone.go
// ZoneChangeMatchesWithCapture); the kernel's three events carry the sets in
// existing fields: a Leaves line's Trigger.N is the destination mask
// (1<<kop.Zone, 0 = any), a Leaves line's Trigger.From the graveyard it
// leaves, an ETB line's Trigger.From the origin it must enter from.

// czDestSet reads a comma Destination$ as a kop.Zone mask (landOriginBits:
// Ante and Command drop). ok is false for a single word (zoneOf reads it); a
// word the kernel does not model, or an empty mask, refuses. Battlefield is
// not a word landOriginBits knows, so a set naming it refuses.
func czDestSet(tp *params) (mask int16, ok bool, err error) {
	v, has := tp.get("Destination")
	if !has || !strings.Contains(v, ",") {
		return 0, false, nil
	}
	for _, w := range strings.Split(v, ",") {
		bit, drop, known := landOriginBits(strings.TrimSpace(w))
		if !known {
			return 0, false, refuse("ChangesZone Destination %q", v)
		}
		if !drop {
			mask |= bit
		}
	}
	if mask == 0 {
		return 0, false, refuse("ChangesZone Destination %q", v)
	}
	return mask, true, nil
}

// czPair lowers the pairs the three-event split does not name. ok is false
// when the pair is none of them (the caller refuses). An Effect's or a
// non-battlefield trigger (an emblem, a graveyard ETB) refuses: the kernel
// checks the zone only on a battlefield listener's walk.
func czPair(tr *lTrigger, mask int16, zone string, giftSelf, costBody bool) (ok bool, err error) {
	if mask != 0 && tr.Origin != zBattlefield || giftSelf {
		return false, nil
	}
	if costBody {
		// the body folds a non-mandatory pay cost into the trigger's one
		// question (Azorius Aethermage's "you may pay {1}"): the lockstep
		// diverges on which land the autopay taps, so the shape stays
		// refused until that is read exactly
		return false, refuse("ChangesZone %s->%s with an optional pay-cost body", zoneLabel[tr.Origin], zoneLabel[tr.Dest])
	}
	switch {
	case mask != 0:
		// a ValidCard$ read on the source's host (Kaya's Ghostform's
		// "enchanted permanent dies or is put into exile") sees the
		// attachment as the host leaves: the kernel fires leave lines
		// before the permanent is removed (CR 603.10a, zones.go leave), and
		// the returned card in exile is that card (trig_objmoved.go)
		tr.Event, tr.N = "Leaves", mask
	case tr.Origin == zGraveyard && tr.Dest == zAny:
		tr.Event, tr.From = "Leaves", zGraveyard
	case tr.Origin == zBattlefield && (tr.Dest == zHand || tr.Dest == zExile || tr.Dest == zLibrary):
		tr.Event, tr.N = "Leaves", 1<<zoneBit(tr.Dest)
	case tr.Dest == zBattlefield && tr.Origin == zStack && czReadsOwnCast(tr.F):
		// every alternative holds only for the resolving spell becoming the
		// permanent (PredWasCastEnter, not negated: kernel enterCast), which
		// moves from the stack, so Origin$ Stack adds nothing to the enter
		tr.Event = "ETB"
	case tr.Origin == zLibrary && tr.Dest == zGraveyard:
		// "a creature card is put into a graveyard from a library"
		// (Dreadhound): gorge matches ValidCard$ against each card the move
		// puts into a graveyard from a library (trigmatch
		// ZoneChangeMatchesWithCapture, one trigger per card); the kernel
		// fires TrigToGraveyard From Library where the card is placed
		// (trig_libtograve.go). The card's own line (Narcomoeba) listens
		// from the graveyard it lands in, which no battlefield walk sees.
		if czReadsSelf(tr.F) {
			return false, refuse("ChangesZone Library->Graveyard of the card itself")
		}
		tr.Event, tr.From = "ToGraveyard", zLibrary
	case tr.Dest == zBattlefield && tr.Origin == zExile:
		// gorge matches a source that enters itself against its LKI snapshot,
		// still in Exile, so a bare Permanent ValidCard$ never reads true of
		// Fire Lord Zuko blinked (Forge and XMage fire it)
		return false, refuse("ChangesZone Exile->Battlefield: gorge reads the entering source's ValidCard$ on its exile-zone LKI")
	case tr.Origin == zExile && tr.Dest == zAny && czImprintOnly(tr.F):
		// "an imprinted card leaves exile" (Mirror Golem's forget line,
		// Duplicant's imprinted-card cleanup): the kernel's TrigLeaveExile
		// fires in placeFromK while the card is still in exile (CR 603.10a
		// look-back; gorge judges the imprint association on the event's LKI
		// snapshot, whose zone is exile). Only a ValidCard$ every clause of
		// which is the imprint read lowers: the pre-move match of any other
		// word could read a zone-bound value the live object no longer holds.
		tr.Event = "LeaveExile"
	default:
		return false, nil
	}
	if zone != "" || tr.Zone != 0 && tr.Zone != zBattlefield {
		return false, refuse("ChangesZone %s->%s trigger zone %d", zoneLabel[tr.Origin], zoneLabel[tr.Dest], tr.Zone)
	}
	return true, nil
}

// czImprintOnly reports whether every clause of f is the bare imprint read:
// kop.W7Imprinted and nothing else, no flags and no unfolded ExiledWithSource
// link. The leave-exile firing carries exactly that match (the store read the
// card still in exile); any other word's pre-move match could read a
// zone-bound value the live object no longer holds.
func czImprintOnly(f *lFilter) bool {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Flags != 0 || c.CmpStat != cmpWord7 || c.CmpN != int(kop.W7Imprinted) {
			return false
		}
	}
	return true
}

// czReadsOwnCast reports a filter whose every clause reads the entering
// object's own cast (a non-negated cmpWasCast).
func czReadsOwnCast(f *lFilter) bool {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.CmpStat != cmpWasCast || c.CmpN&wasCastNot != 0 {
			return false
		}
	}
	return true
}

// czReadsSelf reports a filter clause that names the trigger's own card.
func czReadsSelf(f *lFilter) bool {
	if f == nil {
		return false
	}
	for _, c := range f.Clauses {
		if c.Flags&fSelf != 0 {
			return true
		}
	}
	return false
}

// zoneBit maps a kgen zone to its kop.Zone.
func zoneBit(z int) kop.Zone {
	switch z {
	case zHand:
		return kop.ZoneHand
	case zGraveyard:
		return kop.ZoneGraveyard
	case zExile:
		return kop.ZoneExile
	}
	return kop.ZoneLibrary
}

// czCostBody reports a trigger body whose ability carries a non-mandatory
// Cost$ (the optional pay of "you may pay {1}. If you do, ...").
func czCostBody(t cards.Trigger) bool {
	e := t.Effect
	if e == nil {
		return false
	}
	c := e.ParamStr(cards.PKCost)
	return c != "" && !strings.HasPrefix(c, "Mandatory")
}
