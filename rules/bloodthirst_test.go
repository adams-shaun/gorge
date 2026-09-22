// kw:Bloodthirst (CR 702.54): "If an opponent was dealt damage this turn,
// this permanent enters the battlefield with N +1/+1 counters on it."
//
// The implementation lives wholly in rules/replacement.go: the entering
// permanent's DERIVED keyword list (printed K: line and layer-6 AddKeyword$
// grant alike) is read at MoveZone→Battlefield replacement collection and
// turned into one synthetic Updated entry replacement (bloodthirstEntryMatch),
// gated by the existing CheckSVar$/SVarCompare$ pair over the
// Count$DamageOppsTakenThisTurn head (effects/count.go). These pins run on
// real corpus carriers for every shape:
//
//   - Gorehorn Minotaurs, the printed K:Bloodthirst:2 carrier (both sides of
//     the condition).
//   - Petrified Wood-Kin, the printed K:Bloodthirst:X carrier (X is the
//     damage dealt to your opponents this turn, not the cast's own {X}).
//   - Twins of Discord, the AddKeyword$ GRANT carrier: its
//     `Affected$ Creature.Other+YouCtrl+Colorless` static is the shape the
//     primitive ratchet cannot see (Face.Primitives walks printed keywords
//     only), so the granted keyword working end to end is what the census
//     exists to catch.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// bloodthirstParamOf reads the entering object's derived bloodthirst param —
// the exact read bloodthirstEntryMatch performs.
func bloodthirstParamOf(e *Engine, id state.ObjID) (string, bool) {
	return e.derivedKeywordParam(id, "Bloodthirst")
}

// p1p1Of counts the +1/+1 counters an object carries after its entry.
func bloodthirstP1P1(t *testing.T, e *Engine, id state.ObjID) int32 {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("test precondition: object %d not on the battlefield (zone %+v)", id, o)
	}
	for _, c := range o.Counters {
		if c.Kind == "P1P1" {
			return c.N
		}
	}
	return 0
}

// damageOpponent records the per-turn damage history Bloodthirst's condition
// reads, through the real emit path, and asserts it landed.
func damageOpponent(t *testing.T, e *Engine, amount int32) {
	t.Helper()
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: amount})
	if got := e.DamageTakenThisTurn(1); got != amount {
		t.Fatalf("test precondition: opponent damage history = %d, want %d", got, amount)
	}
}

// TestBloodthirstPrintedEntersWithCounters pins the printed carrier both ways:
// without damage the entry places nothing, with damage it places exactly
// Bloodthirst's N and the counters are live P/T (not deferred past the entry).
func TestBloodthirstPrintedEntersWithCounters(t *testing.T) {
	t.Parallel()
	build := func(t *testing.T) (*Engine, state.ObjID) {
		t.Helper()
		e := handEngine(t, corpusAlternativeCard(t, "Gorehorn Minotaurs"))
		id := e.G.Zone(state.ZHand, 0)[0]
		if got, ok := bloodthirstParamOf(e, id); !ok || got != "2" {
			t.Fatalf("test precondition: derived bloodthirst param = %q,%v, want \"2\",true", got, ok)
		}
		return e, id
	}

	// Without damage: no counters, printed 3/3 stands.
	e, id := build(t)
	placeOnBattlefield(t, e, id)
	if n := bloodthirstP1P1(t, e, id); n != 0 {
		t.Fatalf("entry with no damage this turn placed %d +1/+1 counters, want 0", n)
	}
	if p, tp := e.Derived(id).Power, e.Derived(id).Toughness; p != 3 || tp != 3 {
		t.Fatalf("no-damage entry power/toughness = %d/%d, want 3/3", p, tp)
	}

	// With damage: exactly 2 counters, live P/T.
	e2, id2 := build(t)
	damageOpponent(t, e2, 2)
	placeOnBattlefield(t, e2, id2)
	if n := bloodthirstP1P1(t, e2, id2); n != 2 {
		t.Fatalf("entry with an opponent dealt 2 damage placed %d +1/+1 counters, want 2", n)
	}
	if p, tp := e2.Derived(id2).Power, e2.Derived(id2).Toughness; p != 5 || tp != 5 {
		t.Fatalf("bloodthirst entry power/toughness = %d/%d, want 5/5", p, tp)
	}
}

