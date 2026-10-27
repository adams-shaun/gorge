package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cr-g197: a plain Mode$ CounterAdded line is one occurrence per counter in a
// CounterChange batch (CR 603.2c), but ActivationLimit$/GameActivationLimit$
// still caps the TOTAL. The batch instance expansion in trigger_match.go must
// therefore re-run both limit gates and reserve per extra instance; before
// this file's fix a single two-counter placement queued two Ant-Man triggers
// and made two Insects.

// counterLimitSourceSrc is the authored two-counter source (the corpus is
// never copied into a fixture): one activation puts two +1/+1 counters on one
// target in ONE CounterChange event.
const counterLimitSourceSrc = "Name:Counter Limit Source\nTypes:Artifact\n" +
	"A:AB$ PutCounter | Cost$ 0 | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ 2 | SpellDescription$ Put two +1/+1 counters on target creature.\n" +
	"Oracle:x\n"

const counterLimitBearSrc = "Name:Counter Limit Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// counterLimitBoard seats triggerCard, a fixture Bear and the two-counter
// source on seat 0's battlefield, with the corpus token definitions (so
// Ant-Man's Insect token is a real minted token). It returns the trigger
// card's id, the Bear and the source.
func counterLimitBoard(t *testing.T, seed uint64, triggerCard *cards.Card) (*Engine, Config, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	source := card(t, counterLimitSourceSrc)
	bear := card(t, counterLimitBearSrc)
	cfg := seatZeroStart(Config{Seed: seed,
		Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append(mountainDeck(t, 40), triggerCard, bear, source),
			mountainDeck(t, 40),
		},
		Tokens: testutil.CorpusRegistry(t).Tokens})
	e := New(cfg)
	e.Advance()
	trig := findAndMoveToBattlefield(t, e, 0, triggerCard.Faces[0].Name)
	bearID := findAndMoveToBattlefield(t, e, 0, bear.Faces[0].Name)
	srcID := findAndMoveToBattlefield(t, e, 0, source.Faces[0].Name)
	for _, id := range []state.ObjID{trig, bearID, srcID} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: object %d is not on the battlefield", id)
		}
	}
	// The activated ability is sorcery-speed, so the first activation needs
	// seat 0's main phase rather than the upkeep Advance stopped at.
	driveToStep(t, e, 1, 0, state.StepMain1)
	return e, cfg, trig, bearID, srcID
}

// activateCounterLimitSource activates the source's 0-cost PutCounter ability
// targeting target and resolves it.
func activateCounterLimitSource(t *testing.T, e *Engine, source, target state.ObjID) {
	t.Helper()
	if e.Pending() == nil {
		e.priorityRound()
	}
	submitChoices(t, e, abilityOption(t, e, source, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target decision after activating the counter source: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == target {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("counter source target %d not offerable: %+v", target, d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 40)
	drainCounterLimitTriggers(t, e)
}

// drainCounterLimitTriggers resolves any triggers still queued after the
// pass drain (passUntilStackEmpty stops the moment the stack empties, which
// can leave the batch instances queued and not yet put on the stack, and two
// instances first pose the CR 603.3b order ask).
func drainCounterLimitTriggers(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 50; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
		}
		d := e.Pending()
		if d != nil && d.Kind == decision.KTriggerOrder {
			choices := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				choices = append(choices, o.Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatalf("submit trigger order: %v", err)
			}
			continue
		}
		if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
			return
		}
		if d == nil {
			e.priorityRound()
			d = e.Pending()
		}
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision draining counter triggers: %+v", d)
		}
		passUntilStackEmpty(t, e, 60)
	}
	t.Fatalf("counter trigger drain did not settle: %d pending, %d on the stack", len(e.pendingTriggers), len(e.G.Stack))
}

// counterLimitTriggerPushes counts the TriggerPush events the log carries for id.
func counterLimitTriggerPushes(e *Engine, id state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush && ev.Obj == id {
			n++
		}
	}
	return n
}

