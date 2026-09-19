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

// TestHotheadedGiantAnotherRedCastEntersWithoutCounters is the red-cast half
// the bare !CastSaSource exclusion unlocks: the gate's
// Count$ThisTurnCast_Card.Red+!CastSaSource+YouCtrl counts red spells cast
// this turn OTHER than the giant's own resolving cast, so casting another red
// spell first makes EQ0 fail and the giant enters bare.
func TestHotheadedGiantAnotherRedCastEntersWithoutCounters(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Hotheaded Giant"))
	ritual := e.G.AddObject(corpusAlternativeCard(t, "Desperate Ritual"), 0)
	ritual.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), ritual.ID))
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MR] = 1, 1
	castMode(t, e, ritual.ID, "")
	finishCast(t, e, ritual.ID)
	if n := len(e.G.Zone(state.ZHand, 0)); n != 1 {
		t.Fatalf("fixture hand after the red cast = %d, want 1 (the giant)", n)
	}
	giant := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MR] = 3, 1
	castMode(t, e, giant, "")
	finishCast(t, e, giant)
	if got := e.G.Obj(giant).Counter("M1M1"); got != 0 {
		t.Fatalf("Hotheaded Giant entered with %d M1M1 after another red spell, want 0 (EQ0 must fail)", got)
	}
}

// TestHotheadedGiantOwnCastAloneStillEntersWithCounters is the exclusion
// proof: the gate's count excludes the giant's own resolving cast, so casting
// the giant with NO other red spell still grants the two counters (the
// oracle's "another"). Without the exclusion the giant's own red PutOnStack
// counted 1 and the counters were never granted.
func TestHotheadedGiantOwnCastAloneStillEntersWithCounters(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Hotheaded Giant"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MR] = 3, 1
	castMode(t, e, id, "")
	finishCast(t, e, id)
	if got := e.G.Obj(id).Counter("M1M1"); got != 2 {
		t.Fatalf("lone-cast Hotheaded Giant entered with %d M1M1, want 2 (the count excludes its own cast)", got)
	}
}

// TestDreamThiefAnotherBlueCastDraws pins the same exclusion on a trigger
// condition: Dream Thief's ETB trigger gates on
// Count$ThisTurnCast_Card.!CastSaSource+Blue+YouCtrl ("draw a card if you've
// cast another blue spell this turn"), so casting another blue spell first
// makes the draw happen, while...
func TestDreamThiefAnotherBlueCastDraws(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Dream Thief"))
	drake := e.G.AddObject(corpusAlternativeCard(t, "Wind Drake"), 0)
	drake.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), drake.ID))
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = 2, 1
	castMode(t, e, drake.ID, "")
	finishCast(t, e, drake.ID)
	thief := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = 2, 1
	n0 := len(e.L.Events)
	castMode(t, e, thief, "")
	finishCast(t, e, thief)
	drainQueuedTriggers(t, e)
	draws := 0
	for _, ev := range e.L.Events[n0:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			draws++
		}
	}
	if draws != 1 {
		t.Fatalf("Dream Thief ETB drew %d cards after another blue spell, want 1 (the exclusion leaves the drake's cast counted)", draws)
	}
}

// TestDreamThiefOwnCastAloneDoesNotDraw is the mirror: the thief's own blue
// cast is excluded by the bare !CastSaSource device, so the gate reads 0 and
// no card is drawn ("ANOTHER blue spell").
func TestDreamThiefOwnCastAloneDoesNotDraw(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Dream Thief"))
	thief := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = 2, 1
	n0 := len(e.L.Events)
	castMode(t, e, thief, "")
	finishCast(t, e, thief)
	drainQueuedTriggers(t, e)
	draws := 0
	for _, ev := range e.L.Events[n0:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			draws++
		}
	}
	if draws != 0 {
		t.Fatalf("lone-cast Dream Thief ETB drew %d cards, want 0 (the count excludes its own cast)", draws)
	}
}
