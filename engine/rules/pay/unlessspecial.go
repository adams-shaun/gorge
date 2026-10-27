package pay

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// unlessspecial.go owns the unless cost's tap, counter, waterbend, blight and
// collect-evidence parts (CR 118.12a: the player may pay the cost; CR 118.3:
// a cost needs its resources). Each part has ONE candidate function
// (unlessSpecialCandidates) read by the offer gate (unlessChoiceComponentsPayable)
// and the payment walk (AdvanceUnless / AnswerUnlessPayment), so the offer and
// the charge cannot disagree. The settle helpers at the bottom (PayWaterbend,
// PayBlight, PayEvidence) are shared with the ward payment handler
// (engine/rules/ward.go), which pays the same keyword actions.

// The option kinds the unless walk's special picks carry on the wire. tapcost,
// blightcost, evidence and waterbend_generic are the cast flow's own kinds.
const (
	UnlessKindTap       = "tapcost"
	UnlessKindBlight    = "blightcost"
	UnlessKindCounter   = "countercost"
	UnlessKindWaterbend = "waterbend_generic"
	UnlessKindEvidence  = "evidence"
)

type unlessSpecialKindCode uint16

const (
	unlessSpecialTap unlessSpecialKindCode = iota + 1
	unlessSpecialBlight
	unlessSpecialCounter
	unlessSpecialWaterbend
	unlessSpecialEvidence
)

var unlessSpecialKinds = state.NewStrCodes(
	state.StrEntry[unlessSpecialKindCode]{Key: UnlessKindTap, Val: unlessSpecialTap},
	state.StrEntry[unlessSpecialKindCode]{Key: UnlessKindBlight, Val: unlessSpecialBlight},
	state.StrEntry[unlessSpecialKindCode]{Key: UnlessKindCounter, Val: unlessSpecialCounter},
	state.StrEntry[unlessSpecialKindCode]{Key: UnlessKindWaterbend, Val: unlessSpecialWaterbend},
	state.StrEntry[unlessSpecialKindCode]{Key: UnlessKindEvidence, Val: unlessSpecialEvidence},
)

// waterbendSpec is the permanents a waterbend tap may use (CR 701.67a).
const waterbendSpec = "Artifact,Creature"

// UnlessHasSpecialParts reports whether the cost carries a part the unless
// continuation owns beyond the Sac/Discard/Reveal/Behold/Return/Exile family.
func UnlessHasSpecialParts(c costvocab.Cost) bool {
	return len(c.TapPermanent) > 0 || len(c.AddCounter) > 0 || c.Waterbend > 0 ||
		len(c.Blight) > 0 || len(c.Evidence) > 0
}

// unlessSpecialCandidates is the one enumeration of what can pay a special
// part (or the ExileFromStack self exile). ok=false means kind is not one of
// theirs. seen holds already-committed picks.
func unlessSpecialCandidates(e Engine, payer state.PlayerID, ctx effects.Ctx, zone state.Zone, kind string, part costvocab.CostPart, seen map[state.ObjID]bool) (out []state.ObjID, ok bool) {
	g := e.Game()
	sc := ctx.SpecContext(payer)
	scan := func(p state.PlayerID, spec string, untapped bool) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if seen[id] || !ExistsOnBattlefield(o) || (untapped && o.Tapped) {
				continue
			}
			if e.Eval().MatchesSpec(spec, id, sc) {
				out = append(out, id)
			}
		}
	}
	switch unlessSpecialKinds.Code(kind) {
	case unlessSpecialTap:
		// CR 701.26a: only an untapped permanent can be tapped for a cost.
		scan(payer, part.Spec, true)
	case unlessSpecialBlight:
		// CR 701.68a: a creature you control; it need not be untapped.
		scan(payer, part.Spec, false)
	case unlessSpecialWaterbend:
		scan(payer, waterbendSpec, true)
	case unlessSpecialCounter:
		// The recipient may be any player's permanent (Sanctuary Wall's
		// targeted creature), so every battlefield is scanned in seat order.
		for p := range g.Players {
			scan(state.PlayerID(p), part.Target, false)
		}
	case unlessSpecialEvidence:
		out = EvidenceOrder(g, EvidenceGraveCandidates(g, payer, seen))
	default:
		// ExileFromStack<1/Card.Self>: the resolving spell itself, while it
		// is still on the stack.
		if kind != "exilecost" || zone != state.ZStack {
			return nil, false
		}
		if o := g.Obj(ctx.Source); o != nil && o.Zone == state.ZStack && !seen[o.ID] {
			out = append(out, o.ID)
		}
	}
	return out, true
}

// WaterbendTapsOwed is how many permanents the payer must tap to waterbend n
// (CR 701.67a): every {1} the floating pool cannot cover. The ward handler and
// the unless walk read the same figure.
func WaterbendTapsOwed(g *state.Game, payer state.PlayerID, n int32) int32 {
	return max(n-int32(g.Players[payer].Pool.Total()), 0)
}

// PayWaterbend settles a waterbend of n: the picked permanents tap for {1}
// each and the pool pays the rest. It reports false, having paid nothing, when
// more permanents were picked than {1}s owed or the pool cannot cover the rest.
func PayWaterbend(e Engine, payer state.PlayerID, n int32, taps []state.ObjID) bool {
	remaining := n - int32(len(taps))
	if remaining < 0 || !PayMana(e, payer, costvocab.Cost{Generic: remaining}) {
		return false
	}
	for _, id := range taps {
		e.Emit(events.Event{Kind: events.Tap, Obj: id})
	}
	return true
}

