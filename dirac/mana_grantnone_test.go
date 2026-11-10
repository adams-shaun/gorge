package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantNoManaGap: a source of no colour (Command Tower: commander
// identity, nothing without a commander, CR 903.4f) before a creature holding
// Cryptolith Rite's granted mana ability is a census gap of gorge's, so the
// prefix search taps it first, adding nothing, then the creature; the applied
// prefix leaves the Tower tapped and only the creature's mana in the pool.
func TestGrantNoManaGap(t *testing.T) {
	set := cardsettest.Set(t, "grantnomana")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = CardID(c)
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	s, k := sc.s, sc.k
	sc.perm(id["Cryptolith Rite"], 0)
	tower := sc.slot(sc.perm(id["Command Tower"], 0))
	bear := sc.slot(sc.perm(id["Grizzly Bears"], 0))
	if !k.gen.anyManaNone || !k.gen.has(s.cardAt(tower), gfManaNone) {
		t.Fatal("Command Tower is not flagged a no-colour mana source")
	}
	if s.grantIdx(k, bear) < 0 {
		t.Fatal("Grizzly Bears holds no granted mana ability")
	}
	if s.canTapForMana(k, tower) {
		t.Fatal("the Tower is a planner source: the gap is not what the test reads")
	}
	cost := kop.ManaCost{}
	cost.Pips[kop.R] = 1
	d := payDesc{class: pdSpell, card: id["Shock"], src: NoSlot, og: restrOrigin{from: kop.ZoneHand, cast: true, byYou: true}}
	var g grantSrch
	if !s.grantSearch(k, 0, &cost, NoSlot, d, [6]bool{}, &g) {
		t.Fatal("the grant search finds no prefix for {R}")
	}
	if g.np != 2 || g.path[0].gi != stepManaNone || g.path[0].sl != tower || g.path[0].n != 0 || g.path[1].sl != bear {
		t.Fatalf("path = %+v (np %d); want the Tower (stepManaNone, n 0), then the creature", g.path[:g.np], g.np)
	}
	s.grantApply(k, &g)
	if s.perms[tower].flags&pfTapped == 0 || s.perms[bear].flags&pfTapped == 0 {
		t.Errorf("tapped: tower %v bear %v; want both",
			s.perms[tower].flags&pfTapped != 0, s.perms[bear].flags&pfTapped != 0)
	}
	if got := poolTotal(&s.pl[0]); got != 1 || s.pl[0].pool[kop.R] != 1 {
		t.Errorf("pool = %v; want exactly the creature's {R}", s.pl[0].pool)
	}
}
