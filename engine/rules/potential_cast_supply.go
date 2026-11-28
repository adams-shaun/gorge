package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// castSupplyHost is the narrow slice of the engine castOutrunsSupply reads:
// the supply proof's own reads plus the plain cast offer gate's cost
// composition (castOfferBase, the cost statics, the optional colour
// reductions and the cost-credit keywords).
type castSupplyHost interface {
	supplyProofHost
	castOfferBase(p state.PlayerID, id state.ObjID) Cost
	collectCostStatics() costStaticViews
	offerCostForUsing(statics costStaticViews, p state.PlayerID, id state.ObjID, base Cost, scope costScope) Cost
	reduceColorOptionalViews(statics costStaticViews, p state.PlayerID, id state.ObjID) []reduceColorView
	hasCastConvoke(id state.ObjID) bool
	hasCastImprovise(id state.ObjID) bool
	hasCastDelve(id state.ObjID, ability bool) bool
}

// castOutrunsSupply is abilityOutrunsSupply for a plain cast (CR 601.2f):
// the composed cost the hand walk's plain cast gate charges (the printed
// cost with the spell ability's extras, plus the commander tax, raised and
// reduced by the cost statics) needs more mana than supply, the floating
// pool plus one mana for each source. A census-gap prefix taps each source
// at most once (CR 107.5: a tapped permanent can't pay {T} again), so no
// prefix can float that much and the search could only spend its budget
// (Ancient Stone Idol's {10} under Manabarbs, where every land is a gap:
// three searches of potentialScriptNodes clones per priority decision). A
// cost whose mana count can fall after announcement or past the gate's base
// -- an {X}, a Phyrexian, twobrid or waterbend part, a convoke, improvise or
// delve credit, an either-or additional cost, an optional life-for-pip
// reduction -- is never proven. Like abilityOutrunsSupply, the proof assumes
// the current board: a prefix that lowers the cost (a {T}, Sacrifice source
// feeding a per-graveyard-card reduction) or a stack the search resolves that
// makes more mana is outside it.
func castOutrunsSupply(e castSupplyHost, p state.PlayerID, o decision.Option, supply int32) bool {
	if o.Kind != optCast || o.Mode != "" || o.AltCostIndex != 0 || o.Keyword != "" {
		return false
	}
	ob := e.Game().Obj(o.Obj)
	if ob == nil || ob.Face() == nil {
		return false
	}
	f := ob.Face()
	if e.hasCastConvoke(o.Obj) || e.hasCastImprovise(o.Obj) || e.hasCastDelve(o.Obj, false) || len(altAddCostParts(f)) != 0 {
		return false
	}
	statics := e.collectCostStatics()
	if len(e.reduceColorOptionalViews(statics, p, o.Obj)) != 0 {
		return false
	}
	c := e.offerCostForUsing(statics, p, o.Obj, pay.WithSpellAbilityExtras(f, e.castOfferBase(p, o.Obj)), spellScope(""))
	if c.X != 0 || c.XMin != 0 || c.Waterbend != 0 || c.WaterbendX || len(c.Phyrexian)+len(c.HybridPhyrexian)+len(c.Twobrid) != 0 {
		return false
	}
	return supply < c.CMC()
}

// censusTapSupply is the most mana p can have floating after any prefix of
// the census-gap search: the floating pool (restricted batches included)
// plus, for each untapped source, the most any one of its mana abilities
// nets -- the mana it makes and the deterministic tap extra, less its mana
// fee. Every such ability must cost the source's own {T}, so the search
// activates each source at most once (pathPaidTap: CR 107.5, 605.3a). A
// tapped source's {T} ability adds nothing (CR 107.5: it can't pay the tap),
// while a non-{T} mana ability stays activatable through the tap and may be
// repeatable (a Prism token's sacrifice), so one claims no bound. ok is
// false when a {T} ability's amount is not a literal (a "for each" count, an
// {X} fee), when a card off the battlefield has a mana ability p could
// activate, or when its production is a shape tapMadeAtMost does not know
// (a Special production).
func censusTapSupply(e castSupplyHost, p state.PlayerID) (supply int32, ok bool) {
	g := e.Game()
	pl := &g.Players[p]
	supply = int32(pl.Pool.Total())
	for _, r := range pl.RestrictedMana {
		supply += max(r.Amount, 0)
	}
	// A mana ability off the battlefield (Simian Spirit Guide's, from the
	// hand) is a priority activation the search may take too: no bound.
	for _, z := range [...]state.Zone{state.ZHand, state.ZGraveyard, state.ZExile, state.ZCommand} {
		for _, id := range g.Zone(z, p) {
			if len(e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true)) != 0 {
				return 0, false
			}
		}
	}
	for _, id := range g.Zone(state.ZBattlefield, p) {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		best := int32(0)
		for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true) {
			mp := effects.ManaOf(ma)
			cost := e.manaAbilityCC(p, id, ma).Cost
			if !cost.Tap {
				// A non-{T} mana ability (Diamond Kaleidoscope's sacrifice
				// of a Prism token, Ashnod's Altar's) can be paid while the
				// source is tapped and, unless it is once-only, more than
				// once: the one-best-per-source count cannot bound it.
				return 0, false
			}
			if o.Tapped {
				// CR 107.5: a tapped permanent can't pay its own {T}, so
				// this activation contributes nothing. Its non-{T}
				// abilities were refused above.
				continue
			}
			if cost.X != 0 || cost.XMin != 0 || mp.Each.Present || !mp.AmountIsLit && mp.AmountTrim != "" || condSplitLiveSub(e, p, id, ma) != nil {
				return 0, false
			}
			made, known := tapMadeAtMost(ma, mp)
			if !known {
				return 0, false
			}
			made += int32(e.potentialTapExtra(p, id, ma).Total())
			best = max(best, made-cost.CMC())
		}
		supply += best
	}
	return supply, true
}

// tapMadeAtMost is the most mana one activation of ma makes: a fixed symbol
// string's symbols times its literal amount, or the literal amount of a
// choice of colours (Any, a chosen colour, the commander's colour identity,
// a Combo of listed symbols) or of a reflected ability with no amount of its
// own (Exotic Orchard: one mana). known is false for any other production --
// a Special word (Bloom Tender's one mana per colour among its permanents)
// makes a per-board number of mana.
func tapMadeAtMost(ma *cards.SA, mp *effects.ManaParams) (made int32, known bool) {
	if ma.APIKind() == cards.APIManaReflected {
		return 1, !effects.ManaReflectedOf(ma).Amount.Present
	}
	if mp.AmountTrim != "" && !mp.AmountIsLit {
		return 0, false
	}
	amt := availableAmountOf(mp)
	if !mp.CountsAny {
		fixed := int32(0)
		for _, n := range mp.Counts {
			fixed += n * amt
		}
		return fixed, true
	}
	switch raw := mp.ProducedRaw; raw {
	case "", "Any", "Combo Any", "Chosen", "ComboChosen", "Combo Chosen", "Combo ColorIdentity":
		return amt, true
	default:
		const combo = "Combo "
		if len(raw) <= len(combo) || raw[:len(combo)] != combo {
			return 0, false
		}
		if _, other := cards.ProducedCounts(raw[len(combo):]); other {
			return 0, false
		}
		return amt, true
	}
}
