package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// spawnPairScen seats seat 0's Path of Annihilation beside two Eldrazi Spawn
// tokens whose battlefield (entry) order is the REVERSE of their object-id
// order: the first-created token (the lower ObjID) enters second, as lockstep
// pair 10's #103/#104 do (gorge's prefix search walks its gaps in engine
// order, so the higher id's grant is the first gap there). Nothing else is on
// the battlefield, so the Spawns' printed sacrifices are the only printed
// units and no {W} pip has a printed source.
func spawnPairScen(t *testing.T, cfg Config) (*scen, Slot, Slot) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Path of Annihilation", Count: 4}, {Name: "Destroy Evil", Count: 4},
		{Name: "Goblin Taskmaster", Count: 4}, {Name: "Larger Than Life", Count: 4},
		{Name: "Exquisite Huntmaster", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Island", Count: 6},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 6}}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tok, ok := CardID(0), false
	for c := range tbl.Cards { // the token's script sits beside its carrier
		if tbl.Cards[c].Name == "c_0_1_eldrazi_spawn_sac" {
			tok, ok = CardID(c), true
			break
		}
	}
	if !ok {
		t.Fatal("the Spawn token is not in the mbc matchup table")
	}
	sc := newScenTable(t, tbl, cfg)
	sc.perm(tableID(t, tbl, "Path of Annihilation"), 0)
	// the lower ObjID enters second: gorge's prefix taps the higher id's
	// grant first and gorge's printed plan sacrifices the lower id
	oSec := sc.obj(tok, 0) // the lower ObjID, the later entry
	oFirst := sc.obj(tok, 0)
	for _, o := range []Obj{oFirst, oSec} {
		sc.s.objs[o].of |= ofToken
		sl := sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
		sc.s.perms[sl].flags &^= pfSick
	}
	sc.s.npend = 0
	return sc, sc.s.objs[oFirst].slot, sc.s.objs[oSec].slot
}

func tableID(t *testing.T, tbl *kop.Table, name string) CardID {
	t.Helper()
	for c := range tbl.Cards {
		if tbl.Cards[c].Name == name {
			return CardID(c)
		}
	}
	t.Fatalf("%s not in the mbc matchup table", name)
	return 0
}

// TestSpawnGrantPaymentChoice: a cast the printed sacrifice-only units cannot
// pay is funded, as gorge's payment funds it, by the census gaps first --
// the prefix search's activations (potential_script.go) -- and then by gorge's
// printed plan on the remainder. The plan must therefore compose the granted
// tap and the printed sacrifice of the tokens gorge's planner picks: the
// prefix's first gap's grant (the higher ObjID's token, the first in entry
// order) tapped, and gorge's printed plan's least-ObjID sacrifice spent.
func TestSpawnGrantPaymentChoice(t *testing.T) {
	sc, firstSl, secSl := spawnPairScen(t, Config{NoElide: true, GorgeLowering: true})
	// the premise: the printed units alone cannot pay {1}{W} (the {W} pip has
	// no printed source), while the census credits each untapped Spawn its
	// granted "{T}: Add one mana of any color" beside the printed sacrifice
	// (CR 602.2b, 605.1a), so the plan composes a grant and a sacrifice.
	var cost kop.ManaCost
	cost.Generic, cost.Pips[kop.W] = 1, 1
	var pk planner
	pk.dry = true
	sc.s.tapForPK(sc.k, 0, &cost, 0, NoObj, false, false, &pk)
	if !pk.found {
		t.Fatal("no plan for {1}{W} beside the two Spawns")
	}
	grants, sacs := 0, 0
	for i := 0; i < pk.bestN; i++ {
		u := pk.bestSeq[i]
		ty := &pk.types[pk.utyp[u]]
		sl := pk.units[u]
		if ty.sac || ty.sacOnly {
			sacs++
			if sl != secSl {
				t.Errorf("the plan sacrifices the first-in-entry token's printed sacrifice, want the lower ObjID's (gorge's printed plan)")
			}
			continue
		}
		grants++
		if ty.mask != 0x1f || ty.n != 1 { // any colour (WUBRG) x1
			t.Errorf("the plan's granted unit is mask %d x%d, want any colour x1", ty.mask, ty.n)
		}
		if sl != firstSl {
			t.Errorf("the plan taps the lower ObjID token's grant, want the first-in-entry token's (gorge's prefix gap order)")
		}
	}
	if grants != 1 || sacs != 1 {
		t.Errorf("the plan uses %d grants and %d sacrifices, want one of each", grants, sacs)
	}
	// the whole cost paid by the grant alone (a {W} cast) leaves the printed
	// sacrifices out: gorge's prefix floats the {W} and its printed plan pays
	// a zero remainder
	var cost2 kop.ManaCost
	cost2.Pips[kop.W] = 1
	var pk2 planner
	pk2.dry = true
	sc.s.tapForPK(sc.k, 0, &cost2, 0, NoObj, false, false, &pk2)
	if !pk2.found {
		t.Fatal("no plan for {W} beside the two Spawns")
	}
	for i := 0; i < pk2.bestN; i++ {
		u := pk2.bestSeq[i]
		ty := &pk.types[pk.utyp[u]]
		if ty.sac || ty.sacOnly {
			t.Errorf("the {W} plan sacrifices a Spawn, want the granted tap alone")
		}
		if pk2.units[u] != firstSl {
			t.Errorf("the {W} plan taps the lower ObjID token's grant, want the first-in-entry token's")
		}
	}
	// the lowered cast: gorge pays {1}{W} with the first-in-entry token's
	// granted tap floating the {W} and the lower ObjID token's printed
	// sacrifice the {1}: the first stays on the battlefield, tapped
	sc2, firstSl2, secSl2 := spawnPairScen(t, Config{NoElide: true, GorgeLowering: true})
	de := sc2.hand(tableID(t, sc2.k.t, "Destroy Evil"), 0)
	sc2.do(t, AkCast, TCode(uint8(de)), scAny)
	if sc2.onBF(objOfSlot(sc2, secSl2)) {
		t.Errorf("the lower ObjID token stayed on the battlefield, want it sacrificed for the {1}")
	}
	if sc2.s.perms[secSl2].flags&pfTapped != 0 {
		t.Errorf("the sacrificed token's slot is marked tapped")
	}
	if !sc2.onBF(objOfSlot(sc2, firstSl2)) {
		t.Errorf("the first-in-entry token left the battlefield, want it alive, its grant tapped")
	}
	if sc2.s.perms[firstSl2].flags&pfTapped == 0 {
		t.Errorf("the first-in-entry token is untapped, want its granted tap used for the {W}")
	}
}

// objOfSlot is the battlefield object of a slot (the test's own slots are all
// on the battlefield).
func objOfSlot(sc *scen, sl Slot) Obj { return sc.s.perms[sl].obj }
