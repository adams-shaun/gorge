package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// orderSources checks that d is a TriggerOrder decision and returns the
// pending triggers' source objects in queue order (gorge's list order; the
// options themselves are sorted canonically and name a queue index).
func orderSources(t *testing.T, s *State, d Decision) []uint8 {
	t.Helper()
	if d.Kind != DkTriggerOrder {
		t.Fatalf("decision kind %d, want trigger order", d.Kind)
	}
	var out []uint8
	for i := 0; i < int(s.npend); i++ {
		out = append(out, uint8(s.pend[i].obj))
	}
	return out
}

// Simultaneous SpellCast triggers queue in gorge's battlefield order (entry
// order), not slot order: vanilla game 327 (seed 328), two Jeskai Students
// whose slots and entry order disagree.
func TestCastTriggersInBattlefieldOrder(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.perm(v.Mountain, 0)
	a := sc.perm(v.JeskaiStudent, 0)
	b := sc.perm(v.JeskaiStudent, 0)
	// b entered first: its slot is later, its timestamp earlier
	pa, pb := &sc.s.perms[sc.slot(a)], &sc.s.perms[sc.slot(b)]
	pa.ts, pb.ts = pb.ts, pa.ts
	bolt := sc.hand(v.LightningBolt, 0)
	sc.do(t, AkCast, TCode(bolt), scAny)
	sc.do(t, AkTarget, scAny, TCode(PlayerTarget1))
	got := orderSources(t, sc.s, sc.s.Step())
	if len(got) != 2 || got[0] != uint8(b) || got[1] != uint8(a) {
		t.Fatalf("prowess order %v, want [%d %d]", got, b, a)
	}
}

// The declaration's Attacks and AttackersDeclared triggers queue in one
// battlefield walk (gorge checks one DeclareAttackers event against every
// listener in entry order): FDN pair 92, Battlesong Berserker's "whenever
// you attack" around Drakuseth's "whenever this attacks".
func TestAttackTriggersInBattlefieldOrder(t *testing.T) {
	deck := []cardset.Entry{{Name: "Battlesong Berserker", Count: 2}, {Name: "Drakuseth, Maw of Flames", Count: 1}, {Name: "Mountain", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	b1 := sc.perm(id["Battlesong Berserker"], 0)
	drak := sc.perm(id["Drakuseth, Maw of Flames"], 0)
	b2 := sc.perm(id["Battlesong Berserker"], 0)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(drak), scAny)
	sc.do(t, AkDone, scAny, scAny)
	got := orderSources(t, sc.s, sc.s.Step())
	want := []uint8{uint8(b1), uint8(drak), uint8(b2)}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("attack trigger order %v, want %v", got, want)
	}
}

