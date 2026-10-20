package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// pumpBoard is the A6a base board: a castable {G} instant pump (card 10,
// +3/+3) in hand, the given creatures and stack, and one green mana. It is
// never a main phase, so the ordinary tap gate is not what lets the cast
// through -- every OFF/ON difference is the timing rule.
func pumpBoard(timing TimingRules) Board {
	b := Board{MyTurn: true, IsMain: false, Step: state.StepDeclareBlockers, Timing: timing,
		Cards: TableOf(map[state.ObjID]Card{
			10: {InstantSpeed: true, CMC: 1, ManaCost: "G", Castable: true, PumpPower: 3, PumpToughness: 3},
		}),
	}
	b.Pool[state.MG] = 1
	b.Life.Set(0, 20)
	b.Life.Set(1, 20)
	return b
}

func pumpOptions() []decision.Option {
	return []decision.Option{
		{Index: 0, Kind: "cast", Obj: 10},
		{Index: 1, Kind: "pass"},
	}
}

func withCreatures(b Board, crs map[state.ObjID]Creature) Board {
	b.Creatures = TableOf(crs)
	return b
}

// pumpSaveBoard is the canonical save: a 2/2 attacker is blocked by a 2/2,
// so both die unpumped and the +3/+3 keeps the attacker alive (class 3).
func pumpSaveBoard(timing TimingRules) Board {
	return withCreatures(pumpBoard(timing), map[state.ObjID]Creature{
		20: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
		30: {Power: 2, Toughness: 2, Controller: 1, Blocks: 20},
	})
}

// TestPumpInCombatHoldsWithoutTarget is the firing boundary: the armed rule
// holds the pump when no combatant benefits, and the zero rules cast it.
func TestPumpInCombatHoldsWithoutTarget(t *testing.T) {
	idle := func(timing TimingRules) Board {
		return withCreatures(pumpBoard(timing), map[state.ObjID]Creature{
			20: {Power: 2, Toughness: 2, Controller: 0},
		})
	}
	off, d := castDecision(idle(TimingRules{}), pumpOptions())
	if d.Options[off].Kind != "cast" {
		t.Fatalf("rule off: chose %q, want the cast (0)", d.Options[off].Kind)
	}
	on, d := castDecision(idle(TimingRules{PumpInCombat: true}), pumpOptions())
	if d.Options[on].Kind != "pass" {
		t.Fatalf("rule on: chose %q, want the pass (1) with no worthwhile target", d.Options[on].Kind)
	}
}

// TestPumpInCombatFiresSavingAttacker: the armed rule casts at declare
// blockers when the bonus saves an attacker that would otherwise die.
func TestPumpInCombatFiresSavingAttacker(t *testing.T) {
	b := pumpSaveBoard(TimingRules{PumpInCombat: true})
	if got, _ := castDecision(b, pumpOptions()); got != 0 {
		t.Fatalf("armed rule: chose option %d, want the saving pump cast (0)", got)
	}
	if _, ok := b.bestPumpTarget(0, 3, 3); !ok {
		t.Fatal("bestPumpTarget found no save")
	}
}

// TestPumpInCombatFiresSavingBlocker: a blocking 2/2 survives a 2/2 attacker
// only with the bonus, so the pump is cast.
func TestPumpInCombatFiresSavingBlocker(t *testing.T) {
	b := withCreatures(pumpBoard(TimingRules{PumpInCombat: true}), map[state.ObjID]Creature{
		20: {Power: 2, Toughness: 2, Controller: 1, Attacking: true, Defender: 0},
		30: {Power: 2, Toughness: 2, Controller: 0, Blocks: 20},
	})
	if got, _ := castDecision(b, pumpOptions()); got != 0 {
		t.Fatalf("armed rule: chose option %d, want the saving pump cast (0)", got)
	}
}

// TestPumpInCombatFiresKillingBlocker: a 3/5 attacker survives a 3/4 blocker
// either way, but the +3/+3 kills the blocker -- a class-2 reason to cast.
func TestPumpInCombatFiresKillingBlocker(t *testing.T) {
	b := withCreatures(pumpBoard(TimingRules{PumpInCombat: true}), map[state.ObjID]Creature{
		20: {Power: 3, Toughness: 5, Controller: 0, Attacking: true, Defender: 1},
		30: {Power: 3, Toughness: 4, Controller: 1, Blocks: 20},
	})
	if got, _ := castDecision(b, pumpOptions()); got != 0 {
		t.Fatalf("armed rule: chose option %d, want the kill pump cast (0)", got)
	}
	if class := b.pumpClass(0, 20, b.Creatures.Get(20), Creature{Power: 6, Toughness: 8, Controller: 0, Attacking: true, Defender: 1}); class != pumpClassKill {
		t.Fatalf("pumpClass = %d, want the kill class (%d)", class, pumpClassKill)
	}
}

