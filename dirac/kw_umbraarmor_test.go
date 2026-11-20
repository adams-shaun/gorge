package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func umbraScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "umbra")
	for _, n := range []string{"Snake Umbra", "Bear Umbra", "Wrath of God", "Murder", "Lightning Bolt", "Typhoid Rats", "Hill Giant", "Gray Ogre", "Grizzly Bears"} {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the umbra table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
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
	return &scen{s: s, k: s.k()}, id
}

// wear puts the Aura card on the battlefield attached to host.
func (sc *scen) wear(aura CardID, host Obj) Obj {
	a := sc.perm(aura, 0)
	sc.s.attachTo(sc.k, sc.slot(a), sc.slot(host))
	sc.s.refreshTy(sc.k)
	return a
}

func (sc *scen) inGrave(o Obj) bool { return sc.s.objs[o].zone == kop.ZoneGraveyard }

// TestUmbraArmorReplacesDestroy: a destroy effect, a sweeper, lethal damage
// and a deathtouch mark each take the umbra Aura instead of the enchanted
// creature, which keeps no damage (CR 702.89a, gorge ReplaceUmbraArmor); the
// next destruction, with no umbra left, is ordinary.
func TestUmbraArmorReplacesDestroy(t *testing.T) {
	sc, id := umbraScen(t)
	if !sc.k.t.Cards[id["Snake Umbra"]].UmbraArmor {
		t.Fatal("Snake Umbra does not carry umbra armor in the table")
	}
	sc.lands(0, id["Mountain"], 6)
	sc.lands(0, id["Swamp"], 6)
	sc.lands(0, id["Plains"], 6)

	// a targeted destroy: the creature stays, the umbra goes
	ogre := sc.perm(id["Gray Ogre"], 1)
	snake := sc.wear(id["Snake Umbra"], ogre)
	sc.castAt(t, sc.hand(id["Murder"], 0), ogre)
	if !sc.onBF(ogre) || !sc.inGrave(snake) {
		t.Fatalf("Murder over an umbra: ogre on battlefield %v, umbra in graveyard %v; want both true", sc.onBF(ogre), sc.inGrave(snake))
	}
	sc.castAt(t, sc.hand(id["Murder"], 0), ogre)
	if sc.onBF(ogre) {
		t.Fatal("a second Murder, no umbra left, did not kill the ogre")
	}

	// a sweeper: every umbra'd creature stays, a bare one dies
	a, b := sc.perm(id["Hill Giant"], 1), sc.perm(id["Grizzly Bears"], 1)
	bear := sc.wear(id["Bear Umbra"], a)
	sc.castCard(t, sc.hand(id["Wrath of God"], 0))
	sc.resolveAll(t)
	if !sc.onBF(a) || !sc.inGrave(bear) || sc.onBF(b) {
		t.Fatalf("Wrath: umbra'd giant on battlefield %v, umbra in graveyard %v, bare bears on battlefield %v; want true, true, false", sc.onBF(a), sc.inGrave(bear), sc.onBF(b))
	}

	// lethal damage (state-based): damage leaves with the umbra
	snake2 := sc.wear(id["Snake Umbra"], a)
	_, tough := sc.s.pt(sc.k, sc.slot(a))
	sc.s.perms[sc.slot(a)].damage = uint8(tough)
	sc.s.touchSl(sc.slot(a)) // a write the engine marks (marks.go): sba's clean-scan key reads the mark
	sc.s.sba(sc.k)
	if !sc.onBF(a) || !sc.inGrave(snake2) || sc.s.perms[sc.slot(a)].damage != 0 {
		t.Fatalf("lethal damage: giant on battlefield %v, umbra in graveyard %v, damage %d; want true, true, 0", sc.onBF(a), sc.inGrave(snake2), sc.s.perms[sc.slot(a)].damage)
	}

	// a deathtouch mark with some damage: the same
	snake3 := sc.wear(id["Snake Umbra"], a)
	sc.s.perms[sc.slot(a)].damage = 1
	sc.s.perms[sc.slot(a)].flags |= pfDeathtouched
	sc.s.touchSl(sc.slot(a))
	sc.s.sba(sc.k)
	if p := &sc.s.perms[sc.slot(a)]; !sc.onBF(a) || !sc.inGrave(snake3) || p.damage != 0 || p.flags&pfDeathtouched != 0 {
		t.Fatalf("deathtouch: giant on battlefield %v, umbra in graveyard %v, damage %d, mark %v; want true, true, 0, false", sc.onBF(a), sc.inGrave(snake3), p.damage, p.flags&pfDeathtouched != 0)
	}

	// no umbra left: lethal damage kills
	sc.s.perms[sc.slot(a)].damage = uint8(tough)
	sc.s.touchSl(sc.slot(a)) // a write the engine marks (marks.go): sba's clean-scan key reads the mark
	sc.s.sba(sc.k)
	if sc.onBF(a) {
		t.Fatal("lethal damage with no umbra left did not kill the giant")
	}
}
