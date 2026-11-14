package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// pathSpawnScen seats the mbc pool's Path of Annihilation board: each listed
// name once under seat 0, untapped and not sick, the Eldrazi Spawn a token.
// The matchup's table holds the listed cards, Path's Eldrazi Spawn token and
// the five basics, so the deck carries each named card and a basic spread.
func pathSpawnScen(t *testing.T, cfg Config, board ...string) (*scen, map[string]CardID, []Obj) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Path of Annihilation", Count: 4}, {Name: "Goblin Taskmaster", Count: 4},
		{Name: "Larger Than Life", Count: 4}, {Name: "Exquisite Huntmaster", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6}, {Name: "Plains", Count: 6}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = c
		}
	}
	for c := range tbl.Cards { // the token's script is in the table beside its carrier
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, cfg)
	var objs []Obj
	for _, n := range board {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the mbc matchup table", n)
		}
		o := sc.perm(c, 0)
		if tbl.Cards[c].Types&kop.TToken != 0 || n == "c_0_1_eldrazi_spawn_sac" {
			sc.s.objs[o].of |= ofToken
		}
		objs = append(objs, o)
	}
	return sc, id, objs
}

// TestPlannerPaysGrantedSpawnCast: an untapped Eldrazi Spawn under Path of
// Annihilation is two census units, its granted "{T}: Add one mana of any
// color" beside its printed "Sacrifice this creature: Add {C}" (CR 602.2b,
// 605.1a; availGrant's ownSac shape), so a cast the census pays is offered
// and the witness's plan must pay it too. The planner priced the token as
// its printed sacrifice alone: phase 2 of plan holds a granted source's
// printed ability, never the grant beside it, so a cast the printed {C}
// cannot pay panicked "autopay found no plan".
func TestPlannerPaysGrantedSpawnCast(t *testing.T) {
	// the premise: the census credits the grant. Without the Spawn, Path of
	// Annihilation is no mana source and the {R} cast is not offered; with a
	// summoning-sick Spawn the grant cannot be activated (CR 302.6) and the
	// census credits only the sacrifice, so it is not offered either.
	sc0, id0, _ := pathSpawnScen(t, Config{NoElide: true, GorgeLowering: true}, "Path of Annihilation")
	tm0 := sc0.hand(id0["Goblin Taskmaster"], 0)
	d := sc0.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && a.Source() == TCode(uint8(tm0)) {
			t.Fatalf("{R} offered with no mana source: %v", d.Options[:d.N])
		}
	}
	sc0s, id0s, objs0s := pathSpawnScen(t, Config{NoElide: true, GorgeLowering: true}, "Path of Annihilation", "c_0_1_eldrazi_spawn_sac")
	sc0s.s.perms[sc0s.slot(objs0s[1])].flags |= pfSick
	tm0s := sc0s.hand(id0s["Goblin Taskmaster"], 0)
	d = sc0s.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && a.Source() == TCode(uint8(tm0s)) {
			t.Fatalf("{R} offered with only a summoning-sick Spawn's grant: %v", d.Options[:d.N])
		}
	}
	// the grant alone pays {R}: the Spawn taps for it and stays on the
	// battlefield; its printed sacrifice is no part of this payment.
	sc, id, objs := pathSpawnScen(t, Config{NoElide: true, GorgeLowering: true}, "Path of Annihilation", "c_0_1_eldrazi_spawn_sac")
	spawn := objs[1]
	tm := sc.hand(id["Goblin Taskmaster"], 0)
	sc.do(t, AkCast, TCode(tm), scAny)
	if sc.s.perms[sc.slot(spawn)].flags&pfTapped == 0 {
		t.Errorf("the Spawn is untapped after the {R} cast, want the granted tap")
	}
	if !sc.onBF(spawn) {
		t.Errorf("the Spawn left the battlefield paying {R}, want it tapped, not sacrificed")
	}
	// both units pay {1}{G}: the granted tap makes the {G}, the printed
	// sacrifice the generic one, the token tapped and then sacrificed.
	sc2, id2, objs2 := pathSpawnScen(t, Config{NoElide: true, GorgeLowering: true}, "Path of Annihilation", "c_0_1_eldrazi_spawn_sac")
	spawn2 := objs2[1]
	ltl := sc2.hand(id2["Larger Than Life"], 0)
	sc2.do(t, AkCast, TCode(ltl), scAny)
	if sc2.onBF(spawn2) {
		t.Errorf("the Spawn stayed on the battlefield paying {1}{G}, want it tapped then sacrificed")
	}
	// gorge's census holds printed abilities only, so a cast the printed
	// plan pays is planned as gorge does (lockstep pair 2's Exquisite
	// Huntmaster beside two Swamps and a Manor Gate): three Swamps pay
	// {3}{B} with the Spawn's printed sacrifice for the last generic, and
	// the granted tap is not cheaper to gorge's witness -- the plan keeps it
	// unused. The plan is read dry over that board: the Spawn's one unit in
	// it is the printed sacrifice, never the grant.
	sc3, _, objs3 := pathSpawnScen(t, Config{NoElide: true, GorgeLowering: true},
		"Path of Annihilation", "c_0_1_eldrazi_spawn_sac", "Swamp", "Swamp", "Swamp")
	spawn3 := sc3.slot(objs3[1])
	var cost kop.ManaCost
	cost.Generic, cost.Pips[kop.B] = 3, 1
	var pk planner
	pk.dry = true
	sc3.s.tapForPK(sc3.k, 0, &cost, 0, NoObj, false, false, &pk)
	if !pk.found {
		t.Fatalf("no dry plan for {3}{B} over three Swamps and the Spawn")
	}
	uses := 0
	for i := 0; i < pk.bestN; i++ {
		if sl := pk.units[pk.bestSeq[i]]; sl == spawn3 {
			uses++
			if ty := &pk.types[pk.bestTy[i]]; !ty.sacOnly {
				t.Errorf("the plan uses the Spawn's granted tap beside its printed sacrifice, want the sacrifice alone")
			}
		}
	}
	if uses != 1 {
		t.Errorf("the plan uses the Spawn %d times, want its printed sacrifice once", uses)
	}
}
