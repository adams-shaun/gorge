package rules

// CR 702.147a / 603.3 / 603.7a: Decayed's "when this creature attacks,
// sacrifice it at end of combat" is a real triggered ability. It goes on the
// stack when the creature attacks (players may respond to or counter it), and
// only its resolution creates the delayed end-of-combat sacrifice.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// freeStifleSrc is the corpus Stifle shape at no mana cost: the declare-attackers
// window opens after the main-phase pool has emptied.
var freeStifleSrc = strings.Replace(stifleSrc, "ManaCost:U\n", "ManaCost:0\n", 1)

const decayedBearSrc = "Name:Decayed Bear\nManaCost:B\nTypes:Creature Zombie\nPT:2/2\nK:Haste\nK:Decayed\nOracle:x\n"

// stackAbilitiesOf returns the stack ability objects sourced by src.
func stackAbilitiesOf(e *Engine, src state.ObjID) []state.ObjID {
	var out []state.ObjID
	for _, id := range e.G.Stack {
		if o := e.G.Obj(id); o != nil && o.Ability != nil && o.Source == src {
			out = append(out, id)
		}
	}
	return out
}

// decayedFixture seeds a real deck card (so a log-only replay can rebuild the
// game) and moves it onto seat 0's battlefield through a logged MoveZone,
// ready to attack (the fixtures carry Haste: SummonSick is not
// logged, so clearing it by hand would break the replay).
func decayedFixture(t *testing.T, seed uint64, src string, extras ...string) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg, id := newFixtureDeck(t, seed, src, extras...)
	toMain1(t, e)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZBattlefield})
	e.pending = nil
	e.priorityRound()
	if e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatal("precondition: the fixture did not reach the battlefield")
	}
	return e, cfg, id
}

// declareAttacker drives to the attackers ask and declares id, returning with
// the declaration's priority window open and the trigger still on the stack.
func declareAttacker(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, id)
}

// requireDecayedOnStack asserts the declaration put exactly one ability
// sourced by id on the stack and armed no promise yet (so the resolution
// assertions that follow are not vacuous).
func requireDecayedOnStack(t *testing.T, e *Engine, id state.ObjID) state.ObjID {
	t.Helper()
	abs := stackAbilitiesOf(e, id)
	if len(abs) != 1 {
		t.Fatalf("stack after the declaration holds %d abilities sourced by the decayed attacker, want 1 (stack %v)", len(abs), e.G.Stack)
	}
	if hasDecayedPromise(e, id) {
		t.Fatalf("the sacrifice promise was armed at declaration, before the trigger resolved: %+v", e.G.Delayed)
	}
	return abs[0]
}

func requireSacrificedAtEndOfCombat(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	passUntilStackEmpty(t, e, 40)
	if !hasDecayedPromise(e, id) {
		t.Fatalf("no live __kwDecayedSacrifice registration after the trigger resolved: %+v", e.G.Delayed)
	}
	if got := e.G.Obj(id).Zone; got != state.ZBattlefield {
		t.Fatalf("decayed attacker left the battlefield before end of combat: %s", got)
	}
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEndCombat)
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("decayed attacker at end of combat = %s, want graveyard", got)
	}
}

func TestDecayedAttackTriggerUsesStack(t *testing.T) {
	t.Parallel()
	e, cfg, id := decayedFixture(t, 981, decayedBearSrc)
	declareAttacker(t, e, id)
	requireDecayedOnStack(t, e, id)
	requireSacrificedAtEndOfCombat(t, e, id)
	replayCheck(t, e, cfg)
}

func TestDecayedAttackTriggerCountered(t *testing.T) {
	t.Parallel()
	e, cfg, id := decayedFixture(t, 982, decayedBearSrc, freeStifleSrc)
	stifle := findByName(e, "Stifle", 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: stifle, From: e.G.Obj(stifle).Zone, To: state.ZHand})
	declareAttacker(t, e, id)
	trig := requireDecayedOnStack(t, e, id)

	// Seat 0 holds priority in the declare-attackers window with the trigger
	// on the stack and casts Stifle at it.
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority with the trigger on the stack, got %+v", d)
	}
	idx := castOptionIdx(d, "cast", stifle)
	if idx < 0 {
		t.Fatalf("Stifle not offered with the decayed trigger on the stack: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) != 1 || d.Options[0].Obj != trig {
		t.Fatalf("expected Stifle's target ask to offer only the decayed trigger %d, got %+v", trig, d)
	}
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 40)

	countered := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == trig && ev.From == state.ZStack && ev.To == state.ZExile && ev.Text == "countered" {
			countered = true
		}
	}
	if !countered {
		t.Fatal("precondition: the decayed trigger was not countered")
	}
	if hasDecayedPromise(e, id) {
		t.Fatalf("a countered decayed trigger still armed the sacrifice: %+v", e.G.Delayed)
	}
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEndCombat)
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(id).Zone; got != state.ZBattlefield {
		t.Fatalf("decayed attacker with its trigger countered = %s, want battlefield", got)
	}
	replayCheck(t, e, cfg)
}

