package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// potentialPricer is the slice of *Engine the potential walk's per-ability
// reads need: the priced cost of a mana ability and its folded production.
type potentialPricer interface {
	manaAbilityCC(p state.PlayerID, source state.ObjID, ma *cards.SA) *compiledCost
	addPotentialManaOf(m *state.Mana, ma *cards.SA)
}

// repeatableFeeOnlyMana reports a mana ability whose whole cost is mana (Bog
// Initiate's "{1}: Add {B}.", Orochi Leafcaller's "{G}: ...", Agent of
// Stromgald's "{R}: Add {B}."): no {T}, no sacrifice and no activation limit,
// so a payment activates it as often as it can fund the fee (CR 605.3a: mana
// abilities are activated while paying, and nothing here is used up).
func repeatableFeeOnlyMana(pp potentialPricer, p state.PlayerID, id state.ObjID, ma *cards.SA) bool {
	cost := pp.manaAbilityCC(p, id, ma).Cost
	if cost.Generic+cost.Colored.Total() <= 0 {
		return false
	}
	want := costvocab.Cost{Generic: cost.Generic, Colored: cost.Colored}
	if !cost.Equal(&want) {
		return false
	}
	_, limited := ma.Param(cards.PKActivationLimit)
	_, gameLimited := ma.Param(cards.PKGameActivationLimit)
	return !limited && !gameLimited
}

// addPotentialSacRepeat repeats a sacrifice-cost mana ability's production in
// the bound once per extra sacrifice candidate. An ability whose whole cost
// is sacrificing another permanent ("Sacrifice a token named Wood: Add {R}",
// Jungle Patrol; CR 605.1a) has no {T}, no mana fee and no other part, so the
// payer activates it once per distinct matching permanent it controls -- each
// activation converts one candidate into the production -- where the
// fixpoint's own admission counts the production only once (each source's
// ability joins the pool a single time, potential.go). Three Wood tokens
// beside three lands therefore fund three red mana, and the bound must say
// so or it refuses a cast the seat can pay. Zero extra folds for any other
// shape: a self-sacrifice spends the source (one activation, the kernel's
// last-resort tier), a {T} or mana-fee cost prices once through that same
// admission, and an announced part is priced by its own ask.
func addPotentialSacRepeat(pp potentialPricer, pe pay.Engine, out *state.Mana, p state.PlayerID, id state.ObjID, ma *cards.SA) {
	cost := pp.manaAbilityCC(p, id, ma).Cost
	if len(cost.Sac) != 1 || cost.Sac[0].N != 1 || cost.Sac[0].Referent != 0 || cost.Sac[0].Announced ||
		cost.Tap || cost.Life != 0 || len(cost.Return) != 0 || !pay.PlanLastResortCostOK(cost) ||
		pay.PlanSelfCost(cost.Sac[0], id) {
		return
	}
	// Token fodder only (a ".token" spec term): the kernel's plain-fold repeat
	// is gated on FToken (dirac/legal.go sacAnyRepeatUnits), and a fixed
	// production beside non-token fodder (Krark-Clan Ironworks' "Sacrifice an
	// artifact: Add {C}{C}") counts once in both bounds.
	if !cost.Sac[0].SpecNamesToken() {
		return
	}
	// The kernel's census arm (dirac legal.go sacAnyRepeatUnits) prices the
	// repeats of exactly this shape: a fixed, unrestricted production of one
	// colour of token fodder (its own mask and count; an any-colour or indeterminate one is
	// already open in this fold, producedOpen). A wider shape takes no
	// repeats here, matching the kernel's bound.
	mp := effects.ManaOf(ma)
	if mp.RestrictValid != "" {
		return
	}
	if _, indeterminate := potentialAmount(mp); indeterminate {
		return
	}
	faces := potentialProducedStrip.Replace(mp.Produced)
	if faces == "" {
		faces = "C"
	}
	seen := byte(0)
	for _, r := range faces {
		switch r {
		case 'W', 'U', 'B', 'R', 'G', 'C':
		default:
			return // an open or multi-word production: the fold is already unbounded
		}
		if seen == 0 {
			seen = byte(r)
		} else if seen != byte(r) {
			return // several fixed colours: no census arm prices the repeats
		}
	}
	if seen == 0 {
		return
	}
	n := len(pay.SacrificeCostCandidates(pe, p, id, cost.Sac[0], true))
	for range max(n-1, 0) {
		pp.addPotentialManaOf(out, ma)
	}
}

// addPotentialRepeatable lifts to potentialUnbounded each colour a
// repeatable fee-only ability makes: the bound counts a source's production
// once, but the fee converts mana into the ability's colour once per
// activation, so Bog Initiate beside two other lands funds {B}{B}.
func addPotentialRepeatable(pp potentialPricer, out *state.Mana, p state.PlayerID, id state.ObjID, ma *cards.SA) {
	if !repeatableFeeOnlyMana(pp, p, id, ma) {
		return
	}
	var made state.Mana
	pp.addPotentialManaOf(&made, ma)
	for c, n := range made {
		if n > 0 {
			out[c] = potentialUnbounded
		}
	}
}
