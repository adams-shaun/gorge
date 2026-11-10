package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCantPreventIsCombat: Frenzied Baloth's "combat damage can't be
// prevented" (the StatCantPreventDamage static kgen lowers with
// StNoPrevCombat; kgen TestCantPreventIsCombat) printed on Rhox Pummeler,
// which enters with a SHIELD counter. A Grizzly Bears it blocks deals its 2
// combat damage in full and the counter stays; a Shock at it afterwards (its
// controller casts it in main 2) is
// noncombat damage, so the counter prevents it and is spent (CR 615.12,
// 122.1c; gorge cantPreventDamage's IsCombat$ True against combatDamaging).
func TestCantPreventIsCombat(t *testing.T) {
	deck := []cardset.Entry{{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
		{Name: "Rhox Pummeler", Count: 1}, {Name: "Grizzly Bears", Count: 1}, {Name: "Shock", Count: 1}}
	set := cardsettest.Set(t, "shield")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	rc := &tbl.Cards[id["Rhox Pummeler"]]
	rc.Abilities = append(append([]kop.Ability(nil), rc.Abilities...), kop.Ability{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatCantPreventDamage, StFlags: kop.StNoPrevCombat}})
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Mountain"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	sc := &scen{s: s, k: s.k()}
	if !sc.k.gen.anyNoPrev || sc.k.gen.anyRepl&rbShield == 0 {
		t.Fatal("prep: anyNoPrev / rbShield unset")
	}
	sc.lands(1, id["Mountain"], 4)
	rhox := sc.perm(id["Rhox Pummeler"], 1)
	bear := sc.perm(id["Grizzly Bears"], 0)
	if n := s.Counters(rhox, kop.CtrShield); n != 1 {
		t.Fatalf("precondition: Rhox Pummeler has %d SHIELD, want 1", n)
	}
	if !s.noPrevLive(sc.k, true) || s.noPrevLive(sc.k, false) {
		t.Fatal("precondition: the static must be in force for combat damage only")
	}
	sc.combat(t, []Obj{bear}, [][2]Obj{{rhox, bear}})
	if n, d := s.Counters(rhox, kop.CtrShield), s.perms[sc.slot(rhox)].damage; n != 1 || d != 2 {
		t.Fatalf("combat damage: %d SHIELD, %d damage; want 1, 2 (not prevented)", n, d)
	}
	if s.step != kop.StepMain2 || s.prio != 1 {
		t.Fatalf("after combat: step %d priority %d, want main 2 with player 1 to act", s.step, s.prio)
	}
	sc.castAt(t, sc.hand(id["Shock"], 1), rhox) // Rhox's controller Shocks it: noncombat damage
	if !sc.onBF(rhox) {
		t.Fatal("Shock killed Rhox Pummeler: the noncombat damage was not prevented")
	}
	if n, d := s.Counters(rhox, kop.CtrShield), s.perms[sc.slot(rhox)].damage; n != 0 || d != 2 {
		t.Fatalf("Shock: %d SHIELD, %d damage; want 0, 2 (the shield prevents it)", n, d)
	}
}
