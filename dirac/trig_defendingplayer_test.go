package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAttackPlayerOnce: a Tf2DefPlayer AttackersDeclared line (gorge
// AttackedTarget$ Opponent) fires once for the player-defender group, its
// amount the group's matching attackers.
func TestAttackPlayerOnce(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	crea := kop.FilterID(len(tbl.Filters) - 1)
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.ValueRef{K: kop.VTriggerAmount}}}
	w, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Herald", Types: kop.TEnchantment, Abilities: []kop.Ability{
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigAttackersDeclared, Filter: crea, Flags2: kop.Tf2DefPlayer | kop.Tf2AtkOpp}, Prog: gain}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 1, Toughness: 4})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	sc.perm(w, 0)
	a1, a2 := sc.perm(bear, 0), sc.perm(bear, 0)
	life := s.pl[0].life
	sc.combat(t, []Obj{a1, a2}, nil)
	if got := s.pl[0].life - life; got != 2 {
		t.Fatalf("gained %d, want 2 (one trigger, two attackers)", got)
	}
}
