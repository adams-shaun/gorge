package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Rejecting an event that cannot trigger Ward must not allocate per visited
// object. Moving rejection back below keyword derivation regresses this even
// when no Ward ever fires, the common case during a whole-library scan.
func TestGrantedWardIrrelevantEventDoesNotAllocate(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Ward probe\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nK:Trample\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: id, Timestamp: 1, Layer: LAbilities,
		Affects: "Creature.YouCtrl", Controller: 0, AddKeywords: []string{"Ward:2"}, UntilEOT: true})
	o := e.G.Obj(id)
	for _, tc := range []struct {
		name string
		ev   events.Event
	}{
		{"unrelated_event", events.Event{Kind: events.Note, IDs: []state.ObjID{id}}},
		{"other_target", events.Event{Kind: events.TargetsChosen, IDs: []state.ObjID{id + 1}}},
		{"no_targets", events.Event{Kind: events.TargetsChosen}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(100, func() {
				e.checkGrantedWardTriggers(e, id, o, o.Face(), tc.ev, nil, 0, 0, false)
			})
			if allocs != 0 {
				t.Fatalf("irrelevant Ward event allocated %.0f objects; want zero", allocs)
			}
			if len(e.pendingTriggers) != 0 {
				t.Fatal("irrelevant event queued a Ward trigger")
			}
		})
	}
}

// A scan over cards with no matching triggers must reuse its traversal
// storage, not allocate a one-element face slice for each visited card.
func TestFaceTriggerScanDoesNotAllocatePerCard(t *testing.T) {
	e := layerEngine(t)
	allocs := testing.AllocsPerRun(100, func() {
		e.checkFaceTriggers(e, events.Event{Kind: events.Note}, nil, 0, 0, false, false, false)
	})
	if allocs != 0 {
		t.Fatalf("empty trigger scan allocated %.0f objects; want zero", allocs)
	}
}

func TestReplacementScanDoesNotAllocatePerCard(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Replacement probe\nTypes:Basic Land Forest\nOracle:x\n")
	ev := events.Event{Kind: events.Untap, Obj: id}
	allocs := testing.AllocsPerRun(100, func() {
		got, replaced := e.applyReplacements(ev)
		if replaced || got.Kind != events.Untap || got.Obj != id {
			t.Fatal("a permanent with no replacement changed the untap")
		}
	})
	if allocs != 0 {
		t.Fatalf("empty replacement scan allocated %.0f objects; want zero", allocs)
	}
}

// Phase diagnostics run for every event, not just phase changes. Once a
// spec has been checked, rescanning it must not allocate to parse it again.
func TestPhaseDiagnosticScanDoesNotReparse(t *testing.T) {
	for _, phase := range []string{"Upkeep", "NoSuchPhase"} {
		t.Run(phase, func(t *testing.T) {
			e := layerEngine(t)
			onBoard(t, e, 0, "Name:Phase probe\nTypes:Enchantment\nT:Mode$ Phase | Phase$ "+phase+" | Execute$ Gain\nSVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n")
			allocs := testing.AllocsPerRun(100, func() {
				e.checkFaceTriggers(e, events.Event{Kind: events.Note}, nil, 0, 0, false, false, false)
			})
			if allocs != 0 {
				t.Fatalf("repeated %q diagnostic scan allocated %.0f objects; want zero", phase, allocs)
			}
			if len(e.pendingTriggers) != 0 {
				t.Fatal("a Note event fired a Phase trigger")
			}
		})
	}
}

func TestPhaseDiagnosticMemoIsCloneIndependent(t *testing.T) {
	e := layerEngine(t)
	onBoard(t, e, 0, "Name:Known phase\nTypes:Enchantment\nT:Mode$ Phase | Phase$ Upkeep | Execute$ Gain\nSVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n")
	e.checkFaceTriggers(e, events.Event{Kind: events.Note}, nil, 0, 0, false, false, false)
	c := e.Clone()
	// Populating the clone's writable syntax cache must not populate the
	// parent's cache; a shared map would race in parallel search branches.
	c.parsedPhaseSpec("Draw")
	if _, shared := e.phaseSpecs["Draw"]; shared {
		t.Fatal("clone shares its writable phase syntax cache with the parent")
	}
	for _, branch := range []*Engine{e, c} {
		onBoard(t, branch, 0, "Name:Unknown phase\nTypes:Enchantment\nT:Mode$ Phase | Phase$ NoSuchPhase | Execute$ Gain\nSVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n")
		for range 2 {
			branch.checkFaceTriggers(branch, events.Event{Kind: events.Note}, nil, 0, 0, false, false, false)
		}
		count := 0
		for _, ev := range branch.L.Events {
			if ev.Kind == events.Note && strings.Contains(ev.Text, "NoSuchPhase names no engine step") {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("branch emitted %d diagnostics; want exactly one independently", count)
		}
	}
}
