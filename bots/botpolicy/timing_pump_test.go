package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
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

// TestPumpHoldsInMain: the trick is held in the seat's own first main even
// with a combatant the bonus would help, and cast there with the rule off.
func TestPumpHoldsInMain(t *testing.T) {
	build := func(timing TimingRules) Board {
		b := withCreatures(pumpBoard(timing), map[state.ObjID]Creature{
			20: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
		})
		b.IsMain, b.FirstMain, b.Step = true, true, state.StepMain1
		return b
	}
	off, _ := castDecision(build(TimingRules{}), pumpOptions())
	if off != 0 {
		t.Fatalf("rule off: chose option %d, want the cast (0)", off)
	}
	on, _ := castDecision(build(TimingRules{PumpInCombat: true}), pumpOptions())
	if on != 1 {
		t.Fatalf("rule on: chose option %d, want the pass (1) in the first main", on)
	}
}

// TestPumpInCombatFiresSavingAttacker: the armed rule casts at declare
// blockers when the bonus saves an attacker that would otherwise die.
func TestPumpInCombatFiresSavingAttacker(t *testing.T) {
	b := pumpSaveBoard(TimingRules{PumpInCombat: true})
	if got, _ := castDecision(b, pumpOptions()); got != 0 {
		t.Fatalf("armed rule: chose option %d, want the saving pump cast (0)", got)
	}
	if id, ok := b.bestPumpTarget(0, 3, 3); !ok || id != 20 {
		t.Fatalf("bestPumpTarget = %d, %v, want the saving attacker 20", id, ok)
	}
}

// TestPumpHoldsUntilItMatters: an attacker already winning its combat is no
// reason to pump, so the armed rule holds; the zero rules still cast.
func TestPumpHoldsUntilItMatters(t *testing.T) {
	build := func(timing TimingRules) Board {
		return withCreatures(pumpBoard(timing), map[state.ObjID]Creature{
			20: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
			30: {Power: 1, Toughness: 1, Controller: 1, Blocks: 20},
		})
	}
	off, _ := castDecision(build(TimingRules{}), pumpOptions())
	if off != 0 {
		t.Fatalf("rule off: chose option %d, want the cast (0)", off)
	}
	on, d := castDecision(build(TimingRules{PumpInCombat: true}), pumpOptions())
	if d.Options[on].Kind != "pass" {
		t.Fatalf("rule on: chose %q, want the pass: the attacker already wins", d.Options[on].Kind)
	}
}

// TestPumpInCombatFiresSavingBlocker: a blocking 2/2 survives a 2/2 attacker
// only with the bonus, so the pump is cast. The board is the opponent's turn
// (the rule does not read MyTurn, but this is the stated scenario).
func TestPumpInCombatFiresSavingBlocker(t *testing.T) {
	b := withCreatures(pumpBoard(TimingRules{PumpInCombat: true}), map[state.ObjID]Creature{
		20: {Power: 2, Toughness: 2, Controller: 1, Attacking: true, Defender: 0},
		30: {Power: 2, Toughness: 2, Controller: 0, Blocks: 20},
	})
	b.MyTurn = false
	if got, _ := castDecision(b, pumpOptions()); got != 0 {
		t.Fatalf("armed rule: chose option %d, want the saving pump cast (0)", got)
	}
	if id, ok := b.bestPumpTarget(0, 3, 3); !ok || id != 30 {
		t.Fatalf("bestPumpTarget = %d, %v, want the saving blocker 30", id, ok)
	}
}

