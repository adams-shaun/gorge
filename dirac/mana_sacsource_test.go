package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// spawnScen builds p0's board from the spawnmana pool table: each name
// once per listed copy, untapped and not sick (tokens marked as tokens).
func spawnScen(t *testing.T, cfg Config, board ...string) (*scen, map[string]CardID, []Obj) {
	t.Helper()
	set := cardsettest.Set(t, "spawnmana")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, cfg)
	var objs []Obj
	for _, n := range board {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the spawnmana table", n)
		}
		o := sc.perm(c, 0)
		if tbl.Cards[c].Types&kop.TToken != 0 || n == "c_0_1_eldrazi_spawn_sac" {
			sc.s.objs[o].of |= ofToken
		}
		objs = append(objs, o)
	}
	return sc, id, objs
}

// xAsk walks to Old Stickfingers's X ask and returns it.
func xAsk(t *testing.T, sc *scen, d Decision, what string) Decision {
	t.Helper()
	for i := 0; i < 8; i++ {
		if d.Kind == DkChoose && d.Sub == ChX {
			return d
		}
		if d.Kind == DkNone {
			t.Fatalf("%s: game ended before the X ask", what)
		}
		sc.s.Apply(d.Options[0])
		d = sc.s.Step()
	}
	t.Fatalf("%s: no X ask", what)
	return d
}

// TestSpawnSacManaOffersCast: beside a Swamp and a Forest, one untapped
// Eldrazi Spawn makes Old Stickfingers's {X}{B}{G} a two-X cast (CR 605.1a:
// the ability is a mana ability; gorge's planner lists the sacrifice-only
// source as last-resort fixed production,
// paymentPlanManaUnitsOnlyCompute's appended group). The Spawn's cost taps
// nothing, so summoning sickness does not bar it. With only the Spawn (no
// lands), the {B}{G} is unpaid and the cast is not offered.
func TestSpawnSacManaOffersCast(t *testing.T) {
	sc, id, _ := spawnScen(t, Config{NoElide: true, GorgeLowering: true}, "c_0_1_eldrazi_spawn_sac", "Swamp", "Forest")
	os := sc.hand(id["Old Stickfingers"], 0)
	sc.do(t, AkCast, TCode(os), scAny)
	d := xAsk(t, sc, sc.s.Step(), "the cast")
	if d.N != 2 || d.Options[1].Mode() != 1 {
		t.Fatalf("X options %v, want X = 0 and 1 (the Spawn's {C})", d.Options[:d.N])
	}
	sc2, id2, objs2 := spawnScen(t, Config{NoElide: true, GorgeLowering: true}, "c_0_1_eldrazi_spawn_sac", "Swamp", "Forest")
	os2 := sc2.hand(id2["Old Stickfingers"], 0)
	sc2.s.perms[sc2.slot(objs2[0])].flags |= pfSick
	sc2.do(t, AkCast, TCode(os2), scAny)
	d = xAsk(t, sc2, sc2.s.Step(), "the sick cast")
	if d.N != 2 || d.Options[1].Mode() != 1 {
		t.Fatalf("sick: X options %v, want X = 0 and 1 (the Spawn's {C})", d.Options[:d.N])
	}
	sc3, id3, _ := spawnScen(t, Config{NoElide: true, GorgeLowering: true}, "c_0_1_eldrazi_spawn_sac")
	os3 := sc3.hand(id3["Old Stickfingers"], 0)
	d = sc3.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && a.Source() == TCode(uint8(os3)) {
			t.Errorf("Old Stickfingers offered with no {B}{G} source")
		}
	}
	// Sowing Mycospawn's {1}{C} kicker is the pool's direct {C}-pip cast:
	// with the Spawn and five Forests the {3}{G}+{1}{C} kicked cast is
	// offered; without the Spawn four Forests offer only the plain {3}{G}.
	sc4, id4, _ := spawnScen(t, Config{NoElide: true}, "c_0_1_eldrazi_spawn_sac", "Forest", "Forest", "Forest", "Forest", "Forest")
	sm := sc4.hand(id4["Sowing Mycospawn"], 0)
	d = sc4.s.Step()
	plain, kicked := false, false
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(sm)) {
			if a.Mode()&1 != 0 {
				kicked = true
			} else {
				plain = true
			}
		}
	}
	if !kicked {
		t.Errorf("Sowing Mycospawn kicked {3}{G}+{1}{C} not offered with a Spawn: %v", d.Options[:d.N])
	}
	sc5, id5, _ := spawnScen(t, Config{NoElide: true}, "Forest", "Forest", "Forest", "Forest")
	sm5 := sc5.hand(id5["Sowing Mycospawn"], 0)
	d = sc5.s.Step()
	plain, kicked = false, false
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(sm5)) {
			if a.Mode()&1 != 0 {
				kicked = true
			} else {
				plain = true
			}
		}
	}
	if kicked || !plain {
		t.Errorf("without a Spawn: plain offered %v kicked offered %v, want only the plain cast: %v", plain, kicked, d.Options[:d.N])
	}
}

