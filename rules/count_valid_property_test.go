// Count$Valid <spec>$CardCounters.<KIND> — the summed counter property
// (ticket count-cardcounters-prop).
//
// effects/count.go's zone-count head recognised CardPower/CardToughness/
// CardManaCost/CardTypes/Colors as `$<Property>` suffixes but not
// CardCounters.<KIND>, so the whole token became the spec, matched nothing
// and the count degraded to 0 -- silently, because a count that fails closed
// looks like "no counters". The end-to-end pin is Kate Stewart's attack
// trigger (`SVar:X:Count$Valid Permanent.YouCtrl$CardCounters.TIME`,
// `NumAtt$ +X` on the pay-{8} PumpAll); the ALL spelling is pinned on
// Maester Seymour's shape (the Sylvan Library/Backstreet Bruiser family's
// "number of counters among ..." sums EVERY kind).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// seedObjectCounter puts n counters of kind on an object through the event
// the engine folds, so the read under test is event-backed exactly as in
// play.
func seedObjectCounter(t *testing.T, e *Engine, id state.ObjID, kind string, n int32) {
	t.Helper()
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: n})
}

// TestKateStewartXCountsTimeCounters runs the real corpus card end to end:
// with three time counters spread over seat 0's permanents and eight mana in
// the pool, Kate Stewart's attack trigger's pay-{8} pump grants each attacking
// creature +3/+3. Before the fix X resolved to 0 (the whole
// `Permanent.YouCtrl$CardCounters.TIME` token was treated as the spec, matched
// nothing) and the pump was +0/+0.
func TestKateStewartXCountsTimeCounters(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	reg := testutil.CorpusRegistry(t)
	kateCard, ok := reg.Lookup("Kate Stewart")
	if !ok {
		t.Fatal("corpus has no Kate Stewart")
	}

	// Seed all three time counters on the ally, never on Kate: her own
	// CounterAddedOnce trigger only exists while she is on the battlefield, so
	// a counter placed on HER after placement would queue a Soldier-token
	// trigger beside the pay-{8} decision under test.
	ally := onBoard(t, e, 0, "Name:Time Vault\nManaCost:2\nTypes:Artifact Creature Construct\nPT:0/0\nOracle:x\n")
	enemy := onBoard(t, e, 1, "Name:Odd Relic\nManaCost:2\nTypes:Artifact Creature Construct\nPT:0/0\nOracle:x\n")
	seedObjectCounter(t, e, ally, "TIME", 3)
	seedObjectCounter(t, e, enemy, "TIME", 5) // an opponent's count never counts
	kate := onBoardCard(t, e, 0, kateCard)

	// Preconditions: the trigger's pump is the CardCounters.TIME read and the
	// seeded counter total differs from the answer the buggy read produced
	// (0, so a +X pump must move the power off its base).
	if body := svarBodyOf(t, e.G.Obj(kate).Face(), "X"); body != "Count$Valid Permanent.YouCtrl$CardCounters.TIME" {
		t.Fatalf("test precondition: Kate SVar:X = %q", body)
	}
	if got := e.G.Obj(kate).Counter("TIME") + e.G.Obj(ally).Counter("TIME"); got != 3 {
		t.Fatalf("test precondition: seat 0 time counters = %d, want 3", got)
	}

	// Fund the pay-{8}.
	e.G.Players[0].Pool[state.MC] = 8

	others := []state.ObjID{kate, ally}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: others})

	// Drive the trigger queue until the pay-{8} ask is answered: order the
	// simultaneous triggers, place them on the stack, pass priority and
	// resolve until the attack trigger's optional cost poses its KModes.
	paid := false
	for i := 0; i < 16 && !paid; i++ {
		e.putTriggersOnStack()
		d := e.Pending()
		if d == nil {
			e.resolveTop()
			continue
		}
		switch d.Kind {
		case decision.KTriggerOrder:
			idx := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				idx = append(idx, o.Index)
			}
			submitChoices(t, e, idx...)
		case decision.KChoose:
			if d.Options[0].Kind != "trigger_cost_pay" {
				t.Fatalf("unexpected KChoose while driving the attack trigger: %+v", d)
			}
			submitChoices(t, e, 0) // pay {8}
			paid = true
		case decision.KPriority:
			for _, o := range d.Options {
				if o.Kind == "pass" {
					if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
						t.Fatalf("submit pass: %v", err)
					}
					break
				}
			}
		default:
			t.Fatalf("unexpected decision while driving the attack trigger: %+v", d)
		}
	}
	if !paid {
		t.Fatal("test precondition: the attack trigger's pay-{8} ask never posed")
	}

	// +3/+3 from the three time counters: Kate 3/3 -> 6/6, the ally 0/0 -> 3/3.
	if got := e.Power(kate); got != 6 {
		t.Fatalf("Kate power = %d, want 6 (base 3 + X where X = 3 time counters)", got)
	}
	if got := e.Toughness(kate); got != 6 {
		t.Fatalf("Kate toughness = %d, want 6", got)
	}
	if got := e.Power(ally); got != 3 {
		t.Fatalf("ally power = %d, want 3 (the PumpAll hits every attacker)", got)
	}
}

// TestCardCountersAllSumsEveryKind pins the ALL marker (Maester Seymour,
// Backstreet Bruiser, Lux Artillery: "the number of counters among ...").
// state.Object.Counter is the one home for the marker, so the three
// CardCounters.<KIND> readers cannot disagree on it.
func TestCardCountersAllSumsEveryKind(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	a := onBoard(t, e, 0, "Name:Counter Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b := onBoard(t, e, 0, "Name:Counter Ox\nManaCost:1 G\nTypes:Creature Ox\nPT:3/3\nOracle:x\n")
	seedObjectCounter(t, e, a, "P1P1", 2)
	seedObjectCounter(t, e, a, "SHIELD", 1)
	seedObjectCounter(t, e, b, "LORE", 3)

	// Precondition: the object really holds three distinct kinds in total, so
	// a sum equals 6 and any single-kind read would differ.
	if got := e.G.Obj(a).Counter("P1P1") + e.G.Obj(a).Counter("SHIELD") + e.G.Obj(b).Counter("LORE"); got != 6 {
		t.Fatalf("test precondition: seeded counters = %d, want 6", got)
	}

	ctx := &effects.Ctx{Controller: 0, Source: a}
	if n, ok := effects.EvalCountOK(e, ctx, "Count$Valid Creature.YouCtrl$CardCounters.ALL"); !ok || n != 6 {
		t.Fatalf("CardCounters.ALL = %d (ok %v), want 6 (every kind summed)", n, ok)
	}
	// A single-kind read still works and is NOT the all-kinds total.
	if n, ok := effects.EvalCountOK(e, ctx, "Count$Valid Creature.YouCtrl$CardCounters.P1P1"); !ok || n != 2 {
		t.Fatalf("CardCounters.P1P1 = %d (ok %v), want 2", n, ok)
	}
	// The bare source head shares the same ALL home (Warden of the Inner Sky).
	if n, ok := effects.EvalCountOK(e, ctx, "Count$CardCounters.ALL"); !ok || n != 3 {
		t.Fatalf("bare CardCounters.ALL = %d (ok %v), want 3 (the source's own kinds)", n, ok)
	}
}