func TestDecayedGrantedAndCounterUseStack(t *testing.T) {
	t.Parallel()
	const plain = "Name:Plain Zombie\nManaCost:B\nTypes:Creature Zombie\nPT:2/2\nK:Haste\nOracle:x\n"
	t.Run("counter", func(t *testing.T) {
		t.Parallel()
		e, cfg, id := decayedFixture(t, 983, plain)
		e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "Decayed", Amount: 1})
		if !e.HasKeyword(id, "Decayed") {
			t.Fatal("precondition: the decayed counter did not confer the keyword")
		}
		declareAttacker(t, e, id)
		requireDecayedOnStack(t, e, id)
		requireSacrificedAtEndOfCombat(t, e, id)
		replayCheck(t, e, cfg)
	})
	t.Run("grant", func(t *testing.T) {
		t.Parallel()
		e, _, id := decayedFixture(t, 984, plain)
		e.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: "Card.Self", Controller: 0,
			Layer: state.LAbilities, AddKeywords: []string{"Decayed"},
		})
		if !e.HasKeyword(id, "Decayed") {
			t.Fatal("precondition: the layer-6 Decayed grant is not live")
		}
		declareAttacker(t, e, id)
		requireDecayedOnStack(t, e, id)
		requireSacrificedAtEndOfCombat(t, e, id)
		// No replayCheck: AddContinuous is an eventless fixture grant.
	})
}

func TestRotCurseRakshasaDecayedTrigger(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 985, "Name:Placeholder\nManaCost:0\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	id := onBoardReadyCard(t, e, 0, corpusCard(t, "Rot-Curse Rakshasa"))
	declareAttacker(t, e, id)
	requireDecayedOnStack(t, e, id)
	requireSacrificedAtEndOfCombat(t, e, id)
}

// TestDecayedAttackTriggerRegistersOnce pins the resolution-time dedup: a
// second decayed attack trigger resolving while the promise is live (an extra
// combat declaring the same attacker, a doubled trigger) must not register a
// second sacrifice.
func TestDecayedAttackTriggerRegistersOnce(t *testing.T) {
	t.Parallel()
	e, _, id := decayedFixture(t, 986, decayedBearSrc)
	declareAttacker(t, e, id)
	requireDecayedOnStack(t, e, id)
	passUntilStackEmpty(t, e, 40)
	count := func() int {
		n := 0
		for i := range e.G.Delayed {
			if e.G.Delayed[i].Source == id && e.G.Delayed[i].Execute == "__kwDecayedSacrifice" {
				n++
			}
		}
		return n
	}
	if count() != 1 {
		t.Fatalf("precondition: want one live promise after the first resolution, got %d", count())
	}
	e.emit(events.Event{Kind: events.KeywordTriggerPush, Player: 0, Obj: id, Counter: "__kwDecayedAttack", Text: "decayed ability"})
	if len(stackAbilitiesOf(e, id)) != 1 {
		t.Fatalf("precondition: the second decayed trigger is not on the stack: %v", e.G.Stack)
	}
	passUntilStackEmpty(t, e, 40)
	if n := count(); n != 1 {
		t.Fatalf("a second resolution registered %d promises, want 1", n)
	}
}

// CR 400.7: a decayed creature flickered in response to its own attack
// trigger returns as a new object that never attacked, so the resolving
// trigger must not arm the sacrifice against it.
func TestDecayedAttackTriggerFlickeredInResponse(t *testing.T) {
	t.Parallel()
	e, cfg, id := decayedFixture(t, 984, decayedBearSrc)
	declareAttacker(t, e, id)
	requireDecayedOnStack(t, e, id)
	before := e.G.Obj(id).Incarnation
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZExile, To: state.ZBattlefield})
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.Incarnation == before {
		t.Fatalf("precondition: the creature must be back as a new incarnation, zone %s incarnation %d (was %d)", o.Zone, o.Incarnation, before)
	}
	if len(stackAbilitiesOf(e, id)) != 1 {
		t.Fatalf("precondition: the trigger must still be on the stack: %v", e.G.Stack)
	}
	passUntilStackEmpty(t, e, 40)
	if hasDecayedPromise(e, id) {
		t.Fatalf("the trigger armed the sacrifice against the re-entered object: %+v", e.G.Delayed)
	}
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEndCombat)
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(id).Zone; got != state.ZBattlefield {
		t.Fatalf("the flickered creature = %s after end of combat, want battlefield", got)
	}
	replayCheck(t, e, cfg)
}
