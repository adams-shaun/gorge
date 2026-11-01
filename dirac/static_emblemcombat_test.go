package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEOTEmblemCombatStatic: an until-end-of-turn Effect's CantBlockBy /
// MustAttack static (an UntilEOT emblem; gorge effEffect registers both
// modes as a continuous effect of the Effect's controller) applies with
// no static permanent on the battlefield, relative to the emblem's owner,
// and the cleanup drops it.
func TestEOTEmblemCombatStatic(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	yours := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlOpp})
	theirs := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Emblems = append(tbl.Emblems,
		kop.Emblem{UntilEOT: true, Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCantBlockBy, Filter: yours}}}},
		kop.Emblem{UntilEOT: true, Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatMustAttack, Filter: theirs}}}})
	e1 := uint8(len(tbl.Emblems) - 1) // emb holds the index + 1
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.embComb {
		t.Fatal("embComb not prepared")
	}
	a, b := sc.perm(bear, 0), sc.perm(bear, 1)
	s, k := sc.s, sc.k
	if !s.canBlock(k, sc.slot(b), sc.slot(a)) || s.MustAttack(b) {
		t.Fatal("no emblem: the bear blocks and need not attack")
	}
	s.g.emb[0][0], s.g.emb[0][1], s.g.nemb[0] = e1, e1+1, 2 // both emblems owned by player 0
	if s.canBlock(k, sc.slot(b), sc.slot(a)) {
		t.Fatal("CantBlockBy emblem: player 0's creature can't be blocked")
	}
	if s.canBlock(k, sc.slot(a), sc.slot(b)) == false {
		t.Fatal("CantBlockBy emblem must not cover the opponent's attacker")
	}
	if !s.MustAttack(b) || s.MustAttack(a) {
		t.Fatal("MustAttack emblem: the opponent's creature (only) must attack")
	}
	s.cleanupGen(k)
	if s.g.nemb[0] != 0 || !s.canBlock(k, sc.slot(b), sc.slot(a)) || s.MustAttack(b) {
		t.Fatal("the cleanup drops both emblems")
	}
}