// Combat damage fires per source in gorge's event order: the attackers in
// battlefield order, each source's damage triggers, then its life loss
// (FDN pair 1344: Goldvein Pick before Bloodthirsty Conqueror; pair 437:
// two damage sources in entry order, not slot order).
func TestCombatDamageTriggersInEventOrder(t *testing.T) {
	deck := []cardset.Entry{{Name: "Eager Trufflesnout", Count: 2}, {Name: "Bloodthirsty Conqueror", Count: 1}, {Name: "Swamp", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	ta := sc.perm(id["Eager Trufflesnout"], 0)
	tb := sc.perm(id["Eager Trufflesnout"], 0)
	conq := sc.perm(id["Bloodthirsty Conqueror"], 0)
	// tb entered first
	pa, pb := &sc.s.perms[sc.slot(ta)], &sc.s.perms[sc.slot(tb)]
	pa.ts, pb.ts = pb.ts, pa.ts
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(ta), scAny)
	sc.do(t, AkAttack, TCode(tb), scAny)
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepDamage)
	var d Decision
	for i := 0; i < 20; i++ {
		if d = sc.s.Step(); d.Kind != DkPriority {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	got := orderSources(t, sc.s, d)
	want := []uint8{uint8(tb), uint8(conq), uint8(ta), uint8(conq)}
	if len(got) != 4 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("combat damage trigger order %v, want %v", got, want)
	}
}

// Before turn 1 gorge's active player is the starting player (the features'
// active bit at the opening asks, FDN pair 61).
func TestPregameActiveIsStartingPlayer(t *testing.T) {
	var lib [40]CardID
	for i := range lib {
		lib[i] = v.Plains
	}
	for seed := uint64(1); seed < 64; seed++ {
		s := new(State)
		s.Init(v.Table, lib[:], lib[:], seed, Config{Mulligans: 2})
		if s.StartingPlayer() != 1 {
			continue
		}
		if d := s.Step(); d.Kind != DkMulligan {
			t.Fatalf("first decision kind %d, want mulligan", d.Kind)
		}
		var p Projection
		s.FeatProjGlobals(&p)
		if p.Active != 1 {
			t.Fatalf("seed %d: pregame active %d, want 1", seed, p.Active)
		}
		return
	}
	t.Skip("no seed with starting player 1")
}

// Each combat damage event is checked against every listener once, in
// battlefield order, for both DamageDone and DamageDealtOnce (gorge
// checkTriggers per Damage event; the Once line latches per source and
// batch): FDN pair 2582, Quilled Greatwurm's "whenever a creature you
// control deals combat damage" listed before Eager Trufflesnout's "deals
// combat damage to a player". Trufflesnout's own event queues the
// Greatwurm's line first.
func TestCombatDamageOneWalkPerEvent(t *testing.T) {
	deck := []cardset.Entry{{Name: "Quilled Greatwurm", Count: 1}, {Name: "Eager Trufflesnout", Count: 1}, {Name: "Forest", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	qg := sc.perm(id["Quilled Greatwurm"], 0)
	et := sc.perm(id["Eager Trufflesnout"], 0)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(qg), scAny)
	sc.do(t, AkAttack, TCode(et), scAny)
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepDamage)
	var d Decision
	for i := 0; i < 20; i++ {
		if d = sc.s.Step(); d.Kind != DkPriority {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	got := orderSources(t, sc.s, d)
	want := []uint8{uint8(qg), uint8(qg), uint8(et)}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("combat damage trigger order %v, want %v", got, want)
	}
}

// An effect's sacrifices leave in gorge's emit order: player by player in
// the order asked (the active player first for "each player"), so the dies
// triggers queue in that order (FDN pairs 1644, 6295, 8038: Blasphemous
// Edict and Liliana's -4 with High-Society Hunter and Liliana's "whenever a
// creature you control dies"). Player 1 is active and casts the Edict:
// its Hunter dies first (Liliana triggers), then player 0's creature (the
// Hunter, looking back, triggers).
func TestEachPlayerSacrificeActiveFirst(t *testing.T) {
	deck := []cardset.Entry{{Name: "Blasphemous Edict", Count: 1}, {Name: "High-Society Hunter", Count: 1},
		{Name: "Liliana, Dreadhorde General", Count: 1}, {Name: "Eager Trufflesnout", Count: 1}, {Name: "Swamp", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.s.active, sc.s.prio = 1, 1
	x := sc.perm(id["Eager Trufflesnout"], 0)
	lil := sc.perm(id["Liliana, Dreadhorde General"], 1)
	hsh := sc.perm(id["High-Society Hunter"], 1)
	for i := 0; i < 5; i++ {
		sc.perm(id["Swamp"], 1)
	}
	edict := sc.hand(id["Blasphemous Edict"], 1)
	sc.do(t, AkCast, TCode(edict), scAny)
	var d Decision
	for i := 0; i < 20; i++ {
		if d = sc.s.Step(); d.Kind != DkPriority {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if sc.onBF(x) || sc.onBF(hsh) {
		t.Fatalf("the Edict left a creature: x=%v hunter=%v", sc.onBF(x), sc.onBF(hsh))
	}
	got := orderSources(t, sc.s, d)
	want := []uint8{uint8(lil), uint8(hsh)}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("dies trigger order %v, want %v", got, want)
	}
}

// TestEachPlayerChoosesThenSacrificesTogether: Liliana's -4 with a choice
// for both players. Each player chooses in turn, then what they chose is
// sacrificed at once (CR 101.4), so the active player's picks are still on the
// battlefield at the other player's ask.
func TestEachPlayerChoosesThenSacrificesTogether(t *testing.T) {
	deck := []cardset.Entry{{Name: "Liliana, Dreadhorde General", Count: 1}, {Name: "Eager Trufflesnout", Count: 3}, {Name: "Swamp", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.s.active, sc.s.prio = 1, 1
	var mine, theirs [3]Obj
	for i := range mine {
		theirs[i] = sc.perm(id["Eager Trufflesnout"], 0)
		mine[i] = sc.perm(id["Eager Trufflesnout"], 1)
	}
	lil := sc.perm(id["Liliana, Dreadhorde General"], 1)
	sc.s.perms[sc.slot(lil)].setCtrN(sc.s, int(kop.CtrLoyalty), 6)
	d := sc.s.Step()
	act := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(lil)) && a.Mode() == 1 {
			sc.s.Apply(a)
			act = true
			break
		}
	}
	if !act {
		t.Fatalf("Liliana's -4 not offered: %v", d.Options[:d.N])
	}
	for i := 0; i < 10; i++ {
		if d = sc.s.Step(); d.Kind != DkPriority {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	for pick := 0; pick < 2; pick++ {
		if d.Kind != DkChoose || d.Player != 1 {
			t.Fatalf("pick %d: decision kind %d for p%d, want p1's sacrifice", pick, d.Kind, d.Player)
		}
		sc.s.Apply(MakeAction(AkChoose, TCode(mine[pick]), 0, 0))
		d = sc.s.Step()
	}
	if d.Kind != DkChoose || d.Player != 0 {
		t.Fatalf("decision kind %d for p%d, want p0's sacrifice", d.Kind, d.Player)
	}
	for _, o := range mine[:2] {
		if z := sc.s.objs[o].zone; z != kop.ZoneBattlefield {
			t.Fatalf("p1's chosen creature %d in zone %d at p0's ask, want the battlefield", o, z)
		}
	}
	for pick := 0; pick < 2; pick++ {
		sc.s.Apply(MakeAction(AkChoose, TCode(theirs[pick]), 0, 0))
		sc.s.Step()
	}
	for _, o := range []Obj{mine[0], mine[1], theirs[0], theirs[1]} {
		if z := sc.s.objs[o].zone; z != kop.ZoneGraveyard {
			t.Fatalf("chosen creature %d in zone %d after the last answer, want the graveyard", o, z)
		}
	}
}

// TestChangeZoneAllWalksSeatZeroFirst: Rise of the Dark Realms returns
// every graveyard's creatures in gorge's order, seat 0's graveyard first
// whoever is active (changeZoneAllPlayers: AliveFrom(0)); the kernel
// walked the active player's first (pairs 249, 858: the battlefield
// list order then differed).
func TestChangeZoneAllWalksSeatZeroFirst(t *testing.T) {
	deck := []cardset.Entry{{Name: "Rise of the Dark Realms", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Swamp", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.s.active, sc.s.prio = 1, 1
	grave := func(p uint8) Obj {
		o := sc.obj(id["Eager Trufflesnout"], p)
		sc.s.place(o, kop.ZoneGraveyard, false)
		return o
	}
	g1 := grave(1)
	g0 := grave(0)
	for i := 0; i < 9; i++ {
		sc.perm(id["Swamp"], 1)
	}
	rise := sc.hand(id["Rise of the Dark Realms"], 1)
	sc.do(t, AkCast, TCode(rise), scAny)
	for i := 0; i < 10 && !sc.onBF(g0); i++ {
		if d := sc.s.Step(); d.Kind != DkPriority {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if !sc.onBF(g0) || !sc.onBF(g1) {
		t.Fatalf("Rise left a creature in a graveyard: seat 0's %v, seat 1's %v", sc.onBF(g0), sc.onBF(g1))
	}
	if a, b := sc.s.perms[sc.slot(g0)].ts, sc.s.perms[sc.slot(g1)].ts; a > b {
		t.Fatalf("seat 1's creature entered first (ts %d < %d), want seat 0's graveyard first", b, a)
	}
}

// TestMergedGrantTriggersPerApplication: two Fake Your Own Deaths on one
// creature merge into one override record with two applications, and
// each is its own instance of the granted dies trigger (gorge lists two
// and asks their order; pairs 3122, 3993, 6180).
func TestMergedGrantTriggersPerApplication(t *testing.T) {
	deck := []cardset.Entry{{Name: "Fake Your Own Death", Count: 2}, {Name: "Eager Trufflesnout", Count: 1}, {Name: "Swamp", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	x := sc.perm(id["Eager Trufflesnout"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	for i := 0; i < 2; i++ {
		f := sc.hand(id["Fake Your Own Death"], 0)
		sc.do(t, AkCast, TCode(f), scAny)
		for j := 0; j < 10 && sc.s.objs[f].zone != kop.ZoneGraveyard; j++ {
			d := sc.s.Step()
			if d.Kind == DkTarget {
				sc.do(t, AkTarget, scAny, TCode(x))
				continue
			}
			if d.Kind != DkPriority {
				t.Fatalf("decision kind %d", d.Kind)
			}
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	sc.s.npend = 0
	sc.s.fireLeave(sc.k, sc.slot(x), kop.ZoneGraveyard)
	if sc.s.npend != 2 {
		t.Fatalf("%d dies triggers pending, want 2 (one per Fake Your Own Death)", sc.s.npend)
	}
}

// A combat damage event to a player triggers its damage and its life-loss
// listeners together, in listener (battlefield) order: Bloodthirsty
// Conqueror on the battlefield before Eager Trufflesnout queues first
// (the pair 15166: Conqueror before the Goldvein Pick it entered before).
func TestCombatDamageLifeLossListenerOrder(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Eager Trufflesnout", Count: 1}, {Name: "Bloodthirsty Conqueror", Count: 1}, {Name: "Swamp", Count: 10}})
	conq := sc.perm(id["Bloodthirsty Conqueror"], 0)
	ta := sc.perm(id["Eager Trufflesnout"], 0)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(ta), scAny)
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepDamage)
	var d Decision
	for i := 0; i < 20; i++ {
		if d = sc.s.Step(); d.Kind != DkPriority {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	got := orderSources(t, sc.s, d)
	if len(got) != 2 || got[0] != uint8(conq) || got[1] != uint8(ta) {
		t.Fatalf("combat damage trigger order %v, want [%d %d]", got, conq, ta)
	}
}

// A permanent entering with counters queues its counters-placed triggers
// before the enters triggers, as gorge does (Exemplar of Light, entering
// with two +1/+1 counters from Giada, before Dazzling Angel's "another
// creature you control enters"; the pair 13397).
func TestEntryCounterTriggerBeforeETB(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Exemplar of Light", Count: 1}, {Name: "Dazzling Angel", Count: 1}, {Name: "Giada, Font of Hope", Count: 1}, {Name: "Plains", Count: 10}})
	sc.perm(id["Dazzling Angel"], 0)
	sc.perm(id["Giada, Font of Hope"], 0)
	sc.s.npend = 0
	ex := sc.obj(id["Exemplar of Light"], 0)
	sc.s.enter(sc.k, ex, 0, false, kop.ZoneNone)
	if sc.s.npend != 2 {
		t.Fatalf("%d triggers queued (Exemplar entered with %v)", sc.s.npend, sc.s.perms[sc.slot(ex)].ck)
	}
	if sc.s.pend[0].obj != ex {
		t.Fatalf("first queued trigger from %d, want Exemplar of Light %d", sc.s.pend[0].obj, ex)
	}
}

// An object's delayed "at the beginning of the next end step" trigger
// (Electroduplicate's sacrifice of its token copy) is queued at the
// object's place among the end step's printed triggers, in listener
// order, as gorge walks it as a trigger of the token (the pair 17598).
func TestDelayedEndStepTriggerListenerOrder(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Sylvan Scavenging", Count: 2}, {Name: "Llanowar Elves", Count: 1}, {Name: "Forest", Count: 8}})
	a := sc.perm(id["Sylvan Scavenging"], 0)
	tok := sc.perm(id["Llanowar Elves"], 0)
	b := sc.perm(id["Sylvan Scavenging"], 0)
	sc.s.dly[0] = delayed{act: uint8(kop.OpSacrifice), ctrl: 0, obj: tok, ver: objWord(sc.s.objs[tok].ver), creator: NoObj}
	sc.s.ndly = 1
	sc.s.npend = 0
	sc.s.firePhase(sc.k, kop.StepEnd)
	sc.s.fireDelayedAt(sc.k, 0)
	var got []Obj
	for i := 0; i < int(sc.s.npend); i++ {
		got = append(got, sc.s.pend[i].obj)
	}
	if len(got) != 3 || got[0] != a || got[1] != tok || got[2] != b {
		t.Fatalf("end step queue %v, want [%d %d %d]", got, a, tok, b)
	}
}
