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

// TestSteelExemplarConvergeCastEntersWithoutCounters is the cast half the
// CheckSVar$ X unshadowing unlocks: the face's real SVar:X body
// (Count$Converge) is evaluated through the machinery, so a cast whose
// payment spent two colours of mana takes no counters (LT2 fails), and a
// one-colour cast still takes both (the gate holds at converge 1).
func TestSteelExemplarConvergeCastEntersWithoutCounters(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Steel Exemplar"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MW], e.G.Players[0].Pool[state.MU] = 3, 1, 1
	castMode(t, e, id, "")
	finishCast(t, e, id)
	if got := e.G.Obj(id).Counter("P1P1"); got != 0 {
		t.Fatalf("two-colour-cast Steel Exemplar entered with %d P1P1, want 0 (Count$Converge reads 2: LT2 fails)", got)
	}
	if got := e.G.Obj(id).ConvergeColours; got != 2 {
		t.Fatalf("two-colour-cast Steel Exemplar recorded %d converge colours, want 2", got)
	}
}

func TestSteelExemplarOneColourCastEntersWithCounters(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Steel Exemplar"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MW] = 4, 1
	castMode(t, e, id, "")
	finishCast(t, e, id)
	if got := e.G.Obj(id).Counter("P1P1"); got != 2 {
		t.Fatalf("one-colour-cast Steel Exemplar entered with %d P1P1, want 2 (LT2 holds at converge 1)", got)
	}
}

// TestFreestriderCommandoCastWithManaEntersWithoutCounters is the cast half
// the CheckSVar$ X unshadowing plus the FlagManaSpent capture unlock: the
// face's real SVar:X body (Count$CastTotalManaSpent) reads the total mana
// the payment actually spent, so a cast paid with mana takes no counters
// (EQ0 fails). The un-cast half (2 counters) is the existing pin above.
func TestFreestriderCommandoCastWithManaEntersWithoutCounters(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Freestrider Commando"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MG] = 2, 1
	castMode(t, e, id, "")
	finishCast(t, e, id)
	if got := e.G.Obj(id).Counter("P1P1"); got != 0 {
		t.Fatalf("cast-with-mana Freestrider Commando entered with %d P1P1, want 0 (Count$CastTotalManaSpent reads 3: EQ0 fails)", got)
	}
	if got := e.G.Obj(id).ManaSpent; got != 3 {
		t.Fatalf("cast-with-mana Freestrider Commando recorded %d mana spent, want 3", got)
	}
}
