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
// (strictly) to zero or less.
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
	return ok && n <= 0
}