// TestCounterAddedActivationLimitCapsBatchInstances is the defect: Ant-Man's
// "triggers only once each turn" line must queue ONE trigger (and make one
// Insect) even though the placement carried two counters.
func TestCounterAddedActivationLimitCapsBatchInstances(t *testing.T) {
	t.Parallel()
	antman := mshCorpusCard(t, "Ant-Man, Colony Commander")
	if len(antman.Faces[0].Triggers) < 2 {
		t.Fatalf("precondition: Ant-Man carries %d triggers, want the attack and CounterAdded lines", len(antman.Faces[0].Triggers))
	}
	trig := antman.Faces[0].Triggers[1]
	if trig.Mode != "CounterAdded" || trig.ParamStr(cards.PKActivationLimit) != "1" {
		t.Fatalf("precondition: Ant-Man trigger = %+v, want CounterAdded with ActivationLimit$ 1", trig)
	}
	e, cfg, id, bear, source := counterLimitBoard(t, 3101, antman)
	activateCounterLimitSource(t, e, source, bear)
	if got := e.G.Obj(bear).Counter("P1P1"); got != 2 {
		t.Fatalf("precondition: the one placement put %d counters on the Bear, want 2", got)
	}
	if got := counterLimitTriggerPushes(e, id); got != 1 {
		t.Fatalf("one two-counter placement queued %d Ant-Man triggers, want 1 (ActivationLimit$ 1)", got)
	}
	if got := combatTokenCount(e, 0, "Insect Token"); got != 1 {
		t.Fatalf("Ant-Man made %d Insect tokens, want 1 (one capped trigger)", got)
	}
	replayCheck(t, e, cfg)
}

// TestCounterAddedNoLimitStillOnePerCounter is the CR 603.2c regression
// guard: the identical line without ActivationLimit$ still queues one
// trigger per counter in the batch.
func TestCounterAddedNoLimitStillOnePerCounter(t *testing.T) {
	t.Parallel()
	herald := card(t, "Name:Limitless Counter Herald\nTypes:Creature Human Hero\nPT:2/2\n"+
		"T:Mode$ CounterAdded | ValidCard$ Creature | ValidSource$ You | CounterType$ P1P1 | TriggerZones$ Battlefield | Execute$ TrigToken | TriggerDescription$ Whenever you put a +1/+1 counter on a creature, create a 1/1 green Insect creature token.\n"+
		"SVar:TrigToken:DB$ Token | TokenScript$ g_1_1_insect | TokenOwner$ You\n"+
		"Oracle:x\n")
	if _, present := herald.Faces[0].Triggers[0].Param(cards.PKActivationLimit); present {
		t.Fatal("precondition: the no-limit fixture carries an ActivationLimit$")
	}
	e, cfg, id, bear, source := counterLimitBoard(t, 3103, herald)
	activateCounterLimitSource(t, e, source, bear)
	if got := e.G.Obj(bear).Counter("P1P1"); got != 2 {
		t.Fatalf("precondition: the one placement put %d counters on the Bear, want 2", got)
	}
	if got := counterLimitTriggerPushes(e, id); got != 2 {
		t.Fatalf("one two-counter placement queued %d unlimited triggers, want 2 (CR 603.2c one per counter)", got)
	}
	if got := combatTokenCount(e, 0, "Insect Token"); got != 2 {
		t.Fatalf("the unlimited line made %d Insect tokens, want 2", got)
	}
	replayCheck(t, e, cfg)
}

// TestCounterAddedActivationLimitNextTurnFiresAgain pins the per-turn scope:
// the cap is not a once-per-game latch, so seat 0's next turn's placement
// queues exactly one more trigger.
func TestCounterAddedActivationLimitNextTurnFiresAgain(t *testing.T) {
	t.Parallel()
	antman := mshCorpusCard(t, "Ant-Man, Colony Commander")
	if got := antman.Faces[0].Triggers[1].ParamStr(cards.PKActivationLimit); got != "1" {
		t.Fatalf("precondition: Ant-Man ActivationLimit$ = %q, want 1", got)
	}
	e, cfg, id, bear, source := counterLimitBoard(t, 3105, antman)
	activateCounterLimitSource(t, e, source, bear)
	if got := counterLimitTriggerPushes(e, id); got != 1 {
		t.Fatalf("first turn's two-counter placement queued %d Ant-Man triggers, want 1", got)
	}
	// Seat 0's next turn (turn 1 -> turn 3 in a two-seat game).
	driveToStep(t, e, e.G.Turn+2, 0, state.StepMain1)
	if e.G.Turn != 3 || e.G.Active != 0 {
		t.Fatalf("precondition: stopped at turn %d active %d, want turn 3 active 0", e.G.Turn, e.G.Active)
	}
	activateCounterLimitSource(t, e, source, bear)
	if got := counterLimitTriggerPushes(e, id); got != 2 {
		t.Fatalf("the next turn's two-counter placement made the Ant-Man total %d triggers, want 2 (one more)", got)
	}
	if got := combatTokenCount(e, 0, "Insect Token"); got != 2 {
		t.Fatalf("two capped placements made %d Insect tokens, want 2", got)
	}
	replayCheck(t, e, cfg)
}