// TestBloodthirstXCountsDamageDealtToOpponents pins the X carrier: X is the
// damage dealt to YOUR OPPONENTS this turn — damage you took yourself does not
// count, and the count is the full dealt amount.
func TestBloodthirstXCountsDamageDealtToOpponents(t *testing.T) {
	t.Parallel()
	build := func(t *testing.T) (*Engine, state.ObjID) {
		t.Helper()
		e := handEngine(t, corpusAlternativeCard(t, "Petrified Wood-Kin"))
		id := e.G.Zone(state.ZHand, 0)[0]
		if got, ok := bloodthirstParamOf(e, id); !ok || got != "X" {
			t.Fatalf("test precondition: derived bloodthirst param = %q,%v, want \"X\",true", got, ok)
		}
		return e, id
	}

	// Damage only you took: X reads 0.
	e, id := build(t)
	e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 5})
	if got := e.DamageTakenThisTurn(0); got != 5 {
		t.Fatalf("test precondition: own damage history = %d, want 5", got)
	}
	placeOnBattlefield(t, e, id)
	if n := bloodthirstP1P1(t, e, id); n != 0 {
		t.Fatalf("X entry after damage only to yourself placed %d +1/+1 counters, want 0", n)
	}

	// Damage to the opponent: X is that amount.
	e2, id2 := build(t)
	damageOpponent(t, e2, 4)
	placeOnBattlefield(t, e2, id2)
	if n := bloodthirstP1P1(t, e2, id2); n != 4 {
		t.Fatalf("X entry after 4 damage to the opponent placed %d +1/+1 counters, want 4", n)
	}
	if p, tp := e2.Derived(id2).Power, e2.Derived(id2).Toughness; p != 7 || tp != 7 {
		t.Fatalf("bloodthirst-X entry power/toughness = %d/%d, want 7/7", p, tp)
	}
}

// TestBloodthirstGrantTwinsOfDiscord pins the AddKeyword$ GRANT path end to
// end: Twins of Discord's `S:Mode$ Continuous | Affected$
// Creature.Other+YouCtrl+Colorless | AddKeyword$ Bloodthirst:2` grants the
// keyword to every OTHER colorless creature you control, and such a creature
// enters with the granted counters. The grant is invisible to the primitive
// ratchet (Face.Primitives walks printed keywords only), so this test is the
// census's only defence for the shape.
func TestBloodthirstGrantTwinsOfDiscord(t *testing.T) {
	t.Parallel()
	build := func(t *testing.T) (*Engine, state.ObjID, state.ObjID, state.ObjID) {
		t.Helper()
		twins := corpusAlternativeCard(t, "Twins of Discord")
		brute := card(t, "Name:Colorless Brute\nManaCost:3\nTypes:Creature Eldrazi\nPT:2/2\nOracle:x\n")
		green := card(t, "Name:Green Brute\nManaCost:2G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
		e := handEngine(t, brute, green, twins)
		hand := e.G.Zone(state.ZHand, 0)
		bruteID, greenID, twinsID := hand[0], hand[1], hand[2]
		placeOnBattlefield(t, e, twinsID)
		if tw := e.G.Obj(twinsID); tw == nil || tw.Zone != state.ZBattlefield {
			t.Fatalf("test precondition: Twins of Discord not on the battlefield")
		}
		return e, bruteID, greenID, twinsID
	}

	// The grant itself: the colorless creature carries the derived keyword
	// while it waits in hand — the layer-6 walk is zone-blind for Affected
	// (Twins's static names no AffectedZone$), and the entry read runs on it.
	e, brute, green, _ := build(t)
	if got, ok := bloodthirstParamOf(e, brute); !ok || got != "2" {
		t.Fatalf("test precondition: granted bloodthirst param = %q,%v, want \"2\",true", got, ok)
	}
	// And the negative discriminator the Affected$ spec carries: a
	// NON-colorless creature you control is outside the grant.
	if _, ok := bloodthirstParamOf(e, green); ok {
		t.Fatalf("test precondition: a non-colorless creature must not be granted bloodthirst")
	}

	// With an opponent damaged, the granted carrier enters with 2 counters.
	damageOpponent(t, e, 1)
	placeOnBattlefield(t, e, brute)
	if n := bloodthirstP1P1(t, e, brute); n != 2 {
		t.Fatalf("granted bloodthirst entry placed %d +1/+1 counters, want 2", n)
	}

	// No damage: the same grant places nothing.
	e2, brute2, _, _ := build(t)
	placeOnBattlefield(t, e2, brute2)
	if n := bloodthirstP1P1(t, e2, brute2); n != 0 {
		t.Fatalf("granted bloodthirst entry with no damage placed %d +1/+1 counters, want 0", n)
	}
}
