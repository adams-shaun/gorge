package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A resolving AddMana with a spend restriction (CR 106.6): a loyalty
// ability is no mana ability (CR 605.1a), so its mana is added on
// resolution, and gorge's ManaAdd keeps the restriction on it
// (Player.RestrictedMana). Chandra, Chill of Compliance's +1 adds a {U}
// spendable only on a noncreature spell. The units float in State.rb, the
// restricted-mana byte the payment and offer machinery already reads
// (mana_window.go restrSpendable / restrAvail / payRestr).

// DebugPlainLoyaltyMana: a test-only mutation that adds a restricted
// AddMana's units to the plain pool (the pre-fix behavior).
var DebugPlainLoyaltyMana bool

// addManaRestr floats n units of colour col for p, spendable only as
// filter f allows. n <= 0 adds nothing; a pool whose batches cannot hold
// them (State.rb: every batch taken, or past 255) stalls loud,
// never a silent spill.
//
//go:noinline
func (s *State) addManaRestr(k *prep, p uint8, col, n int, f kop.FilterID) {
	if n <= 0 {
		return
	}
	if DebugPlainLoyaltyMana {
		s.pl[p].pool[col] += uint8(n)
		return
	}
	if k.artMana && k.cards[s.res.e.card].manaTyped {
		// gorge tags a resolving DB$ Mana by its source (effects.ManaProducerTag):
		// an artifact's units are typed, spent after the plain ones (rbTyped), as
		// floatRestrV records for a mana ability
		s.g.poolArt[p&1] |= poolArtRestr
	}
	if k.gen == nil || n > 255 || !s.addRestr(k, p, f, col, uint8(n)) {
		s.capFull(StallRestrSrc)
	}
}

// prepAddManaRestr registers the restriction of every AddMana in a's
// program (genPrep.restrF), so addRestr can name it. Cold, once per table.
//
//go:noinline
func (g *genPrep) prepAddManaRestr(a *kop.Ability) {
	for i := range a.Prog {
		if in := &a.Prog[i]; in.Op == kop.OpAddMana && in.F != 0 {
			g.addRestrF(in.F)
		}
	}
	g.prepDBManaRestr(a) // mana_resolverestriction.go
}

// A resolving DB$ Mana with RestrictValid$ Spell.<spec>
// (Abstract Paintmage's "{U}{R}, instant or sorcery only", Desolation of
// Smaug's four restricted mana of any colours; CR 106.6, 605.1a). A fixed colour is OpAddMana F, a
// colour choice is ExtManaChoice F; both float the units as restricted mana
// (State.rb) through addManaRestr, one restriction for every unit
// (gorge effMana's ManaAdd restriction, askManaChoice picking first).

// DebugActOnlyRestrSpell: a test-only mutation that lets a spell spend the
// activated-only marker batch (restrSpendableB).
var DebugActOnlyRestrSpell bool

// DebugDBManaRestrPlain: a test-only mutation that adds a resolving colour
// choice's restricted units to the plain pool (the pre-fix behavior).
var DebugDBManaRestrPlain bool

// manaChoiceAdd adds n units of the picked colour col for p: restricted as
// filter f allows when f != 0, plain otherwise. Cold, once per pick.
//
//go:noinline
func (s *State) manaChoiceAdd(k *prep, p uint8, col int, n uint8, f kop.FilterID) {
	if DebugDBManaRestrPlain {
		s.pl[p].pool[col] += n
		return
	}
	s.addManaRestr(k, p, col, int(n), f) // mana_resolverestriction.go
}

// prepDBManaRestr registers the restriction of a's colour choices
// (ExtManaChoice F). Cold, once per table.
//
//go:noinline
func (g *genPrep) prepDBManaRestr(a *kop.Ability) {
	for i := range a.Prog {
		in := &a.Prog[i]
		if in.F != 0 && in.Op == kop.OpExt && in.A.K == kop.VConst && in.A.N == int16(kop.ExtManaChoice) {
			g.addRestrF(in.F)
		}
	}
}