// TestSpawnSacManaPaysCast: the X = 1 cast is paid with the Spawn's {C}
// and the lands' {B}{G}: the Spawn is sacrificed to the graveyard, the
// lands tap, and the pool is empty.
func TestSpawnSacManaPaysCast(t *testing.T) {
	sc, id, objs := spawnScen(t, Config{NoElide: true}, "c_0_1_eldrazi_spawn_sac", "Swamp", "Forest")
	spawn, swamp := objs[0], objs[1]
	os := sc.hand(id["Old Stickfingers"], 0)
	sc.do(t, AkCast, TCode(os), scAny)
	xAsk(t, sc, sc.s.Step(), "the cast")
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1)) // X = 1
	sc.passBoth(t)
	sc.s.Step()
	if sc.onBF(spawn) || sc.s.objs[spawn].zone != kop.ZoneGraveyard {
		t.Errorf("Spawn on bf %v zone %d, want sacrificed to the graveyard", sc.onBF(spawn), sc.s.objs[spawn].zone)
	}
	if !sc.tapped(swamp) {
		t.Errorf("Swamp untapped")
	}
	if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
		t.Errorf("pool %v after the cast, want empty", pool)
	}
}

// TestSpawnSacManaFiresSacTrigger: the mana sacrifice fires sacrifice
// triggers (CR 605.3c: the activation pays its cost complete): Writhing
// Chrysalis's "whenever you sacrifice another Eldrazi Spawn" puts a +1/+1
// counter on itself when the Spawn's {C} pays. (Old Stickfingers's own cast
// trigger makes two more Spawns first, and the trigger resolves after the
// spell.)
func TestSpawnSacManaFiresSacTrigger(t *testing.T) {
	for _, cfg := range []Config{{NoElide: true}, {NoElide: true, GorgeLowering: true}} {
		testSpawnSacFiresSacTrigger(t, cfg)
	}
}

func testSpawnSacFiresSacTrigger(t *testing.T, cfg Config) {
	sc, id, objs := spawnScen(t, cfg, "Writhing Chrysalis", "c_0_1_eldrazi_spawn_sac", "Swamp", "Forest")
	ch, spawn := objs[0], objs[1]
	os := sc.hand(id["Old Stickfingers"], 0)
	sc.do(t, AkCast, TCode(os), scAny)
	xAsk(t, sc, sc.s.Step(), "the cast")
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1)) // X = 1
	d := sc.s.Step()
	for i := 0; i < 40; i++ {
		if sc.onBF(ch) && sc.s.perms[sc.slot(ch)].ctrLow(int(kop.CtrP1P1)) >= 1 {
			break
		}
		if d.Kind == DkNone {
			break
		}
		sc.s.Apply(d.Options[0])
		d = sc.s.Step()
	}
	if sc.onBF(spawn) {
		t.Errorf("the first Spawn still on the battlefield")
	}
	if !sc.onBF(ch) || sc.s.perms[sc.slot(ch)].ctrLow(int(kop.CtrP1P1)) < 1 {
		t.Errorf("Chrysalis +1/+1 counters %d, want 1", sc.s.perms[sc.slot(ch)].ctrLow(int(kop.CtrP1P1)))
	}
}

