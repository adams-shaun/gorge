package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// noPrevScen is the shield table's matchup (Rhox Pummeler enters with a
// SHIELD counter, Shock) with one hand-built end-of-turn emblem carrying
// kop.StatCantPreventDamage appended, as kgen lowers an Effect's "damage
// can't be prevented this turn". It returns the scenario, the card ids and
// the emblem's 1-based Table.Emblems index.
func noPrevScen(t *testing.T) (*scen, map[string]CardID, uint8) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
		{Name: "Rhox Pummeler", Count: 1}, {Name: "Shock", Count: 1}}
	set := cardsettest.Set(t, "shield")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tbl.Emblems = append(tbl.Emblems, kop.Emblem{UntilEOT: true, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCantPreventDamage}}}})
	emb := uint8(len(tbl.Emblems))
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
	return &scen{s: s, k: s.k()}, id, emb
}

// TestCantPreventDamage: while the end-of-turn emblem's StatCantPreventDamage
// applies, Shock at a creature with a SHIELD counter is dealt in full and the
// counter stays; after the cleanup drops the emblem the counter prevents the
// next Shock again and is spent (CR 615.12, 122.1c; gorge cantPreventDamage).
// DebugNoPreventOff (the static ignored) must let the shield prevent.
func TestCantPreventDamage(t *testing.T) {
	run := func(t *testing.T, off bool) {
		DebugNoPreventOff = off
		defer func() { DebugNoPreventOff = false }()
		sc, id, emb := noPrevScen(t)
		if !sc.k.gen.anyNoPrev || sc.k.gen.anyRepl&rbShield == 0 {
			t.Fatal("prep: anyNoPrev / rbShield unset")
		}
		sc.lands(0, id["Mountain"], 4)
		rhox := sc.perm(id["Rhox Pummeler"], 1)
		if n := sc.s.Counters(rhox, kop.CtrShield); n != 1 {
			t.Fatalf("precondition: Rhox Pummeler has %d SHIELD, want 1", n)
		}
		embDurPut(sc, 0, NoObj, emb)
		if !sc.s.noPrevLive(sc.k, false) && !off {
			t.Fatal("precondition: the emblem's static is not in force")
		}
		sc.castAt(t, sc.hand(id["Shock"], 0), rhox)
		n, d := sc.s.Counters(rhox, kop.CtrShield), sc.s.perms[sc.slot(rhox)].damage
		if off {
			if n != 0 || d != 0 {
				t.Fatalf("static ignored: %d SHIELD, %d damage; want the shield to prevent (0, 0)", n, d)
			}
			return
		}
		if n != 1 || d != 2 {
			t.Fatalf("while damage can't be prevented: %d SHIELD, %d damage; want 1, 2", n, d)
		}
		sc.s.cleanupGen(sc.k)
		if sc.s.g.nemb[0] != 0 || sc.s.noPrev(sc.k, false) {
			t.Fatal("the emblem outlived the cleanup")
		}
		sc.s.perms[sc.slot(rhox)].damage = 0
		sc.castAt(t, sc.hand(id["Shock"], 0), rhox)
		if n, d := sc.s.Counters(rhox, kop.CtrShield), sc.s.perms[sc.slot(rhox)].damage; n != 0 || d != 0 {
			t.Fatalf("after the cleanup: %d SHIELD, %d damage; want the shield to prevent (0, 0)", n, d)
		}
	}
	t.Run("static", func(t *testing.T) { run(t, false) })
	t.Run("mutation", func(t *testing.T) { run(t, true) })
}

// TestCantPreventDamagePrepOff: a pool without the static pays nothing.
func TestCantPreventDamagePrepOff(t *testing.T) {
	sc, _ := shieldScen(t, "Rhox Pummeler")
	if sc.k.gen.anyNoPrev || sc.s.noPrev(sc.k, false) {
		t.Fatal("anyNoPrev set on a table without StatCantPreventDamage")
	}
}