// TestPumpInCombatFiresKillingBlocker: a 2/2 attacker's +3/+3 kills a 0/4
// blocker it could not kill before (class 2). A 0/6 blocker survives the
// pumped attacker, so that board holds.
func TestPumpInCombatFiresKillingBlocker(t *testing.T) {
	build := func(timing TimingRules, toughness int32) Board {
		return withCreatures(pumpBoard(timing), map[state.ObjID]Creature{
			20: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
			30: {Power: 0, Toughness: toughness, Controller: 1, Blocks: 20},
		})
	}
	off, _ := castDecision(build(TimingRules{}, 4), pumpOptions())
	if off != 0 {
		t.Fatalf("rule off: chose option %d, want the cast (0)", off)
	}
	on, _ := castDecision(build(TimingRules{PumpInCombat: true}, 4), pumpOptions())
	if on != 0 {
		t.Fatalf("rule on, killable blocker: chose option %d, want the kill pump cast (0)", on)
	}
	if id, ok := build(TimingRules{PumpInCombat: true}, 4).bestPumpTarget(0, 3, 3); !ok || id != 20 {
		t.Fatalf("bestPumpTarget = %d, %v, want the killing attacker 20", id, ok)
	}
	held, d := castDecision(build(TimingRules{PumpInCombat: true}, 6), pumpOptions())
	if d.Options[held].Kind != "pass" {
		t.Fatalf("rule on, 0/6 blocker: chose %q, want the pass (the pumped attacker cannot kill it)", d.Options[held].Kind)
	}
}

// TestPumpInCombatFiresLethal: an unblocked 2-power attacker takes a
// 4-life defender from 2 to 5, so the pump is cast; a 10-life defender is
// out of reach and an already-lethal attack is not a reason to cast.
func TestPumpInCombatFiresLethal(t *testing.T) {
	lethal := func(life int32) Board {
		b := withCreatures(pumpBoard(TimingRules{PumpInCombat: true}), map[state.ObjID]Creature{
			20: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
		})
		b.Life.Set(1, life)
		return b
	}
	fires := lethal(4)
	if got, _ := castDecision(fires, pumpOptions()); got != 0 {
		t.Fatalf("armed rule: chose option %d, want the lethal pump cast (0)", got)
	}
	if id, ok := fires.bestPumpTarget(0, 3, 3); !ok || id != 20 {
		t.Fatalf("bestPumpTarget = %d, %v, want the lethal attacker 20", id, ok)
	}
	if got, d := castDecision(lethal(10), pumpOptions()); d.Options[got].Kind != "pass" {
		t.Fatalf("out of reach: chose %q, want the pass (1)", d.Options[got].Kind)
	}
	already := lethal(2)
	if got, d := castDecision(already, pumpOptions()); d.Options[got].Kind != "pass" {
		t.Fatalf("already lethal: chose %q, want the pass (1)", d.Options[got].Kind)
	}
	if id, ok := already.bestPumpTarget(0, 3, 3); ok {
		t.Fatalf("bestPumpTarget = %d, true, want no target: the attack is already lethal", id)
	}
}