// TestSpawnSacManaTappedPays: a tapped Spawn's sacrifice has no {T} in its cost
// (CR 602.2b), so gorge's cast planner counts it and the X ask prices X = 1 as
// well ({B}{G} from the lands, the Spawn's {C} for the X).
func TestSpawnSacManaTappedPays(t *testing.T) {
	sc, id, objs := spawnScen(t, Config{NoElide: true}, "c_0_1_eldrazi_spawn_sac", "Swamp", "Forest")
	sc.s.perms[sc.slot(objs[0])].flags |= pfTapped
	os := sc.hand(id["Old Stickfingers"], 0)
	sc.do(t, AkCast, TCode(os), scAny)
	d := xAsk(t, sc, sc.s.Step(), "the tapped cast")
	if d.N != 2 || d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("X options %v, want X = 0 and X = 1 (the tapped Spawn pays)", d.Options[:d.N])
	}
}

// mapScen is spawnScen's pool with a Map token's activated ability appended
// ("{1}, {T}, Sacrifice this token: Target creature you control explores.
// Activate only as a sorcery.", the mzpauper kgen lowering of
// c_a_map_sac_explore): p0 controls it beside a TAPPED Eldrazi Spawn, the
// only source of its {1}, and the named
// extra permanents.
func mapScen(t *testing.T, extra ...string) (*scen, Obj, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "spawnmana")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "c_a_map_sac_explore",
		Types: kop.TArtifact | kop.TToken,
		Abilities: []kop.Ability{{
			Kind:     kop.AbActivated,
			Cost:     kop.Cost{Mana: kop.ManaCost{Generic: 1}, Flags: kop.CostTap | kop.CostSacSelf | kop.CostSorcery},
			Targets:  [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: kop.FilterID(len(tbl.Filters) - 1)}},
			NTargets: 1,
			Prog:     []kop.Instr{{Op: kop.OpExplore, Obj: kop.RefTarget0}},
		}},
	})
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	spawn := sc.perm(id["c_0_1_eldrazi_spawn_sac"], 0)
	sc.s.objs[spawn].of |= ofToken
	sc.s.perms[sc.slot(spawn)].flags |= pfTapped
	m := sc.perm(id["c_a_map_sac_explore"], 0)
	sc.s.objs[m].of |= ofToken
	for _, n := range extra {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the spawnmana table", n)
		}
		sc.perm(c, 0)
	}
	return sc, spawn, m
}

func mapOffered(d *Decision, m Obj) Action {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(m)) {
			return a
		}
	}
	return 0
}

// TestSpawnSacTappedPaysTheMapsGeneric: the Map's {1} is payable by the tapped
// Spawn's sacrifice, a mana ability needing no untap, so the activation is offered
// whether or not the Spawn is the only creature to target (gorge's offer walk
// lists it beside the lone Spawn).
func TestSpawnSacTappedPaysTheMapsGeneric(t *testing.T) {
	sc, _, m := mapScen(t)
	d := sc.s.Step()
	if a := mapOffered(&d, m); a == 0 {
		t.Fatalf("Map activation not offered with the tapped Spawn its only payer: %v", d.Options[:d.N])
	}
	sc2, _, m2 := mapScen(t, "Warped Tusker")
	d = sc2.s.Step()
	if a := mapOffered(&d, m2); a == 0 {
		t.Fatalf("Map activation not offered beside a second creature: %v", d.Options[:d.N])
	}
}
