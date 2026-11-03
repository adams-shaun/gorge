package rules

// Greatest-power SacValid pools and ConditionDefined$ groups reading derived
// power, driven through the resolution kernel (engine/rules/resolve).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGreatestPowerSacValidPoolReadsDerivedPower drives Consume's exact
// Sacrifice shape (`ValidTgts$ Player | SacValid$
// Creature.greatestPowerControlledByTargeted`) through effSacrifice's pool
// walk with a real LPT/SubModify +5/+5 pump on the 2/2 Grizzly Bears. The
// pumped bear is the derived-greatest (7 vs the Hill Giant's 3), so the pool
// must admit only the bear and sacrifice it.
func TestGreatestPowerSacValidPoolReadsDerivedPower(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	giantCard := mustCorpusCard(t, reg, "Hill Giant")
	bearCard := mustCorpusCard(t, reg, "Grizzly Bears")
	e := New(Config{Seed: 12, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	src := e.G.AddObject(giantCard, 0)
	giant := e.G.AddObject(giantCard, 1)
	bear := e.G.AddObject(bearCard, 1)
	for _, o := range []*state.Object{src, giant, bear} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	e.AddContinuous(state.ContinuousEffect{Source: bear.ID, Controller: 1, Layer: LPT, Sub: SubModify,
		Affects: "Creature.Self", AddPower: 5, AddToughness: 5})
	// Preconditions: both comparison creatures are battlefield permanents,
	// the pump really moved the derived read, and the printed faces still
	// order the other way -- otherwise the assertion cannot distinguish
	// derived from printed.
	if e.G.Obj(giant.ID).Zone != state.ZBattlefield || e.G.Obj(bear.ID).Zone != state.ZBattlefield {
		t.Fatal("precondition: both comparison creatures must be on the battlefield")
	}
	if e.Power(bear.ID) != 7 || e.Power(giant.ID) != 3 {
		t.Fatalf("precondition: derived powers bear=%d giant=%d, want 7 and 3",
			e.Power(bear.ID), e.Power(giant.ID))
	}
	if bp, gp := e.G.Obj(bear.ID).Face().Power(), e.G.Obj(giant.ID).Face().Power(); bp >= gp {
		t.Fatalf("precondition: printed powers bear=%d giant=%d, want bear < giant", bp, gp)
	}
	// Consume's own ability line, as an SA: the target player sacrifices the
	// creature with the greatest power among creatures they control.
	sa := &cards.SA{Kind: "SP", API: "Sacrifice", Line: "test:Consume", Params: map[string]string{
		"ValidTgts": "Player",
		"SacValid":  "Creature.greatestPowerControlledByTargeted",
	}}
	// The cast-time target answer, as the engine's own pre-ask would record
	// it (Ctx.SubPreAsk keyed by the SA line): the target player.
	e.probe(func() {
		effects.Resolve(e, &effects.Ctx{Source: src.ID, Controller: 0,
			Targets:   []state.Target{{Player: 1, IsPlayer: true}},
			SubPreAsk: map[string][]state.Target{sa.Line: {{Player: 1, IsPlayer: true}}}}, sa)
	})
	if z := e.G.Obj(bear.ID).Zone; z != state.ZGraveyard {
		t.Fatalf("derived-greatest Grizzly Bears zone = %s, want Graveyard (it must be the sacrifice)", z)
	}
	if z := e.G.Obj(giant.ID).Zone; z != state.ZBattlefield {
		t.Fatalf("Hill Giant zone = %s, want Battlefield (printed-greatest must not be chosen)", z)
	}
}

// TestGreatestPowerConditionDefinedGroupReadsDerivedPower drives Getaway
// Glamer's destroy condition (`ConditionDefined$ Targeted | ConditionPresent$
// Creature.greatestPower`) with a real LPT/SubModify +5/+5 pump on the 2/2
// Grizzly Bears so the pumped bear (derived 7) outranks the targeted 6/6
// Colossal Dreadmaw. The condition means "no other creature has greater
// power", so it must FAIL and the Dreadmaw must survive; a defined-group walk
// matching members with an unbound context would read the bear at printed 2,
// and destroyed the Dreadmaw. The unpumped positive control proves the
// resolution really reaches effDestroy and the condition really gates it (a
// test that passes only because the pre-ask suspended would be vacuous).
func TestGreatestPowerConditionDefinedGroupReadsDerivedPower(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	srcCard := mustCorpusCard(t, reg, "Hill Giant")
	bearCard := mustCorpusCard(t, reg, "Grizzly Bears")
	bigCard := mustCorpusCard(t, reg, "Colossal Dreadmaw")

	// board builds the two-creature board, optionally pumping the bear, and
	// resolves Getaway Glamer's destroy-mode SA targeting the Dreadmaw.
	board := func(pump bool) (*Engine, state.ObjID, state.ObjID) {
		t.Helper()
		e := New(Config{Seed: 12, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
		src := e.G.AddObject(srcCard, 0)
		bear := e.G.AddObject(bearCard, 1)
		big := e.G.AddObject(bigCard, 1)
		for _, o := range []*state.Object{src, bear, big} {
			e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		}
		if e.G.Obj(bear.ID).Zone != state.ZBattlefield || e.G.Obj(big.ID).Zone != state.ZBattlefield {
			t.Fatal("precondition: both comparison creatures must be on the battlefield")
		}
		if pump {
			e.AddContinuous(state.ContinuousEffect{Source: bear.ID, Controller: 1, Layer: LPT, Sub: SubModify,
				Affects: "Creature.Self", AddPower: 5, AddToughness: 5})
		}
		if bp, dp := e.G.Obj(bear.ID).Face().Power(), e.G.Obj(big.ID).Face().Power(); bp >= dp {
			t.Fatalf("precondition: printed powers bear=%d dreadmaw=%d, want bear < dreadmaw", bp, dp)
		}
		sa := &cards.SA{Kind: "SP", API: "Destroy", Line: "test:GetawayGlamer", Params: map[string]string{
			"ValidTgts":        "Creature",
			"ConditionDefined": "Targeted",
			"ConditionPresent": "Creature.greatestPower",
		}}
		e.probe(func() {
			effects.Resolve(e, &effects.Ctx{Source: src.ID, Controller: 0,
				Targets:   []state.Target{{Obj: big.ID}},
				SubPreAsk: map[string][]state.Target{sa.Line: {{Obj: big.ID}}}}, sa)
		})
		return e, bear.ID, big.ID
	}

	// Positive control: without the pump the Dreadmaw IS the greatest, so the
	// condition holds and the destroy lands -- proving the resolution reaches
	// effDestroy with the target bound.
	e0, _, big0 := board(false)
	if z := e0.G.Obj(big0).Zone; z != state.ZGraveyard {
		t.Fatalf("positive control: unpumped targeted Colossal Dreadmaw zone = %s, want Graveyard (condition must hold)", z)
	}

	// The pumped case: the bear's derived 7 > the Dreadmaw's 6, so the
	// condition must fail and the Dreadmaw must survive.
	e1, bear1, big1 := board(true)
	if e1.Power(bear1) != 7 || e1.Power(big1) != 6 {
		t.Fatalf("precondition: derived powers bear=%d dreadmaw=%d, want 7 and 6",
			e1.Power(bear1), e1.Power(big1))
	}
	if z := e1.G.Obj(big1).Zone; z != state.ZBattlefield {
		t.Fatalf("pumped targeted Colossal Dreadmaw zone = %s, want Battlefield (a pumped greater creature must defeat the condition)", z)
	}
}

// TestGreatestPowerCountsAnAnimatedVehicle: Getaway Glamer's destroy condition
// reads the derived type line, so a crewed Vehicle (a creature until end of
// turn, CR 301.7) is a creature in the comparison set and a legal member of
// it. Tied for the greatest power with the Grizzly Bears' pumped 3, the 3/3
// Vehicle is destroyed; read off its printed Artifact Vehicle line it matched
// nothing and survived.
func TestGreatestPowerCountsAnAnimatedVehicle(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	srcCard := mustCorpusCard(t, reg, "Hill Giant")
	bearCard := mustCorpusCard(t, reg, "Grizzly Bears")
	vehicleCard := mustCorpusCard(t, reg, "Mobile Homestead")
	for _, tc := range []struct {
		name    string
		bearAdd int32
		want    state.Zone
	}{
		{"tied for the greatest power", 1, state.ZGraveyard},
		{"out-powered", 2, state.ZBattlefield},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := New(Config{Seed: 12, Names: []string{"a", "b"},
				Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
			src := e.G.AddObject(srcCard, 0)
			bear := e.G.AddObject(bearCard, 1)
			vehicle := e.G.AddObject(vehicleCard, 1)
			for _, o := range []*state.Object{src, bear, vehicle} {
				e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
			}
			e.AddContinuous(state.ContinuousEffect{Source: vehicle.ID, Controller: 1, Layer: LType, Affects: "Card.Self",
				AddTypes: []string{"Creature"}, UntilEOT: true})
			e.AddContinuous(state.ContinuousEffect{Source: bear.ID, Controller: 1, Layer: LPT, Sub: SubModify,
				Affects: "Creature.Self", AddPower: tc.bearAdd, AddToughness: tc.bearAdd})
			if !e.IsCreature(vehicle.ID) || e.Power(vehicle.ID) != 3 || e.Power(bear.ID) != 2+tc.bearAdd {
				t.Fatalf("precondition: vehicle creature=%v power=%d, bear power=%d", e.IsCreature(vehicle.ID), e.Power(vehicle.ID), e.Power(bear.ID))
			}
			sa := &cards.SA{Kind: "SP", API: "Destroy", Line: "test:GetawayGlamer", Params: map[string]string{
				"ValidTgts":        "Creature",
				"ConditionDefined": "Targeted",
				"ConditionPresent": "Creature.greatestPower",
			}}
			e.probe(func() {
				effects.Resolve(e, &effects.Ctx{Source: src.ID, Controller: 0,
					Targets:   []state.Target{{Obj: vehicle.ID}},
					SubPreAsk: map[string][]state.Target{sa.Line: {{Obj: vehicle.ID}}}}, sa)
			})
			if z := e.G.Obj(vehicle.ID).Zone; z != tc.want {
				t.Fatalf("targeted animated Vehicle zone = %s, want %s", z, tc.want)
			}
		})
	}
}