// TestPumpDefendsAsBlocker: on the opponent's turn the blocker save fires at
// declare blockers, and the same board holds at declare attackers.
func TestPumpDefendsAsBlocker(t *testing.T) {
	build := func(timing TimingRules, step state.Step, blocks state.ObjID) Board {
		b := withCreatures(pumpBoard(timing), map[state.ObjID]Creature{
			20: {Power: 2, Toughness: 2, Controller: 0, Blocks: blocks},
			21: {Power: 3, Toughness: 3, Controller: 1, Attacking: true, Defender: 0},
		})
		b.MyTurn, b.Step = false, step
		return b
	}
	off, _ := castDecision(build(TimingRules{}, state.StepDeclareBlockers, 21), pumpOptions())
	if off != 0 {
		t.Fatalf("rule off: chose option %d, want the cast (0)", off)
	}
	on, _ := castDecision(build(TimingRules{PumpInCombat: true}, state.StepDeclareBlockers, 21), pumpOptions())
	if on != 0 {
		t.Fatalf("rule on, declare blockers: chose option %d, want the saving pump cast (0)", on)
	}
	held, d := castDecision(build(TimingRules{PumpInCombat: true}, state.StepDeclareAttackers, 0), pumpOptions())
	if d.Options[held].Kind != "pass" {
		t.Fatalf("rule on, declare attackers: chose %q, want the pass (outside declare blockers)", d.Options[held].Kind)
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

// TestPumpTargetsOwnCombatant: the brief's Giant Growth case. The ordinary
// ranking leads with the opposing creature; the armed rule names the seat's
// own saving combatant.
func TestPumpTargetsOwnCombatant(t *testing.T) {
	build := func(timing TimingRules) Board {
		b := withCreatures(pumpBoard(timing), map[state.ObjID]Creature{
			20: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
			21: {Power: 2, Toughness: 2, Controller: 1, Blocks: 20},
		})
		b.Stack = []StackEntry{{ID: 40, Controller: 0, IsSpell: true, PumpPower: 3, PumpToughness: 3}}
		return b
	}
	opts := []decision.Option{
		{Index: 0, Kind: "creature", Obj: 21},
		{Index: 1, Kind: "creature", Obj: 20},
	}
	target := func(b Board) int {
		d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1, Source: 40,
			TargetEffect: &decision.TargetEffect{API: "Pump"}, Options: opts}
		in := Decide(b, &d, rng(1))
		if len(in.Choices) != 1 {
			return -1
		}
		return in.Choices[0]
	}
	if got := target(build(TimingRules{})); got != 0 {
		t.Fatalf("rule off: chose option %d, want the opponent's creature (0)", got)
	}
	if got := target(build(TimingRules{PumpInCombat: true})); got != 1 {
		t.Fatalf("rule on: chose option %d, want the own saving combatant (1)", got)
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

// TestPumpTimedTapInCombat: with no mana floating the manual seat floats a
// Forest toward the saving pump at declare blockers (the rule-off window
// passes), and does not tap when the pump would not fire.
func TestPumpTimedTapInCombat(t *testing.T) {
	opts := []decision.Option{
		{Index: 0, Kind: "activate", Obj: 30},
		{Index: 1, Kind: "pass"},
	}
	build := func(timing TimingRules, save bool) Board {
		var b Board
		if save {
			b = pumpSaveBoard(timing)
		} else {
			b = withCreatures(pumpBoard(timing), map[state.ObjID]Creature{
				20: {Power: 2, Toughness: 2, Controller: 0},
			})
		}
		b.Pool[state.MG] = 0
		prod := cards.ManaProduction{}
		prod.Colour[state.MG] = 1
		b.Cards.Set(30, Card{OnBattlefield: true, Produces: prod})
		return b
	}
	off, d := castDecision(build(TimingRules{}, true), opts)
	if d.Options[off].Kind != "pass" {
		t.Fatalf("rule off: chose %q, want the pass (1) with no mana", d.Options[off].Kind)
	}
	on, d := castDecision(build(TimingRules{PumpInCombat: true}, true), opts)
	if d.Options[on].Kind != "activate" {
		t.Fatalf("rule on: chose %q, want the Forest activation (0)", d.Options[on].Kind)
	}
	held, d := castDecision(build(TimingRules{PumpInCombat: true}, false), opts)
	if d.Options[held].Kind != "pass" {
		t.Fatalf("rule on, no target: chose %q, want the pass (1)", d.Options[held].Kind)
	}
}

// TestPumpHeldGetsNoMainTap: the main-phase tap gate must not float mana
// toward a pump the armed rule holds. With the rule off the same window taps.
func TestPumpHeldGetsNoMainTap(t *testing.T) {
	opts := []decision.Option{
		{Index: 0, Kind: "activate", Obj: 30},
		{Index: 1, Kind: "pass"},
	}
	build := func(timing TimingRules) Board {
		b := withCreatures(pumpBoard(timing), map[state.ObjID]Creature{
			20: {Power: 2, Toughness: 2, Controller: 0, Attacking: true, Defender: 1},
		})
		b.IsMain, b.FirstMain, b.Step = true, true, state.StepMain1
		b.Pool[state.MG] = 0
		prod := cards.ManaProduction{}
		prod.Colour[state.MG] = 1
		b.Cards.Set(30, Card{OnBattlefield: true, Produces: prod})
		return b
	}
	off, d := castDecision(build(TimingRules{}), opts)
	if d.Options[off].Kind != "activate" {
		t.Fatalf("rule off: chose %q, want the Forest activation (0)", d.Options[off].Kind)
	}
	on, d := castDecision(build(TimingRules{PumpInCombat: true}), opts)
	if d.Options[on].Kind != "pass" {
		t.Fatalf("rule on: chose %q, want the pass (1): the held pump must not float mana", d.Options[on].Kind)
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
