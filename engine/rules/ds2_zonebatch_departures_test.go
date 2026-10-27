package rules

// A simultaneous departure batch is ONE zone batch (CR 603.2c, 603.10a):
// DestroyAll's and Sacrifice Amount$ N's moves are one "event" for
// trig:ChangesZoneAll, so Vengeful Townsfolk's "whenever one or more other
// creatures you control die" queues ONCE per batch, not once per permanent.
//
// The brief's two live gorge-compliance rows (VengefulTownsfolkTest
// test-on-non-token and test-double-sacrifice) are exactly these two shapes.
// The bracket that fixes them is Engine.BatchDepartures -- the one structural
// place every simultaneous departure batch already goes through
// (DestroyAll, Sacrifice, Destroy, SacrificeAll, ChangeZoneAll, ChangeZone),
// so the fix cannot miss a future caller.
//
// The negative half matters as much as the positive one: two SEPARATE
// destroys are two batches and must still queue two triggers.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// townsfolkBatchEngine builds a two-seat fixture game whose seat-0
// battlefield holds the REAL corpus Vengeful Townsfolk (the ChangesZoneAll
// carrier under test) plus one hand-authored creature per bearCount, and
// returns the engine, those creature ids in placement order and the townsfolk
// id. The townsfolk must be the real script -- its T: line IS the subject --
// while the creatures are freely authored (no Forge text is ever inlined).
func townsfolkBatchEngine(t *testing.T, reg *cards.Registry, bearCount int) (*Engine, []state.ObjID, state.ObjID) {
	t.Helper()
	e, _, _ := dsBoard(t, reg, "Vengeful Townsfolk")
	var tf state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Vengeful Townsfolk" {
			tf = id
		}
	}
	if tf == 0 {
		t.Fatalf("precondition: Vengeful Townsfolk not on the battlefield")
	}
	ids := make([]state.ObjID, 0, bearCount)
	for i := 0; i < bearCount; i++ {
		ids = append(ids, onBoard(t, e, 0, "Name:Fixture Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	}
	return e, ids, tf
}

// townsfolkCounters reads the townsfolk's +1/+1 counter count.
func townsfolkCounters(t *testing.T, e *Engine, tf state.ObjID) int {
	t.Helper()
	o := e.G.Obj(tf)
	if o == nil {
		t.Fatal("Vengeful Townsfolk left the battlefield")
	}
	for _, c := range o.Counters {
		if c.Kind == "P1P1" {
			return int(c.N)
		}
	}
	return 0
}

// queuedChangesZoneAll counts the pending triggers the townsfolk's
// ChangesZoneAll line has queued.
func queuedChangesZoneAll(t *testing.T, e *Engine, tf state.ObjID) int {
	t.Helper()
	n := 0
	for _, pt := range e.pendingTriggers {
		if pt.Source == tf {
			n++
		}
	}
	return n
}

// resolveTownsfolkCounters drains the pending triggers and the stack, then
// returns the townsfolk's +1/+1 counter count.
func resolveTownsfolkCounters(t *testing.T, e *Engine, tf state.ObjID) int {
	t.Helper()
	for i := 0; i < 50; i++ {
		if d := e.Pending(); d != nil && len(d.Options) > 0 {
			// A standing priority decision from the fixture cannot be
			// resolved by resolveTop; take its first option (the usual
			// "pass" answer) so the drain makes progress.
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
				Choices: []int{d.Options[0].Index}}); err != nil {
				t.Fatalf("submit the standing decision during the drain: %v", err)
			}
			continue
		}
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		if len(e.G.Stack) == 0 {
			return townsfolkCounters(t, e, tf)
		}
		e.resolveTop()
	}
	t.Fatalf("townsfolk drain did not settle: %d pending, %d on the stack", len(e.pendingTriggers), len(e.G.Stack))
	return 0
}

// TestDestroyAllIsOneChangesZoneAllBatch pins the DestroyAll half: three
// creatures die in ONE DestroyAll, so the batch-level ChangesZoneAll line
// queues exactly one trigger and the townsfolk gets one counter (4/4, not 6/6).
func TestDestroyAllIsOneChangesZoneAllBatch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, bears, tf := townsfolkBatchEngine(t, reg, 3)
	// Preconditions: the townsfolk and three Bears really are on seat 0's
	// battlefield, and the trigger queue really is empty -- a vacuous board
	// would make the counter assertion below pass for the wrong reason.
	if z := e.G.Obj(tf).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: townsfolk in %s, want battlefield", z)
	}
	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			t.Fatalf("precondition: Bear %d in %s, want battlefield", id, z)
		}
	}
	if n := len(e.pendingTriggers); n != 0 {
		t.Fatalf("precondition: %d pending triggers before the destroy-all, want 0", n)
	}

	effects.Resolve(e, &effects.Ctx{Source: tf, Controller: 0},
		&cards.SA{Kind: "SP", API: "DestroyAll", Params: map[string]string{"ValidCards": "Creature.Other+YouCtrl"}})

	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("precondition: Bear %d in %s after DestroyAll, want graveyard", id, z)
		}
	}
	if got := queuedChangesZoneAll(t, e, tf); got != 1 {
		t.Fatalf("DestroyAll of 3 creatures queued %d ChangesZoneAll triggers, want 1 (CR 603.2c: one simultaneous event)", got)
	}
	if got := resolveTownsfolkCounters(t, e, tf); got != 1 {
		t.Fatalf("Vengeful Townsfolk got %d +1/+1 counters from one DestroyAll of 3 creatures, want 1", got)
	}
}

