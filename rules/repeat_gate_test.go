// RepeatCheckSVar$/RepeatSVarCompare$ -- the DB$ Repeat between-iteration
// gate (task agent-20260919T192133Z-20ce31d6).
//
// The end-to-end pins run on the REAL corpus card Grist, the Hunger Tide,
// whose [+1] is `AB$ Repeat | RepeatCheckSVar$ MilledInsect |
// RepeatSVarCompare$ EQ1 | RepeatSubAbility$ CleanupBegin` with
// `SVar:MilledInsect:Remembered$Valid Card.Insect` and a body that mills one
// card with RememberMilled$ True. The gate is a do-while (the body runs
// first, then the gate decides whether to run again), re-evaluated per
// iteration over the remembered set the body itself grows.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// gristWithLibraryTop puts the real corpus Grist on seat 0's battlefield and
// loads seat 0's library with the named cards on TOP of its mountains. It
// asserts the precondition every later assertion rides on: Grist on the
// battlefield at its printed 3 loyalty.
func gristWithLibraryTop(t *testing.T, top ...*cards.Card) (e *Engine, grist state.ObjID) {
	t.Helper()
	e = handEngineTokens(t, corpusAlternativeCard(t, "Grist, the Hunger Tide"))
	grist = e.G.Zone(state.ZHand, 0)[0]
	lib := e.G.Zone(state.ZLibrary, 0)
	var ids []state.ObjID
	for _, c := range top {
		o := e.G.AddObject(c, 0)
		o.Zone = state.ZLibrary
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZLibrary, 0, append(ids, lib...))
	placeOnBattlefield(t, e, grist)
	g := e.G.Obj(grist)
	if g == nil || g.Zone != state.ZBattlefield {
		t.Fatalf("test precondition: Grist not on the battlefield: %+v", g)
	}
	if got := g.Counter("LOYALTY"); got != 3 {
		t.Fatalf("test precondition: Grist loyalty = %d, want its printed 3", got)
	}
	e.priorityRound()
	if e.Pending() == nil || e.Pending().Kind != decision.KPriority {
		t.Fatalf("test precondition: not at a priority decision after the entry settled: %+v", e.Pending())
	}
	return e, grist
}

// activateGristPlusOne submits Grist's [+1] activation (the first loyalty
// ability option at priority) and resolves it, settling whatever the loop's
// tokens and mills queue.
func activateGristPlusOne(t *testing.T, e *Engine, grist state.ObjID) {
	t.Helper()
	activateAbility(t, e, grist)
	if len(e.G.Stack) == 0 {
		t.Fatalf("activation did not reach the stack")
	}
	e.resolveTop()
	for i := 0; i < 25; i++ {
		if len(e.pendingTriggers) == 0 && len(e.G.Stack) == 0 {
			return
		}
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		e.resolveTop()
	}
	t.Fatalf("Grist's [+1] did not settle: %d pending triggers, %d on the stack",
		len(e.pendingTriggers), len(e.G.Stack))
}

func battlefieldTokensNamed(t *testing.T, e *Engine, name string) (n int) {
	t.Helper()
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone == state.ZBattlefield && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}

// TestGristLoyaltyRepeatsWhileInsectsAreMilled is the crossing pin: with an
// Insect on top of the library the first iteration mills it, puts a loyalty
// counter and repeats; the second iteration mills a non-Insect (a Mountain)
// and the gate stops the loop. 2 tokens, 2 mills, one loyalty counter.
func TestGristLoyaltyRepeatsWhileInsectsAreMilled(t *testing.T) {
	t.Parallel()
	insect := card(t, "Name:Test Insect\nTypes:Creature Insect\nPT:1/1\nOracle:x\n")
	e, grist := gristWithLibraryTop(t, insect)
	activateGristPlusOne(t, e, grist)

	if got := battlefieldTokensNamed(t, e, "Insect Token"); got != 2 {
		t.Fatalf("Insect tokens on the battlefield = %d, want 2 (the loop repeated once)", got)
	}
	g := e.G.Obj(grist)
	// 3 printed + 1 activation cost + 1 put by the first iteration's body.
	if got := g.Counter("LOYALTY"); got != 5 {
		t.Fatalf("Grist loyalty = %d, want 5 (activation +1, one body counter)", got)
	}
	gy := e.G.Zone(state.ZGraveyard, 0)
	if len(gy) != 2 {
		t.Fatalf("seat 0 graveyard holds %d cards, want 2 (one mill per iteration)", len(gy))
	}
	if o := e.G.Obj(gy[0]); o == nil || o.Face() == nil || o.Face().Name != "Test Insect" {
		t.Fatalf("top of the graveyard = %+v, want the milled Test Insect", o)
	}
}

// TestGristLoyaltyStopsWhenNoInsectIsMilled is the gate-false control: with
// only Mountains on top of the library the first iteration mills a
// non-Insect, the gate fails and the loop stops after one iteration -- one
// token, one mill, no loyalty counter.
func TestGristLoyaltyStopsWhenNoInsectIsMilled(t *testing.T) {
	t.Parallel()
	e, grist := gristWithLibraryTop(t)
	activateGristPlusOne(t, e, grist)

	if got := battlefieldTokensNamed(t, e, "Insect Token"); got != 1 {
		t.Fatalf("Insect tokens on the battlefield = %d, want 1 (gate false, no repeat)", got)
	}
	if got := e.G.Obj(grist).Counter("LOYALTY"); got != 4 {
		t.Fatalf("Grist loyalty = %d, want 4 (activation +1 only, no body counter)", got)
	}
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 1 {
		t.Fatalf("seat 0 graveyard holds %d cards, want 1", got)
	}
}
