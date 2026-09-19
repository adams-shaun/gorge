package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The K:etbCounter expansion's CheckSVar$ gate family, pinned on real corpus
// carriers in both directions:
//
//   - Myojin of Night's Reach's `CheckSVar$ FromHand` gates on the
//     Count$wasCastFromYourHandByYou branch head (Host.WasCastFromHandByYou's
//     log scan over the cast's PutOnStack): a cast-from-hand Myojin enters
//     with its divinity counter, one cheated into play does not. Before the
//     head was registered the gate resolved to 0 and NEVER held — the
//     round-2 regression this file pins the fix for.
//   - Hotheaded Giant and Freestrider Commando carry a
//     `CheckSVar$ <name> | SVarCompare$ <op><N>` gate field: the split into
//     CheckSVar + SVarCompare params is what makes the gate readable at all
//     (a whole-stuffed CheckSVar resolved no SVar and failed closed).
//
// The entries are placed by a raw hand→battlefield MoveZone emit — the
// un-cast entry every gate must deny or grant on its own provenance.

// placeFromHand emits the raw hand→battlefield move (the fixture entry) and
// drains any trigger the move queues, the zallDrain pattern.
func placeFromHand(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	for i := 0; i < 50; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		if len(e.G.Stack) == 0 {
			return
		}
		e.resolveTop()
	}
	t.Fatalf("entry drain did not settle: %d pending, %d on the stack", len(e.pendingTriggers), len(e.G.Stack))
}

func TestMyojinCastFromHandEntersWithDivinity(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Myojin of Night's Reach"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 5, 3
	castMode(t, e, id, "")
	finishCast(t, e, id)
	myojin := e.G.Obj(id)
	if myojin.Zone != state.ZBattlefield {
		t.Fatalf("cast Myojin in %s, want battlefield", myojin.Zone)
	}
	if got := myojin.Counter("DIVINITY"); got != 1 {
		t.Fatalf("cast-from-hand Myojin entered with %d divinity, want 1 (the CheckSVar$ FromHand gate must hold)", got)
	}
}

func TestMyojinNotCastFromHandEntersWithoutDivinity(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Myojin of Night's Reach"))
	id := e.G.Zone(state.ZHand, 0)[0]
	placeFromHand(t, e, id)
	myojin := e.G.Obj(id)
	if myojin.Zone != state.ZBattlefield {
		t.Fatalf("cheated Myojin in %s, want battlefield", myojin.Zone)
	}
	if got := myojin.Counter("DIVINITY"); got != 0 {
		t.Fatalf("cheated-into-play Myojin entered with %d divinity, want 0 (the oracle's 'if you cast it from your hand' half)", got)
	}
}

func TestHotheadedGiantNoRedCastEntersWithCounters(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Hotheaded Giant"))
	id := e.G.Zone(state.ZHand, 0)[0]
	placeFromHand(t, e, id)
	if got := e.G.Obj(id).Counter("M1M1"); got != 2 {
		t.Fatalf("Hotheaded Giant entered with %d M1M1, want 2 (no red spell cast: SVarCompare$ EQ0 holds)", got)
	}
}

func TestFreestriderCommandoUncastEntersWithCounters(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Freestrider Commando"))
	id := e.G.Zone(state.ZHand, 0)[0]
	placeFromHand(t, e, id)
	if got := e.G.Obj(id).Counter("P1P1"); got != 2 {
		t.Fatalf("un-cast Freestrider Commando entered with %d P1P1, want 2 (CheckSVar$ X reads 0: SVarCompare$ EQ0 holds)", got)
	}
}

func TestSteelExemplarUncastEntersWithCounters(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Steel Exemplar"))
	id := e.G.Zone(state.ZHand, 0)[0]
	placeFromHand(t, e, id)
	if got := e.G.Obj(id).Counter("P1P1"); got != 2 {
		t.Fatalf("un-cast Steel Exemplar entered with %d P1P1, want 2 (SVarCompare$ LT2 holds at 0)", got)
	}
}