// TestSacrificeAmountIsOneChangesZoneAllBatch pins the Sacrifice half: one
// api:Sacrifice with Amount$ 2 sacrifices two creatures as a single batch, so
// the ChangesZoneAll line queues once (Barter in Blood's shape).
func TestSacrificeAmountIsOneChangesZoneAllBatch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	// The board corpus Barter in Blood itself, cast and resolved for real:
	// `Amount$ 2 | SacValid$ Creature | Defined$ Player` is exactly the shape
	// under test.
	e := corpusEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Vengeful Townsfolk"), lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Elite Vanguard"),
			lookup(t, reg, "Barter in Blood")}, nil)
	tf := moveByName(t, e, 0, "Vengeful Townsfolk", state.ZBattlefield)
	bears := []state.ObjID{
		moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield),
		moveByName(t, e, 0, "Elite Vanguard", state.ZBattlefield),
	}
	if z := e.G.Obj(tf).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: townsfolk in %s, want battlefield", z)
	}
	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			t.Fatalf("precondition: sacrifice candidate %d in %s, want battlefield", id, z)
		}
	}
	if n := len(e.pendingTriggers); n != 0 {
		t.Fatalf("precondition: %d pending triggers before the sacrifice, want 0", n)
	}

	addMana(t, e, 0, "CCBB")
	submitChoices(t, e, castOptionFor(t, e, fixtureInHand(t, e, "Barter in Blood")).Index)
	// Resolve the spell. "Each player sacrifices two creatures" asks the one
	// seat that has candidates and then resolves, so the drain below answers
	// the ask with its maximum batch and lets the trigger resolve.
	seen := 0
	for i := 0; i < 200; i++ {
		d := e.Pending()
		if d == nil || e.G.Over {
			break
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			submitChoices(t, e, tapePassIndex(d))
			continue
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "sacrifice" {
			seen++
			picks := make([]int, 0, d.Max)
			// Pick the two creatures under test (the townsfolk itself is an
			// offer too, and sacrificing it would end the assertion).
			for _, o := range d.Options {
				if len(picks) == d.Max {
					break
				}
				if o.Obj != tf && o.Obj != 0 {
					picks = append(picks, o.Index)
				}
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: picks}); err != nil {
				t.Fatalf("answer the %d sacrifice ask: %v", d.Player, err)
			}
			continue
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: tapePick(d)}); err != nil {
			t.Fatalf("submit %s/%s: %v", d.Kind, d.ResumeKind, err)
		}
	}
	if seen != 1 {
		t.Fatalf("precondition: Barter in Blood posed %d sacrifice asks, want 1", seen)
	}

	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("precondition: sacrifice candidate %d in %s, want graveyard", id, z)
		}
	}
	// The counter already resolved inside the drain above (a trigger the
	// drain put on the stack is resolved there), so it is read, not re-drained.
	// The per-batch latch is what keeps it at ONE: without it the two moves
	// queue two triggers and this reads 2.
	if got := townsfolkCounters(t, e, tf); got != 1 {
		t.Fatalf("Vengeful Townsfolk got %d +1/+1 counters from one sacrifice of 2 creatures, want 1", got)
	}
}