// PayBlight puts n -1/-1 counters on the chosen creature (CR 701.68a), by the
// payer. asCost marks the placement a cost's own (the unless path); the ward
// handler keeps its historical plain publication.
func PayBlight(e Engine, payer state.PlayerID, id state.ObjID, n int32, asCost bool) {
	e.EmitCounterBy(events.Event{Kind: events.CounterChange, Obj: id, Counter: "M1M1", Amount: n}, payer, asCost)
}

// PayEvidence exiles the chosen graveyard cards as the collect-evidence
// payment (CR 701.59a) when they are all in the payer's graveyard and their
// total mana value is at least n; otherwise nothing moves and it reports false.
func PayEvidence(e Engine, payer state.PlayerID, n int32, ids []state.ObjID, text string) bool {
	grave := e.Game().Zone(state.ZGraveyard, payer)
	for _, id := range ids {
		if !slices.Contains(grave, id) {
			return false
		}
	}
	if EvidenceManaValue(e.Game(), payer, ids) < n {
		return false
	}
	for _, id := range ids {
		e.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZExile, Text: text})
	}
	if len(ids) > 0 {
		e.Emit(events.Event{Kind: events.CollectEvidenceAction, Player: payer})
	}
	return true
}

// unlessSpecialPicks is how many objects the payer must pick for one special
// part, given its current candidates: lo==hi except for evidence, whose
// minimum is the fewest cards reaching the total. ok=false is an unpayable
// part. Offer, ask and answer all read this.
func unlessSpecialPicks(e Engine, u *UnlessPayment, kind string, part costvocab.CostPart, eligible []state.ObjID) (lo, hi int, ok bool) {
	switch unlessSpecialKinds.Code(kind) {
	case unlessSpecialTap:
		return int(part.N), int(part.N), len(eligible) >= int(part.N)
	case unlessSpecialBlight, unlessSpecialCounter:
		return 1, 1, len(eligible) >= 1
	case unlessSpecialWaterbend:
		need := int(WaterbendTapsOwed(e.Game(), u.Payer, part.N))
		return need, need, len(eligible) >= need
	case unlessSpecialEvidence:
		if EvidenceManaValue(e.Game(), u.Payer, eligible) < part.N {
			return 0, 0, false
		}
		return EvidenceGreedyMin(e.Game(), eligible, part.N), len(eligible), true
	}
	return 0, 0, false
}

// unlessSpecialAnswerOK rechecks a payer's pick for a special part against the
// live game: count in range, distinct, every object a current candidate, and
// for evidence the total reached.
func unlessSpecialAnswerOK(e Engine, u *UnlessPayment, kind string, part costvocab.CostPart, eligible, ids []state.ObjID) bool {
	lo, hi, ok := unlessSpecialPicks(e, u, kind, part, eligible)
	if !ok || len(ids) < lo || len(ids) > hi {
		return false
	}
	for i, id := range ids {
		if !slices.Contains(eligible, id) || slices.Contains(ids[:i], id) {
			return false
		}
	}
	return unlessSpecialKinds.Code(kind) != unlessSpecialEvidence || EvidenceManaValue(e.Game(), u.Payer, ids) >= part.N
}

// recordUnlessSpecialPick files a settled pick. Blights and CounterTo stay
// parallel to Cost.Blight / Cost.AddCounter (one object per part).
func recordUnlessSpecialPick(u *UnlessPayment, kind string, ids []state.ObjID) {
	switch unlessSpecialKinds.Code(kind) {
	case unlessSpecialTap:
		u.Taps = append(u.Taps, ids...)
	case unlessSpecialBlight:
		u.Blights = append(u.Blights, ids...)
	case unlessSpecialCounter:
		u.CounterTo = append(u.CounterTo, ids...)
	case unlessSpecialWaterbend:
		u.WaterbendTaps = append(u.WaterbendTaps, ids...)
	case unlessSpecialEvidence:
		u.Evidence = append(u.Evidence, ids...)
	}
}

// chargeUnlessWaterbend pays a Waterbend part before anything else is charged
// (the parser admits it only alone, so the pool is the whole price): false is
// a decline with nothing paid.
func chargeUnlessWaterbend(e Engine, u *UnlessPayment) bool {
	return u.Cost.Waterbend == 0 || PayWaterbend(e, u.Payer, u.Cost.Waterbend, u.WaterbendTaps)
}

// chargeUnlessSpecial emits the settled picks of the special parts: taps,
// blight counters, the unless AddCounter placements and the evidence exile.
// Every pick was proved by unlessSpecialAnswerOK / the exact-candidate record.
func chargeUnlessSpecial(e Engine, u *UnlessPayment) {
	for _, id := range u.Taps {
		e.Emit(events.Event{Kind: events.Tap, Obj: id})
	}
	for i, id := range u.Blights {
		PayBlight(e, u.Payer, id, u.Cost.Blight[i].N, true)
	}
	for i, id := range u.CounterTo {
		part := u.Cost.AddCounter[i]
		e.EmitCounterBy(events.Event{Kind: events.CounterChange, Obj: id, Counter: part.Spec, Amount: part.N}, u.Payer, true)
	}
	if len(u.Evidence) > 0 {
		need := int32(0)
		for _, part := range u.Cost.Evidence {
			need = costvocab.AddClampedGeneric(need, int64(part.N))
		}
		PayEvidence(e, u.Payer, need, u.Evidence, "collected as evidence as a cost")
	}
}
