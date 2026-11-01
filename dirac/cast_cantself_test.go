package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCantBeCast: a StatCantBeCast with StRaiseOpp and the your-turn
// condition (Dragonlord Dromoka: your opponents can't cast spells during
// your turn) forbids the opponent's casts during its controller's turn
// only, and never the controller's own.
func TestCantBeCast(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondYourTurn})
	yt := kop.CondID(len(tbl.Conds) - 1)
	lock, spell := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Lock", Types: kop.TCreature, Power: 5, Toughness: 7, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCantBeCast, StFlags: kop.StRaiseOpp, Cond2: yt}}}},
		kop.CardDef{Name: "Test Spell", Types: kop.TInstant})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyCantCast {
		t.Fatal("anyCantCast not set")
	}
	sc.perm(lock, 0)
	sc.s.active = 0
	if !sc.s.castForbidden(sc.k, 1, spell, NoObj) || sc.s.castForbidden(sc.k, 0, spell, NoObj) {
		t.Fatal("during its controller's turn: the opponent forbidden, the controller not")
	}
	sc.s.active = 1
	if sc.s.castForbidden(sc.k, 1, spell, NoObj) {
		t.Fatal("during the opponent's own turn the opponent is forbidden")
	}
}