// TestSeparateDestroysAreSeparateChangesZoneAllBatches is the negative half:
// two single-target destroys are two batches of one and must still queue two
// triggers (VengefulTownsfolkTest#testTwoSeparateDestroy).
func TestSeparateDestroysAreSeparateChangesZoneAllBatches(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, bears, tf := townsfolkBatchEngine(t, reg, 2)
	if z := e.G.Obj(tf).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: townsfolk in %s, want battlefield", z)
	}
	if e.HasKeyword(bears[0], "Indestructible") || e.HasKeyword(bears[1], "Indestructible") {
		t.Fatalf("precondition: a destroy target is Indestructible")
	}
	one := &cards.SA{Kind: "DB", API: "Destroy", Params: map[string]string{"Defined": "Targeted"}}
	effects.Resolve(e, &effects.Ctx{Source: tf, Controller: 0,
		Targets: []state.Target{{Obj: bears[0]}}, TargetsOffered: true}, one)
	if z := e.G.Obj(bears[0]).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: first destroy left the Bear in %s", z)
	}
	// The FIRST destroy's trigger is resolved before the second destroy: two
	// events at different times are two batches, which is what the second
	// destroy below must still be.
	if got := resolveTownsfolkCounters(t, e, tf); got != 1 {
		t.Fatalf("first single destroy gave %d +1/+1 counters, want 1", got)
	}
	e.emit(events.Event{Kind: events.Note, Text: "two separate destroys"})
	effects.Resolve(e, &effects.Ctx{Source: tf, Controller: 0,
		Targets: []state.Target{{Obj: bears[1]}}, TargetsOffered: true}, one)
	if z := e.G.Obj(bears[1]).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: second destroy left the Bear in %s", z)
	}
	if got := queuedChangesZoneAll(t, e, tf); got != 1 {
		t.Fatalf("the second separate destroy queued %d ChangesZoneAll triggers, want 1", got)
	}
	if got := resolveTownsfolkCounters(t, e, tf); got != 2 {
		t.Fatalf("Vengeful Townsfolk got %d +1/+1 counters from two separate destroys, want 2", got)
	}
}

// TestSweeperSBALethalDeathsAreOneChangesZoneAllBatch is the SBA half of the
// same rule: one DamageAll sweep resolves as ONE round of state-based actions
// (CR 704.3), so the lethal-damage deaths it causes are one event for a
// Mode$ ChangesZoneAll line (CR 603.10a/603.2c). Casting the real Pyroclasm
// kills two real creatures, and the townsfolk must put exactly ONE +1/+1
// counter on itself -- the pre-fix SBA path emitted one MoveZone per member
// with no zone batch open, so the latch queued twice.
func TestSweeperSBALethalDeathsAreOneChangesZoneAllBatch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Vengeful Townsfolk"), lookup(t, reg, "Grizzly Bears"),
			lookup(t, reg, "Elite Vanguard"), lookup(t, reg, "Pyroclasm")}, nil)
	tf := moveByName(t, e, 0, "Vengeful Townsfolk", state.ZBattlefield)
	bears := []state.ObjID{
		moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield),
		moveByName(t, e, 0, "Elite Vanguard", state.ZBattlefield),
	}
	pyro := moveByName(t, e, 0, "Pyroclasm", state.ZHand)
	// Preconditions: the watcher and both victims really are on the
	// battlefield, the sweeper really is in hand, the watcher has no counter
	// yet and the trigger queue is empty -- otherwise the final assertion
	// could pass for the wrong reason.
	if z := e.G.Obj(tf).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: townsfolk in %s, want battlefield", z)
	}
	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			t.Fatalf("precondition: sweeper victim %d in %s, want battlefield", id, z)
		}
	}
	if z := e.G.Obj(pyro).Zone; z != state.ZHand {
		t.Fatalf("precondition: Pyroclasm in %s, want hand", z)
	}
	if got := townsfolkCounters(t, e, tf); got != 0 {
		t.Fatalf("precondition: townsfolk already has %d +1/+1 counters", got)
	}
	if n := len(e.pendingTriggers); n != 0 {
		t.Fatalf("precondition: %d pending triggers before the sweep, want 0", n)
	}

	addMana(t, e, 0, "CR")
	submitChoices(t, e, castOptionFor(t, e, pyro).Index)
	// Resolve the sweeper and the SBA round it triggers, then let the
	// ChangesZoneAll trigger resolve. Priority passes while the stack is
	// non-empty; every other ask takes the tape's deterministic answer.
	for i := 0; i < 200; i++ {
		d := e.Pending()
		if d == nil || e.G.Over {
			break
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			submitChoices(t, e, tapePassIndex(d))
			continue
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: tapePick(d)}); err != nil {
			t.Fatalf("submit %s/%s: %v", d.Kind, d.ResumeKind, err)
		}
	}

	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("precondition: sweeper victim %d in %s after Pyroclasm, want graveyard", id, z)
		}
	}
	if z := e.G.Obj(tf).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: townsfolk in %s after Pyroclasm, want battlefield", z)
	}
	// One sweep killed both victims, so the "one or more" line must have
	// latched once and left exactly one counter. Without the zone batch the
	// two MoveZone events queue two triggers and this reads 2.
	if got := townsfolkCounters(t, e, tf); got != 1 {
		t.Fatalf("Vengeful Townsfolk got %d +1/+1 counters from one Pyroclasm killing 2 creatures, want 1", got)
	}
}
