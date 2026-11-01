package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// paid_mana_census.go is the ONE paid-cost layer over the shared payment
// census (windowManaUnits). The census is ON by default; Config.
// DisablePaidManaCensus restores the pre-flip narrow census. It widens what
// the attack-tax budget and tap list, the unless-cost windows, AvailableMana
// and the pool-short ability offer (legal_walk_autopay.go) read: a mana
// source whose activation cost is more than a bare tap is counted when the
// engine can pay that cost with no choice and no sub-ask -- a
// self-sacrifice (Sac<1/CARDNAME>: Treasure, Lotus Petal, Black Lotus) or a
// fixed PayLife<N> that leaves the payer alive.
//
// Every reader asks paidCostShape, and the cast window's own paid layer
// (castWindowProbeUnits) asks it too, so the windows cannot disagree about
// which sources are payable. Still excluded (fail closed): a Sac of another
// permanent, discard, exile, counter removal, tapping other permanents, a
// generic/coloured mana fee (a cast window prices that one itself), an
// announced or dynamic amount, RestrictValid$ and InstantSpeed$ abilities,
// and a production needing the source's as-enters colour.

// paidCostShape reports whether cost is a paid activation cost the census
// can pay deterministically right now by source id of payer p, and the life
// it spends. life is the payer's life still unspent: a PayLife<N> that would
// take it to 0 or below is not payable (CR 119.4 permits it, but the payer
// would lose; no census counts a source that kills its payer). A
// self-sacrifice requires the source to be the only match of its CARDNAME
// spec and its sacrifice to be unblocked (pay.CastWindowSelfSacCost).
func (e *Engine) paidCostShape(p state.PlayerID, id state.ObjID, cost Cost, life int32) (int32, bool) {
	switch {
	case castWindowPayLifeCost(cost):
		if life <= cost.Life {
			return 0, false
		}
		return cost.Life, true
	case pay.CastWindowSelfSacCost(asPayer(e), p, id, cost):
		return 0, true
	}
	return 0, false
}

// paidManaAbility classifies one of a source's window mana abilities for the
// paid-cost census: ok when ma is a non-free ability with a literal positive
// amount, no RestrictValid$, an available tap part and a paidCostShape cost;
// life is what it spends. lifeLeft is the payer's life not yet committed to
// other sources of this census.
func (e *Engine) paidManaAbility(p state.PlayerID, id state.ObjID, o *state.Object, ma *cards.SA, lifeLeft int32) (life int32, ok bool) {
	mf := e.manaStaticOf(ma)
	if mf.RestrictValid || mf.FreeCost || mf.Amount <= 0 {
		return 0, false
	}
	if (mf.Tap && o.Tapped) || (mf.Untap && !o.Tapped) {
		return 0, false
	}
	if producedNeedsChosen(mf.Produced) {
		return 0, false
	}
	total := int32(0)
	for _, n := range mf.Counts {
		total += n
	}
	if total <= 0 {
		return 0, false
	}
	cost := e.abilityCompiledCost(ma).Cost
	if pay.ActivationTapCostUnavailable(o, &cost) {
		return 0, false
	}
	return e.paidCostShape(p, id, cost, lifeLeft)
}

// windowManaUnitsPaid is the census the attack-tax and unless-cost readers
// share: windowManaUnits, plus -- unless Config.DisablePaidManaCensus
// -- the paid-cost layer. With the census disabled it IS windowManaUnits.
func (e *Engine) windowManaUnitsPaid(p state.PlayerID) []windowManaUnit {
	units := e.windowManaUnits(p)
	if !e.paidManaCensus {
		return units
	}
	return e.appendPaidManaUnits(p, units)
}

// appendPaidManaUnits adds the paid-cost alternatives to units, walking p's
// battlefield in zone order (deterministic). A source taps once, so its
// life is the MAX over its alternatives; the life committed by earlier
// sources is withheld from later ones so every counted source can be paid
// together and the payer still survives.
func (e *Engine) appendPaidManaUnits(p state.PlayerID, units []windowManaUnit) []windowManaUnit {
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	committed := int32(0)
	life := e.G.Players[p].Life
	ids := pay.AppendBattlefieldManaSourceIDs(e.G, e.idsBorrow(), p)
	defer e.idsRelease(ids)
	for _, id := range ids {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		maxLife := int32(0)
		for _, ma := range e.availableManaAbilitiesForWindow(p, id, false) {
			lc, ok := e.paidManaAbility(p, id, o, ma, life-committed)
			if !ok {
				continue
			}
			for _, alt := range e.paidManaAlts(id, ma, lc) {
				units = pay.AppendUnitAlt(units, id, alt)
				if lc > maxLife {
					maxLife = lc
				}
			}
		}
		committed += maxLife
	}
	return units
}

