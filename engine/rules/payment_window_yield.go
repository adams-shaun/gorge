package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// windowSourceYields reports whether source id has a mana ability the
// resolution-time payment window (paymentManaAskClass) can use that would add
// at least one mana right now. A source whose every such ability provably adds
// nothing -- Brigid, Doun's Mind's "{T}: Add X {G} or X {W}, where X is the
// number of other creatures you control" with no other creature -- cannot
// contribute to the payment, so the window does not offer to tap it: tapping
// it is a legal mana activation (CR 605.1a), but in a payment window it only
// taps the permanent for nothing. So does a production of a commander's colour
// identity with no commander (CR 903.4f, CR 106.5). Otherwise only the source's
// own printed abilities with a strictly resolved Each$ multiplier or Amount$ are
// judged; anything else (a gained ability, an unresolvable count, a literal
// amount) counts as yielding, so the window never loses a source that might pay.
func (e *Engine) windowSourceYields(p state.PlayerID, id state.ObjID) bool {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return true
	}
	for _, ma := range e.availableManaAbilities(p, id) {
		if pay.InstantSpeedOnly(ma) {
			continue
		}
		if !e.manaAbilityYieldsNothing(p, id, o.Face(), ma) {
			return true
		}
	}
	return false
}

// manaAbilityYieldsNothing is windowSourceYields' per-ability verdict: true
// when the ability produces a colour of an empty commander identity, or is the
// face's own and its Each$ multiplier or its non-literal Amount$ resolves
// (strictly) to zero or less. An announced source-anchored SubCounter<X/Kind>
// cost is the exception: the ability's produced mana IS those counters, so a
// zero Amount$ read with no X announced means "yields" exactly when the source
// carries none.
func (e *Engine) manaAbilityYieldsNothing(p state.PlayerID, id state.ObjID, f *cards.Face, ma *cards.SA) bool {
	mp := effects.ManaOf(ma)
	if pay.IsColourIdentityProduced(mp.Produced) && len(pay.CommanderIdentityColours(asPayer(e), p)) == 0 {
		return true
	}
	if !slices.Contains(f.Abilities, ma) {
		return false
	}
	if mp.Each.Present {
		return e.manaEachCount(p, id, ma, nil, pay.GainedManaRef{}) <= 0
	}
	if mp.AmountTrim == "" || mp.AmountIsLit {
		return false
	}
	ctx := e.manaAmountCtx(p, id)
	effects.SetSVars(ctx, f.SVars)
	n, ok := effects.ManaAmountResolvedStrict(mp, e, ctx, 1)
	if !ok || n > 0 {
		return false
	}
	// A storage land's "{T}, Remove X storage counters: Add X mana" resolves
	// Amount$ X to zero here only because no X is announced: the produced mana
	// is the announced counters, so the source yields when it carries any.
	// That is the kernel's own gate (potentialGapCost's ctrN(...) > 0), so both
	// sides price the source alike; without it a 0-counter land would be
	// offered and tapped for nothing.
	if part, announced := announcedSourceSubCounter(e.abilityCompiledCost(ma).Cost); announced {
		o := e.G.Obj(id)
		return o == nil || o.Counter(part.Spec) <= 0
	}
	return true
}

// announcedSourceSubCounter returns the cost's announced SubCounter<X/Kind>
// part anchored on the source itself -- the storage land's shape. An announced
// part anchored on another permanent is not the source's own production, so it
// names no counter kind here.
func announcedSourceSubCounter(c Cost) (CostPart, bool) {
	for _, part := range c.SubCounter {
		if part.Announced && subCounterTargetsSource(part.Target) {
			return part, true
		}
	}
	return CostPart{}, false
}
