package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// dhReplScen: seat 0 holds a Lion's Eye Diamond shape (kop.DiscardHandManaAb,
// its whole hand discarded, then it is sacrificed for three mana of one
// colour) as an artifact creature, two exile-instead replacements over
// creatures (a Samurai of the Pale Curtain shape, kop.ReplExileOnLeave), and a
// three-{R} activation (the play). It returns the scenario, the play's
// activation action, and the Diamond's slot.
func dhReplScen(t *testing.T) (*scen, Action, Slot) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneBattlefield})
	creatures := kop.FilterID(len(tbl.Filters) - 1)
	curtain := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Pale Curtain", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplExileOnLeave, Filter: creatures, C: 1}},
	}})
	diamond := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Diamond", Types: kop.TArtifact | kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostSacSelf, DiscardHand: true}, ManaColors: kop.ColAny, ManaN: 3},
	}})
	cannon := CardID(len(tbl.Cards))
	cannonCost := kop.ManaCost{}
	cannonCost.Pips[kop.R] = 4
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Cannon", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbActivated, Cost: kop.Cost{Mana: cannonCost}, Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}},
	}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.perm(curtain, 0)
	sc.perm(curtain, 0)
	d := sc.perm(diamond, 0)
	c := sc.perm(cannon, 0)
	a := MakeAction(AkActivate, TCode(c), 0, 0)
	return sc, a, sc.slot(d)
}

// TestDhScriptStallPropagates: the Diamond search's only payment line is the
// Diamond's activation — the hand discarded, then the Diamond sacrificed —
// and the sacrifice dies under two competing exile-instead replacements, which
// latches StallReplLeave in the fork (dieZone). The play ({R}{R}{R}{R}) is
// past every line (the Diamond makes three), so the search fails and dhScript
// declares
// the latched stall on the real state instead of dropping the play as "no
// payment": the game draws as the unmodelled stall instead of silently losing
// the activation gorge offers (TestTapXScriptStallPropagates's mechanism).
func TestDhScriptStallPropagates(t *testing.T) {
	sc, a, diamond := dhReplScen(t)
	s, k := sc.s, sc.k
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	if !s.dhGaps(k, 0).has(diamond) {
		t.Fatal("precondition: the artifact creature is no Diamond gap")
	}
	var d Decision
	s.restrOptions(k, &d, 0)
	if hasOption(&d, a) {
		t.Fatal("precondition: the printed census pays the play, want it past every line")
	}
	if s.dhScript(k, a, 0, false) {
		t.Fatal("the search paid the play, want its only line to stall")
	}
	if !s.Stalled() || s.StallReason() != StallReplLeave {
		t.Fatalf("the search dropped the play silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallReplLeave))
	}
}