// paidManaAlts expands one classified ability into its census alternatives:
// a plain production is one alt; a choice-shaped production is one alt per
// producible colour whose ability is pinned to that colour (the attack
// window's own pin, so the activation poses no colour sub-ask). An ability
// carrying an activation limit is not pinned (the limit's pile identity
// would be lost on the copy) and is left out.
func (e *Engine) paidManaAlts(id state.ObjID, ma *cards.SA, life int32) []windowManaAlt {
	mf := e.manaStaticOf(ma)
	alt := windowManaAlt{Ma: ma, Counts: mf.Counts, Amt: mf.Amount, Life: life, LifeKeep: life > 0}
	if !mf.Any {
		return []windowManaAlt{alt}
	}
	if mp := effects.ManaOf(ma); mp.Produced == "" {
		// The executor's deterministic one-colourless default.
		return []windowManaAlt{alt}
	}
	return pinnedManaAlts(alt, mf.Counts)
}

// pinnedManaAlts splits a choice-shaped production over the colours in counts
// into one alt per colour, each pinned so that its activation poses no colour
// ask. An ability carrying an activation limit is left out.
func pinnedManaAlts(alt windowManaAlt, counts [6]int32) []windowManaAlt {
	ma := alt.Ma
	if _, limited := ma.Param(cards.PKActivationLimit); limited || ma.ParamStr(cards.PKGameActivationLimit) != "" ||
		pay.ChainGatesOnActivationCount(ma) {
		return nil
	}
	var out []windowManaAlt
	for c, n := range counts {
		if n == 0 {
			continue
		}
		var single [6]int32
		single[c] = 1
		a := alt
		a.Counts = single
		a.Ma = pay.WithProduced(ma, ma, string(cards.ManaSymbol(c)))
		out = append(out, a)
	}
	return out
}

// unlessWindowUnits is the census the unless-cost windows read:
// windowManaUnitsPaid plus the free abilities whose colour is chosen on
// activation (CR 605.3a), which that census withholds. They arrive pinned, one
// alt per colour: the shape the reach search prices, and a tap in the
// component window that poses no colour ask.
func (e *Engine) unlessWindowUnits(p state.PlayerID) []windowManaUnit {
	units := e.windowManaUnitsPaid(p)
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	ids := pay.AppendBattlefieldManaSourceIDs(e.G, e.idsBorrow(), p)
	defer e.idsRelease(ids)
	for _, id := range ids {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		for _, ma := range e.availableManaAbilitiesForWindow(p, id, false) {
			mf := e.manaStaticOf(ma)
			if !mf.Any || !mf.FreeCost || mf.RestrictValid || mf.Amount <= 0 ||
				(mf.Tap && o.Tapped) || (mf.Untap && !o.Tapped) {
				continue
			}
			mp := effects.ManaOf(ma)
			counts := mf.Counts
			if mp.NeedsChosen {
				chosen := pay.ChosenProducedColour(e.G, id)
				if chosen == "" {
					continue
				}
				counts, _ = cards.ProducedCounts(pay.SubstituteChosenProduced(mp.Produced, chosen))
			}
			// A lone colourless slot is the executor's default, which the
			// shared census already counts.
			if counts == ([6]int32{5: 1}) {
				continue
			}
			for _, a := range pinnedManaAlts(windowManaAlt{Ma: ma, Amt: mf.Amount}, counts) {
				units = pay.AppendUnitAlt(units, id, a)
			}
		}
	}
	return units
}

// availablePaidAbilities is AvailableMana's paid-cost extension: the
// abilities of source id (already in free's reading order) the paid-cost
// census would also count, with the life they commit.
func (e *Engine) availablePaidAbilities(p state.PlayerID, id state.ObjID, o *state.Object, lifeLeft int32) ([]*cards.SA, int32) {
	var out []*cards.SA
	maxLife := int32(0)
	for _, ma := range e.availableManaAbilities(p, id) {
		lc, ok := e.paidManaAbility(p, id, o, ma, lifeLeft)
		if !ok {
			continue
		}
		out = append(out, ma)
		if lc > maxLife {
			maxLife = lc
		}
	}
	return out, maxLife
}