// TestPumpInCombatFiresLethal: an unblocked 2-power attacker takes a
// 3-life defender from 2 to 5, so the pump is cast; an attack already at or
// above the defender's life is not a reason to cast.
func TestPumpInCombatFiresLethal(t *testing.T) {
	lethal := func(power int32) Board {
		b := withCreatures(pumpBoard(TimingRules{PumpInCombat: true}), map[state.ObjID]Creature{
			20: {Power: power, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
		})
		b.Life.Set(1, 3)
		return b
	}
	if got, _ := castDecision(lethal(2), pumpOptions()); got != 0 {
		t.Fatalf("armed rule: chose option %d, want the lethal pump cast (0)", got)
	}
	if got, _ := castDecision(lethal(3), pumpOptions()); got != 1 {
		t.Fatalf("already lethal: chose option %d, want the pass (1)", got)
	}
}

// TestPumpInCombatHoldsOutsideDeclareBlockers: the pump is a combat trick, so
// the armed rule holds it in any other step even when a target would benefit.
func TestPumpInCombatHoldsOutsideDeclareBlockers(t *testing.T) {
	b := pumpSaveBoard(TimingRules{PumpInCombat: true})
	b.Step = state.StepMain2
	if got, d := castDecision(b, pumpOptions()); d.Options[got].Kind != "pass" {
		t.Fatalf("chose %q, want the pass outside declare blockers", d.Options[got].Kind)
	}
}

// TestPumpInCombatHoldsWhenOwnSpellOnStack: a second pump while the seat's
// own spell is unresolved cannot affect this combat, so it is held.
func TestPumpInCombatHoldsWhenOwnSpellOnStack(t *testing.T) {
	b := pumpSaveBoard(TimingRules{PumpInCombat: true})
	b.Stack = []StackEntry{{ID: 40, Controller: 0, IsSpell: true}}
	if got, d := castDecision(b, pumpOptions()); d.Options[got].Kind != "pass" {
		t.Fatalf("chose %q, want the pass with an own spell on the stack", d.Options[got].Kind)
	}
}

// TestPumpInCombatHoldsForIndestructible: damage cannot kill an
// indestructible attacker, so neither the save nor the kill class applies
// and the pump is held.
func TestPumpInCombatHoldsForIndestructible(t *testing.T) {
	b := withCreatures(pumpBoard(TimingRules{PumpInCombat: true}), map[state.ObjID]Creature{
		20: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1, Keywords: []string{"Indestructible"}},
		30: {Power: 2, Toughness: 2, Controller: 1, Blocks: 20},
	})
	if got, d := castDecision(b, pumpOptions()); d.Options[got].Kind != "pass" {
		t.Fatalf("chose %q, want the pass when the bonus cannot save or kill", d.Options[got].Kind)
	}
}

// TestPumpInCombatTargetsBestClass: the target half picks the highest-value
// combatant (save over kill over lethal, then threat, then lowest id), not
// the ordinary removal ranking's target.
func TestPumpInCombatTargetsBestClass(t *testing.T) {
	build := func(timing TimingRules) Board {
		b := pumpBoard(timing)
		b.Stack = []StackEntry{{ID: 60, Controller: 0, IsSpell: true, PumpPower: 3, PumpToughness: 3}}
		b.Creatures = TableOf(map[state.ObjID]Creature{
			20: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
			30: {Power: 2, Toughness: 2, Controller: 1, Blocks: 20},
			40: {Power: 3, Toughness: 5, Controller: 0, Attacking: true, Defender: 1},
			50: {Power: 3, Toughness: 4, Controller: 1, Blocks: 40},
		})
		return b
	}
	opts := []decision.Option{
		{Index: 0, Kind: "creature", Obj: 40},
		{Index: 1, Kind: "creature", Obj: 20},
	}
	target := func(b Board) int {
		d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1, Source: 60,
			TargetEffect: &decision.TargetEffect{API: "Pump"}, Options: opts}
		in := Decide(b, &d, rng(1))
		if len(in.Choices) != 1 {
			return -1
		}
		return in.Choices[0]
	}
	if got := target(build(TimingRules{})); got != 0 {
		t.Fatalf("rule off: chose option %d, want the ordinary ranking's first option (0)", got)
	}
	if got := target(build(TimingRules{PumpInCombat: true})); got != 1 {
		t.Fatalf("rule on: chose option %d, want the save target (1)", got)
	}
}

// TestPumpInCombatTargetPicksLowestIDOnTie: equal class and threat break on
// the lowest object id, independent of the option order.
func TestPumpInCombatTargetPicksLowestIDOnTie(t *testing.T) {
	b := pumpBoard(TimingRules{PumpInCombat: true})
	b.Stack = []StackEntry{{ID: 60, Controller: 0, IsSpell: true, PumpPower: 3, PumpToughness: 3}}
	b.Creatures = TableOf(map[state.ObjID]Creature{
		20: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
		30: {Power: 2, Toughness: 2, Controller: 1, Blocks: 20},
		22: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
		32: {Power: 2, Toughness: 2, Controller: 1, Blocks: 22},
	})
	if id, ok := b.bestPumpTarget(0, 3, 3); !ok || id != 20 {
		t.Fatalf("bestPumpTarget = %d, %v, want the lowest id 20", id, ok)
	}
}

// TestPumpInCombatDefaultUntouched pins the opt-in boundary: the zero
// TimingRules casts exactly as it did before the rule existed.
func TestPumpInCombatDefaultUntouched(t *testing.T) {
	b := pumpSaveBoard(TimingRules{})
	if got, d := castDecision(b, pumpOptions()); d.Options[got].Kind != "cast" {
		t.Fatalf("zero TimingRules chose %q, want the cast", d.Options[got].Kind)
	}
}
