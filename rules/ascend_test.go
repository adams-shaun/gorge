package rules

// K:Ascend (CR 702.131, task ascend1): the city's blessing. The grant has two
// halves -- a permanent with the keyword grants continuously while its
// controller controls ten or more permanents (rules/ascend.go's emit-side
// scan), and an instant/sorcery with the keyword grants AS IT RESOLVES,
// before its own body and condition checks read the latch (resolveTop's
// spell branch). The latch is one-way. The pins below drive the grant through
// the real emitted-event paths (a library->battlefield MoveZone for the
// permanent half, resolveTop for the spell half) on real corpus carriers.

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// enterOnBattlefield puts a real corpus card on p's battlefield through the
// REAL MoveZone event path (the fixture placements onBoard/onBoardCard are
// eventless by design), so Engine.emit's post-fold Ascend scan sees the entry
// exactly as a cast would deliver it.
func enterOnBattlefield(t testing.TB, e *Engine, p state.PlayerID, c *cards.Card) state.ObjID {
	t.Helper()
	o := e.G.AddObject(c, p)
	e.G.SetZone(state.ZLibrary, p, append(e.G.Zone(state.ZLibrary, p), o.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	return o.ID
}

// vanillaBears places n distinct 2/2 Bears (no keywords) under p's control,
// eventlessly, as Ascend-count filler.
func vanillaBears(t testing.TB, e *Engine, p state.PlayerID, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		onBoard(t, e, p, fmt.Sprintf("Name:Filler Bear %d\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", i))
	}
}

// TestAscendPermanentGrantsTheBlessingAtTenPermanents pins the permanent
// half on a real corpus carrier (Dusk Charger): the tenth permanent entering
// under an Ascend controller latches the blessing; an opponent with the same
// board shape but no Ascend permanent never gets it; and the latch survives
// the Ascend permanent's own death (CR 702.131a, "for the rest of the game").
func TestAscendPermanentGrantsTheBlessingAtTenPermanents(t *testing.T) {
	e := layerEngine(t)
	vanillaBears(t, e, 0, 9)
	dusk := enterOnBattlefield(t, e, 0, corpusCard(t, "Dusk Charger"))
	if !e.G.Players[0].Blessing {
		t.Fatalf("ten permanents with an Ascend carrier did not grant the blessing")
	}
	if e.G.Players[1].Blessing {
		t.Fatalf("seat 1 holds the blessing with no Ascend permanent")
	}
	// The latch never clears: the Ascend permanent dies, the blessing stays.
	e.emit(events.Event{Kind: events.MoveZone, Obj: dusk, From: state.ZBattlefield, To: state.ZGraveyard})
	if !e.G.Players[0].Blessing {
		t.Fatalf("the blessing cleared when the Ascend permanent left the battlefield")
	}
}

// TestAscendNinePermanentsNeverGrants pins the gate's count side: nine
// permanents under an Ascend controller hold no blessing.
func TestAscendNinePermanentsNeverGrants(t *testing.T) {
	e := layerEngine(t)
	vanillaBears(t, e, 0, 8)
	enterOnBattlefield(t, e, 0, corpusCard(t, "Dusk Charger"))
	if e.G.Players[0].Blessing {
		t.Fatalf("nine permanents granted the blessing")
	}
}

// TestDetectiveOfTheMonthCantBlockByNeedsTheBlessing is the report's named
// pin, driven through the real compiled card text: with the blessing,
// Detective of the Month's Condition$ Blessing CantBlockBy makes a Detective
// unblockable; at nine permanents the gate denies and the Detective blocks
// normally.
func TestDetectiveOfTheMonthCantBlockByNeedsTheBlessing(t *testing.T) {
	t.Run("blessed/unblockable", func(t *testing.T) {
		e := layerEngine(t)
		vanillaBears(t, e, 0, 9)
		enterOnBattlefield(t, e, 0, corpusCard(t, "Detective of the Month"))
		attacker := onBoard(t, e, 0, "Name:Beat Cop\nManaCost:1 W\nTypes:Creature Detective\nPT:2/2\nOracle:x\n")
		blocker := onBoard(t, e, 1, "Name:Wall Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		if !e.blockRestricted(blocker, attacker) {
			t.Fatalf("blessed Detective still blockable")
		}
	})
	t.Run("nine permanents/blockable", func(t *testing.T) {
		e := layerEngine(t)
		vanillaBears(t, e, 0, 8)
		enterOnBattlefield(t, e, 0, corpusCard(t, "Detective of the Month"))
		attacker := onBoard(t, e, 0, "Name:Beat Cop\nManaCost:1 W\nTypes:Creature Detective\nPT:2/2\nOracle:x\n")
		blocker := onBoard(t, e, 1, "Name:Wall Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		if e.blockRestricted(blocker, attacker) {
			t.Fatalf("unblessed Detective unblockable (Condition$ Blessing not gated)")
		}
	})
}

// TestAscendSpellResolutionGrantsTheBlessing pins the spell half on a real
// corpus instant/sorcery (Golden Demise): the blessing is granted BEFORE the
// resolving spell's own body runs, and only when the controller controls ten
// or more permanents. The spell's own Count$Blessing-gated branch is a
// separate (unimplemented) head -- this pin asserts the state change only.
func TestAscendSpellResolutionGrantsTheBlessing(t *testing.T) {
	t.Run("ten permanents", func(t *testing.T) {
		e := layerEngine(t)
		vanillaBears(t, e, 0, 10)
		spell := e.G.AddObject(corpusCard(t, "Golden Demise"), 0)
		spell.Zone = state.ZStack
		e.G.SetZone(state.ZStack, 0, []state.ObjID{spell.ID})
		e.resolveTop()
		if !e.G.Players[0].Blessing {
			t.Fatalf("resolving an Ascend spell with ten permanents did not grant the blessing")
		}
		if len(e.G.Stack) != 0 {
			t.Fatalf("resolved spell stayed on the stack (%d objects)", len(e.G.Stack))
		}
	})
	t.Run("nine permanents", func(t *testing.T) {
		e := layerEngine(t)
		vanillaBears(t, e, 0, 9)
		spell := e.G.AddObject(corpusCard(t, "Golden Demise"), 0)
		spell.Zone = state.ZStack
		e.G.SetZone(state.ZStack, 0, []state.ObjID{spell.ID})
		e.resolveTop()
		if e.G.Players[0].Blessing {
			t.Fatalf("resolving an Ascend spell with nine permanents granted the blessing")
		}
	})
}

// TestAscendContinuousBuffFollowsTheBlessing pins the Condition$ Blessing
// Continuous read on the real carrier (Dusk Charger's +2/+2): buffed only
// while the latch holds, re-derived across the emitted BlessingChange.
func TestAscendContinuousBuffFollowsTheBlessing(t *testing.T) {
	e := layerEngine(t)
	dusk := onBoardCard(t, e, 0, corpusCard(t, "Dusk Charger"))
	if got := e.Power(dusk); got != 3 {
		t.Fatalf("unblessed Dusk Charger power = %d, want 3", got)
	}
	e.emit(events.Event{Kind: events.BlessingChange, Player: 0})
	if got := e.Power(dusk); got != 5 {
		t.Fatalf("blessed Dusk Charger power = %d, want 5", got)
	}
}

// TestCantBlockByConditionGateThresholdNotBlessingSpecific proves the new
// per-static condition gate in blockRestricted is condition-driven, not
// hardcoded to Blessing: Cephalid Inkmage's real CantBlockBy (Condition$
// Threshold) only restricts once its controller's graveyard holds seven cards.
func TestCantBlockByConditionGateThresholdNotBlessingSpecific(t *testing.T) {
	e := layerEngine(t)
	ink := onBoardCard(t, e, 0, corpusCard(t, "Cephalid Inkmage"))
	blocker := onBoard(t, e, 1, "Name:Wall Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if e.blockRestricted(blocker, ink) {
		t.Fatalf("Inkmage unblockable below threshold")
	}
	for i := 0; i < 7; i++ {
		addToGraveyardType(t, e, 0, "Sorcery")
	}
	if !e.blockRestricted(blocker, ink) {
		t.Fatalf("Inkmage still blockable at threshold")
	}
}
