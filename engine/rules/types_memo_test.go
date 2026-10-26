package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// typesMemoAgrees asserts the memo-served layer-4 answer for id equals an
// uncached walk, and returns it.
func typesMemoAgrees(t *testing.T, e *Engine, id state.ObjID) []string {
	t.Helper()
	got, ok := e.layer4Top(id)
	want := e.layer4Fresh(id)
	if !ok {
		// Only a board with no live layer-4 effect bypasses the memo at a
		// top-level read (the walk is then the printed-list fast path).
		if _, live := e.layer4Local(e.active()); live {
			t.Fatalf("layer4Top(%d) not memo-eligible at a top-level read with a layer-4 effect live", id)
		}
		got = want
	}
	if !slices.Equal(got.Types, want.Types) || (got.Types == nil) != (want.Types == nil) ||
		got.AllCreatureTypes != want.AllCreatureTypes || got.LandSet != want.LandSet {
		t.Fatalf("obj %d: memo %+v, fresh %+v", id, got, want)
	}
	if d := e.derivedComputeFresh(id, 0).Types; !slices.Equal(got.Types, d) {
		t.Fatalf("obj %d: memo types %v, full Derived walk %v", id, got.Types, d)
	}
	return got.Types
}

// TestTypesMemoInvalidation pins types_memo.go's key: every change that can
// move an object's layer-4 answer -- a type-granting registration and its
// removal, a counter that flips a counters_ gate (including the transparent
// rebuild that keeps derivedSeq and retires only the touched object), a zone
// change -- is seen by the next read, while an unchanged board is served from
// the memo without a walk or an allocation.
func TestTypesMemoInvalidation(t *testing.T) {
	e := layerEngine(t)
	rock := onBoard(t, e, 0, "Name:Rock\nManaCost:2\nTypes:Artifact\nOracle:x\n")
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	lord := onBoard(t, e, 1, "Name:Lord\nManaCost:2\nTypes:Artifact\nOracle:x\n")
	e.emit(events.Event{Kind: events.Priority, Player: 0})

	if e.IsCreature(rock) || !e.IsCreature(bear) {
		t.Fatalf("printed: rock creature %v, bear creature %v", e.IsCreature(rock), e.IsCreature(bear))
	}

	// A local layer-4 registration from another permanent: artifacts with a
	// charge counter are creatures (the station shape, sourced elsewhere so a
	// counter on the rock is an object-local event the transparent rebuild
	// admits).
	e.AddContinuous(state.ContinuousEffect{Source: lord, Affects: "Artifact.counters_GE1_CHARGE", Layer: state.LType, AddTypes: []string{"Creature"}})
	e.emit(events.Event{Kind: events.Priority, Player: 0})
	if e.IsCreature(rock) {
		t.Fatal("rock is a creature without a charge counter")
	}
	typesMemoAgrees(t, e, rock)
	m := e.typesMemo.at(rock)
	if m == nil || m.seq == 0 || m.seq != e.derivedSeq {
		t.Fatalf("rock's answer was not memoized: %+v (derivedSeq %d)", m, e.derivedSeq)
	}

	// An unchanged board is a hit: no walk, no allocation.
	if allocs := testing.AllocsPerRun(100, func() { _ = e.IsCreature(rock) }); allocs != 0 {
		t.Fatalf("memo hit allocates %.1f per read", allocs)
	}
	// A layer-inert marker keeps the entry.
	seq := e.derivedSeq
	e.emit(events.Event{Kind: events.Priority, Player: 1})
	e.active()
	if e.derivedSeq != seq || e.typesMemo.at(rock).seq != seq {
		t.Fatalf("a Priority marker moved the key: derivedSeq %d -> %d", seq, e.derivedSeq)
	}

	// A charge counter on the rock: an object-local event. When the
	// rebuild is transparent derivedSeq stays and only the rock's entry is
	// retired; either way the next read sees the new type.
	typesMemoAgrees(t, e, bear)
	e.emit(events.Event{Kind: events.CounterChange, Obj: rock, Counter: "CHARGE", Amount: 1})
	if !e.IsCreature(rock) {
		t.Fatal("rock with a charge counter is not a creature: stale memo")
	}
	typesMemoAgrees(t, e, rock)
	t.Logf("counter rebuild transparent: %v", e.derivedSeq == seq)
	if e.derivedSeq == seq {
		// The transparent path ran: the bear's entry survived it.
		if bm := e.typesMemo.at(bear); bm == nil || bm.seq != seq {
			t.Fatalf("transparent rebuild retired an untouched object's entry: %+v", bm)
		}
	}
	typesMemoAgrees(t, e, bear)

	// Removing the registration (no event: a registry write) moves
	// continuousVersion.
	stamp := e.continuous[len(e.continuous)-1].Timestamp
	e.EndEffect(lord, stamp)
	if e.IsCreature(rock) {
		t.Fatal("rock is still a creature after the grant ended: stale memo")
	}
	typesMemoAgrees(t, e, rock)

	// A self-scoped grant on the rock itself, then the rock leaves the
	// battlefield: the zone change is seen.
	e.AddContinuous(state.ContinuousEffect{Source: rock, Affects: "Card.Self", Layer: state.LType, AddTypes: []string{"Creature", "Golem"}})
	if got := typesMemoAgrees(t, e, rock); !slices.Contains(got, "Golem") {
		t.Fatalf("self grant not applied: %v", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: rock, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := typesMemoAgrees(t, e, rock); slices.Contains(got, "Golem") {
		t.Fatalf("graveyard rock kept its battlefield grant: %v", got)
	}

	// A nested read (inside a derivation) never touches the memo.
	e.charsWalk.Depth++
	if _, ok := e.layer4Top(bear); ok {
		t.Fatal("layer4Top served a read nested in a derivation")
	}
	e.charsWalk.Depth--

	// A clone starts with an empty memo and answers the same.
	c := e.Clone()
	if c.typesMemo.at(bear) != nil {
		t.Fatal("clone inherited the memo")
	}
	if got, want := typesMemoAgrees(t, c, bear), typesMemoAgrees(t, e, bear); !slices.Equal(got, want) {
		t.Fatalf("clone %v, original %v", got, want)
	}
}

// TestAffectsSelfBound pins the classification behind matchesWithCharsPT's
// self-scoped early reject.
func TestAffectsSelfBound(t *testing.T) {
	t.Parallel()
	for spec, want := range map[string]bool{
		"Card.Self":                         true,
		"Card.Self+counters_GE4_CHARGE":     true,
		"Creature.Self+YouCtrl":             true,
		"Card.Self,Creature.Self+token":     true,
		"Card.Self,Creature.YouCtrl":        false,
		"Creature.YouCtrl":                  false,
		"Creature.Other+YouCtrl":            false,
		"Creature.EnchantedBy":              false,
		"Card.!Self":                        false,
		"Card.nonSelf":                      false,
		"Creature.SelfDestruct":             false,
		"":                                  false,
		"Card.Self+namedSelf, the Wanderer": false,
	} {
		if got := affectsSelfBound(spec); got != want {
			t.Errorf("affectsSelfBound(%q) = %v, want %v", spec, got, want)
		}
		// The memoized answer is the same.
		if got := affectsSelfBound(spec); got != want {
			t.Errorf("memoized affectsSelfBound(%q) = %v, want %v", spec, got, want)
		}
	}
}
