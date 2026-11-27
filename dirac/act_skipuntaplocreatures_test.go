package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSkipUntapPlayerCreatures: OpSkipUntap C=SkipUntapPlayerCreatures keeps
// the player Obj's creatures tapped through that player's next untap step and
// leaves their lands alone (kgen's Blinding Beam lowering: an Event$ Untap |
// Layer$ CantHappen | ValidCard$ Creature | ValidStepTurnToController$
// Player.IsRemembered replacement). The mark ends at that untap step.
func TestSkipUntapPlayerCreatures(t *testing.T) {
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 18}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	lc, pc := CardID(255), CardID(255)
	for i := range tbl.Cards {
		switch tbl.Cards[i].Name {
		case "Savannah Lions":
			lc = CardID(i)
		case "Plains":
			pc = CardID(i)
		}
	}
	in := kop.Instr{Op: kop.OpSkipUntap, C: kop.SkipUntapPlayerCreatures, Obj: kop.RefOpponent}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Creature Freeze", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{in}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anySkipUntap {
		t.Fatalf("anySkipUntap %v, want true", sc.k.gen.anySkipUntap)
	}
	s := sc.s
	lion, land := sc.perm(lc, 1), sc.perm(pc, 1)
	s.perms[sc.slot(lion)].flags |= pfTapped
	s.perms[sc.slot(land)].flags |= pfTapped
	s.res.e.ctrl = 0 // the resolving spell is seat 0's; RefOpponent is seat 1
	s.exec(sc.k, &in)
	s.active, s.step = 1, kop.StepUntap
	s.beginStep(sc.k)
	if !sc.tapped(lion) {
		t.Fatal("the opponent's creature untapped during the restricted untap step")
	}
	if sc.tapped(land) {
		t.Fatal("the opponent's land stayed tapped: the creatures-only mark reached lands")
	}
	s.step = kop.StepUntap
	s.beginStep(sc.k)
	if sc.tapped(lion) {
		t.Fatal("the next untap step did not untap the creature")
	}
}
