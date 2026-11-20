package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file pins the event-level shape of Food Chain's X on the mana-ability
// paid-exile path: the ManaAdd amount is 1 + the exiled card's mana value and
// the event carries the ability's creature-spell mana restriction. The
// mana_paid_exiled_test.go sibling pins the same rule through the priority
// activation and the pool; these cases go through the mana-ability entry
// directly and add the no-mana-cost (MV 0) fuel and the event text.
const (
	foodChainFuelThree = "Name:Food Chain Fuel Three\nManaCost:2 G\nTypes:Creature Beast\nPT:3/3\nOracle:x\n"
	foodChainFuelFree  = "Name:Food Chain Fuel Zero\nTypes:Creature Beast\nPT:0/1\nOracle:x\n"
)

// activateFoodChainExiling activates Food Chain's mana ability directly,
// answers the Exile<1/Creature> cost with victim and the Produced$ Any colour
// ask with white, and returns the single ManaAdd event the activation emitted.
func activateFoodChainExiling(t *testing.T, e *Engine, chain, victim state.ObjID) events.Event {
	t.Helper()
	ma := e.G.Obj(chain).Face().Abilities[0]
	from := len(e.L.Events)
	e.resolveManaAbility(0, chain, ma, false)

	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("exile cost ask = %+v, want a KChoose", d)
	}
	choice := -1
	for _, opt := range d.Options {
		if opt.Kind == "mana_exile" && opt.Obj == victim {
			choice = opt.Index
		}
	}
	if choice < 0 {
		t.Fatalf("victim %d was not offered by the exile ask: %+v", victim, d.Options)
	}
	submitChoices(t, e, choice)
	if o := e.G.Obj(victim); o == nil || o.Zone != state.ZExile {
		t.Fatalf("precondition: the exiled fuel is not in exile after payment: %+v", o)
	}

	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("colour ask after the exile cost = %+v, want a KChoose", d)
	}
	pick := -1
	for _, opt := range d.Options {
		if opt.Kind == "mana" && opt.ManaSymbol == "W" {
			pick = opt.Index
		}
	}
	if pick < 0 {
		t.Fatalf("colour ask offers no white mana: %+v", d.Options)
	}
	submitChoices(t, e, pick)

	var adds []events.Event
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.ManaAdd && ev.Player == 0 {
			adds = append(adds, ev)
		}
	}
	if len(adds) != 1 {
		t.Fatalf("activation emitted %d ManaAdd events, want exactly 1: %+v", len(adds), adds)
	}
	return adds[0]
}

// TestFoodChainPaidExileSizesTheManaAddEvent checks the reported symptom on
// the event itself. The two fuels have different mana values (3 and 0), so the
// expected amounts (4 and 1) also differ from each other and from the broken
// constant 1.
func TestFoodChainPaidExileSizesTheManaAddEvent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		fuel   string
		wantMV int32
	}{
		{name: "mana value three", fuel: foodChainFuelThree, wantMV: 3},
		{name: "mana value zero", fuel: foodChainFuelFree, wantMV: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := smallLibraryEngine(t, 1)
			chain := onBoardCard(t, e, 0, corpusAlternativeCard(t, "Food Chain"))
			fuel := onBoard(t, e, 0, tc.fuel)
			if o := e.G.Obj(chain); o == nil || o.Zone != state.ZBattlefield {
				t.Fatalf("precondition: Food Chain zone = %+v, want battlefield", o)
			}
			if o := e.G.Obj(fuel); o == nil || o.Zone != state.ZBattlefield {
				t.Fatalf("precondition: fuel zone = %+v, want battlefield", o)
			}
			if got := e.G.Obj(fuel).Face().ManaValue(); got != tc.wantMV {
				t.Fatalf("precondition: fuel mana value = %d, want %d", got, tc.wantMV)
			}
			if pool := e.G.Players[0].Pool; pool != (state.Mana{}) {
				t.Fatalf("precondition: mana pool is not empty: %+v", pool)
			}

			add := activateFoodChainExiling(t, e, chain, fuel)
			if add.Amount != tc.wantMV+1 {
				t.Fatalf("ManaAdd amount = %d, want 1 + MV %d = %d", add.Amount, tc.wantMV, tc.wantMV+1)
			}
			if !strings.Contains(add.Text, "mana-restriction:Spell.Creature") {
				t.Fatalf("ManaAdd text = %q, want the creature-spell mana restriction", add.Text)
			}
		})
	}
}

// TestFoodChainSecondActivationDoesNotReuseTheFirstExileList activates Food
// Chain twice in one engine, exiling an MV-3 card and then an MV-0 card. A
// paid list left over from the first activation would price the second as 4
// instead of 1.
func TestFoodChainSecondActivationDoesNotReuseTheFirstExileList(t *testing.T) {
	t.Parallel()
	e := smallLibraryEngine(t, 1)
	chain := onBoardCard(t, e, 0, corpusAlternativeCard(t, "Food Chain"))
	big := onBoard(t, e, 0, foodChainFuelThree)
	free := onBoard(t, e, 0, foodChainFuelFree)
	if o := e.G.Obj(chain); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Food Chain zone = %+v, want battlefield", o)
	}
	if o := e.G.Obj(big); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: first fuel zone = %+v, want battlefield", o)
	}
	if o := e.G.Obj(free); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: second fuel zone = %+v, want battlefield", o)
	}
	bigMV, freeMV := e.G.Obj(big).Face().ManaValue(), e.G.Obj(free).Face().ManaValue()
	if bigMV != 3 || freeMV != 0 || bigMV == freeMV {
		t.Fatalf("precondition: fuel mana values = %d and %d, want distinct 3 and 0", bigMV, freeMV)
	}
	if pool := e.G.Players[0].Pool; pool != (state.Mana{}) {
		t.Fatalf("precondition: mana pool is not empty: %+v", pool)
	}

	if add := activateFoodChainExiling(t, e, chain, big); add.Amount != 4 {
		t.Fatalf("first ManaAdd amount = %d, want 4", add.Amount)
	}
	if add := activateFoodChainExiling(t, e, chain, free); add.Amount != 1 {
		t.Fatalf("second ManaAdd amount = %d, want 1 (no stale paid list)", add.Amount)
	}
	if got := e.G.Players[0].Pool[state.MW]; got != 5 {
		t.Fatalf("white pool = %d, want 4 + 1 = 5", got)
	}
}
