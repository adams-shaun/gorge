package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const nightShiftName = "Night Shift of the Living Dead"

// nightShiftGame builds a two-seat game with seat 0's Night Shift and (when
// oppNightShift) seat 1's on the battlefield and `kangaroos` Mother Kangaroos
// in seat 0's deck, ready to be moved onto the battlefield one at a time.
func nightShiftGame(t *testing.T, seed uint64, oppNightShift bool, kangaroos int) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	ns, kang := lookup(t, reg, nightShiftName), lookup(t, reg, "Mother Kangaroo")
	extras0 := []*cards.Card{ns}
	for i := 0; i < kangaroos; i++ {
		extras0 = append(extras0, kang)
	}
	var extras1 []*cards.Card
	if oppNightShift {
		extras1 = []*cards.Card{ns}
	}
	e, _ := flipEngine(t, reg, seed, extras0, extras1)
	mine := moveByName(t, e, 0, nightShiftName, state.ZBattlefield)
	if oppNightShift {
		moveByName(t, e, 1, nightShiftName, state.ZBattlefield)
	}
	if o := e.G.Obj(mine); o == nil || o.Zone != state.ZBattlefield || !effects.PrintedHasKeyword(o, "After you roll a die, you may pay 1 life. If you do, increase or decrease the result by 1. Do this only once each turn.") {
		t.Fatalf("precondition: Night Shift is not on the battlefield carrying its keyword: %+v", o)
	}
	return e, mine
}

// kangarooEnters moves one more Mother Kangaroo onto seat 0's battlefield and
// queues its roll trigger on the stack.
func kangarooEnters(t *testing.T, e *Engine) {
	t.Helper()
	moveByName(t, e, 0, "Mother Kangaroo", state.ZBattlefield)
	e.putTriggersOnStack()
	if len(e.G.Stack) < 1 {
		t.Fatalf("precondition: Kangaroo ETB trigger not queued: stack %d", len(e.G.Stack))
	}
}

// lastDie returns the most recent die Note: natural, final result.
func lastDie(t *testing.T, e *Engine) (natural, result int32) {
	t.Helper()
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		if _, _, nat, res, ok := effects.DieRollResult(e.L.Events[i]); ok {
			return nat, res
		}
	}
	t.Fatal("no die was rolled")
	return 0, 0
}

func zombieTokens(e *Engine) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Zombie Employee Token" {
			n++
		}
	}
	return n
}

// nightShiftSeed makes Mother Kangaroo's first roll a natural 5 (measured by
// a seed search; asserted by every test that depends on it).
const nightShiftSeed = 4

// chooseAsks counts the KChoose asks posed so far, including one a direct
// (kernel-less) resolveTop could not serve: that logs the "ask answered with
// its default" Note instead of a pending decision.
func chooseAsks(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.DecisionAsk && ev.Text == string(decision.KChoose) ||
			ev.Kind == events.Note && strings.HasPrefix(ev.Text, "ask answered with its default") {
			n++
		}
	}
	return n
}

func modsThisTurn(e *Engine, id state.ObjID) int32 {
	return e.G.Obj(id).RuntimeSVars["ModsThisTurn"]
}

// rollWithAnswer rolls Kangaroo's die under Night Shift, asserts the offer's
// shape and a natural 5, answers it with choice and drains the stack.
func rollWithAnswer(t *testing.T, e *Engine, choice int) {
	t.Helper()
	kangarooEnters(t, e)
	d := passUntilNonPriority(t, e, 20)
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 3 || d.Min != 1 || d.Max != 1 {
		t.Fatalf("precondition: expected the roll-modify choose-one ask, got %+v", d)
	}
	if d.Options[0].Kind != "keep" || d.Options[1].Kind != "increase" || d.Options[2].Kind != "decrease" {
		t.Fatalf("option order = %v %v %v, want keep/increase/decrease", d.Options[0].Kind, d.Options[1].Kind, d.Options[2].Kind)
	}
	submitChoices(t, e, choice)
	passUntilStackEmpty(t, e, 60)
}

func TestNightShiftModifiesDieBeforeRolledDieTriggers(t *testing.T) {
	t.Parallel()
	e, ns := nightShiftGame(t, nightShiftSeed, false, 1)
	life := e.G.Players[0].Life
	rollWithAnswer(t, e, 1)
	nat, res := lastDie(t, e)
	if nat != 5 {
		t.Fatalf("precondition: natural die = %d, want 5 (re-measure nightShiftSeed)", nat)
	}
	if res != 6 {
		t.Fatalf("result = %d, want 6 (natural 5, increased by 1)", res)
	}
	if got := e.G.Players[0].Life; got != life-1 {
		t.Fatalf("life = %d, want %d (one life paid)", got, life-1)
	}
	if got := modsThisTurn(e, ns); got != 1 {
		t.Fatalf("ModsThisTurn = %d, want 1", got)
	}
	if got := zombieTokens(e); got != 1 {
		t.Fatalf("Zombie Employee tokens = %d, want 1: the roll-a-6 trigger must read the MODIFIED result", got)
	}

	// Same seed, declined: the die stands, nothing is paid or counted.
	e, ns = nightShiftGame(t, nightShiftSeed, false, 1)
	life = e.G.Players[0].Life
	rollWithAnswer(t, e, 0)
	nat, res = lastDie(t, e)
	if nat != 5 || res != 5 {
		t.Fatalf("declined: natural %d result %d, want 5 and 5", nat, res)
	}
	if e.G.Players[0].Life != life || modsThisTurn(e, ns) != 0 || zombieTokens(e) != 0 {
		t.Fatalf("declined: life %d (was %d), ModsThisTurn %d, tokens %d; want nothing paid, counted or created",
			e.G.Players[0].Life, life, modsThisTurn(e, ns), zombieTokens(e))
	}

	// Decrease: 5 -> 4, one life, counted.
	e, ns = nightShiftGame(t, nightShiftSeed, false, 1)
	life = e.G.Players[0].Life
	rollWithAnswer(t, e, 2)
	if nat, res = lastDie(t, e); nat != 5 || res != 4 {
		t.Fatalf("decreased: natural %d result %d, want 5 and 4", nat, res)
	}
	if e.G.Players[0].Life != life-1 || modsThisTurn(e, ns) != 1 {
		t.Fatalf("decreased: life %d (was %d), ModsThisTurn %d", e.G.Players[0].Life, life, modsThisTurn(e, ns))
	}
}

func TestNightShiftOncePerTurnAndResetsAtTurnBegin(t *testing.T) {
	t.Parallel()
	e, ns := nightShiftGame(t, nightShiftSeed, false, 3)
	rollWithAnswer(t, e, 2)
	if got := modsThisTurn(e, ns); got != 1 {
		t.Fatalf("precondition: ModsThisTurn = %d after an accepted modification, want 1", got)
	}
	// A second roll the same turn poses no ask and keeps the natural die.
	asks, dice := chooseAsks(e), len(dieRolls(e))
	kangarooEnters(t, e)
	passUntilStackEmpty(t, e, 60)
	if len(dieRolls(e)) != dice+1 {
		t.Fatalf("precondition: second roll did not happen (%d dice, was %d)", len(dieRolls(e)), dice)
	}
	if chooseAsks(e) != asks {
		t.Fatal("a second modification was offered in the same turn")
	}
	if nat, res := lastDie(t, e); nat != res {
		t.Fatalf("second roll modified: natural %d result %d", nat, res)
	}
	// The next turn's TurnBegin resets the count and the ask returns.
	driveToStep(t, e, e.G.Turn+1, 1, state.StepMain1)
	if got := modsThisTurn(e, ns); got != 0 {
		t.Fatalf("precondition: ModsThisTurn = %d after TurnBegin, want 0", got)
	}
	rollWithAnswer(t, e, 1)
	if got := modsThisTurn(e, ns); got != 1 {
		t.Fatalf("ModsThisTurn = %d after the next turn's modification, want 1", got)
	}
}

func TestNightShiftNotOfferedWhenUnaffordableOrOpposing(t *testing.T) {
	t.Parallel()
	// Zero life: nothing payable, no ask. (Resolved directly: a state-based
	// check at priority would end the game first.)
	e, _ := nightShiftGame(t, nightShiftSeed, false, 1)
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -e.G.Players[0].Life})
	if e.G.Players[0].Life != 0 {
		t.Fatalf("precondition: life = %d, want 0", e.G.Players[0].Life)
	}
	asks := chooseAsks(e)
	kangarooEnters(t, e)
	e.resolveTop()
	if nat, res := lastDie(t, e); nat != 5 || res != 5 {
		t.Fatalf("precondition: natural %d result %d, want an unmodified 5", nat, res)
	}
	if chooseAsks(e) != asks {
		t.Fatal("a roll-modify ask was posed to a player with no life to pay")
	}

	// An opponent's Night Shift never offers to seat 0 (and seat 0 has none).
	e, _ = nightShiftGame(t, nightShiftSeed, true, 1)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face().Name == nightShiftName {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
		}
	}
	opp := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 1) {
		if o := e.G.Obj(id); o != nil && o.Face().Name == nightShiftName {
			opp++
		}
	}
	if opp != 1 || len(e.G.Zone(state.ZBattlefield, 0)) != 0 {
		t.Fatalf("precondition: want exactly the opponent's Night Shift in play, got %d (seat 0 holds %d permanents)", opp, len(e.G.Zone(state.ZBattlefield, 0)))
	}
	asks = chooseAsks(e)
	kangarooEnters(t, e)
	for i := 0; i < 30 && len(e.G.Stack) > 0; i++ {
		if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
			t.Fatalf("an opponent's Night Shift offered seat 0 a modification: %+v", d)
		}
		passOnce(t, e)
	}
	if len(e.G.Stack) != 0 {
		t.Fatal("precondition: the roll trigger never resolved")
	}
	if nat, res := lastDie(t, e); nat != res {
		t.Fatalf("opposing Night Shift modified seat 0's die: natural %d result %d", nat, res)
	}
	if chooseAsks(e) != asks {
		t.Fatal("an opponent's Night Shift offered seat 0 a modification")
	}
}

func TestNightShiftResumedAskDoesNotReroll(t *testing.T) {
	t.Parallel()
	e, _ := nightShiftGame(t, nightShiftSeed, false, 1)
	kangarooEnters(t, e)
	before := len(dieRolls(e))
	d := passUntilNonPriority(t, e, 20)
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 3 {
		t.Fatalf("precondition: expected the roll-modify ask, got %+v", d)
	}
	if got := len(dieRolls(e)); got != before {
		t.Fatalf("precondition: %d dice noted before the answer, want %d (no Note before the answer)", got, before)
	}
	submitChoices(t, e, 1)
	passUntilStackEmpty(t, e, 60)
	if got := len(dieRolls(e)); got != before+1 {
		t.Fatalf("dice = %d after the answer, want %d: the resumed roll rolled again", got, before+1)
	}
	if nat, res := lastDie(t, e); nat != 5 || res != 6 {
		t.Fatalf("natural %d result %d, want 5 -> 6", nat, res)
	}
}

func TestRollDiceReplRegisteredForCorpusCarriers(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	if !sup["repl:RollDice"] {
		t.Fatal("effects.Supported() lacks repl:RollDice")
	}
	for _, name := range []string{"Barbarian Class", "Pixie Guide"} {
		if got := reg.Unsupported(lookup(t, reg, name), sup); len(got) != 0 {
			t.Fatalf("%s still unsupported: %v", name, got)
		}
	}
	for _, tok := range reg.Unsupported(lookup(t, reg, "Wyll, Blade of Frontiers"), sup) {
		if tok == "repl:RollDice" {
			t.Fatal("Wyll still reports repl:RollDice unsupported")
		}
	}
	if got := reg.Unsupported(lookup(t, reg, nightShiftName), sup); len(got) != 0 {
		t.Fatalf("%s still unsupported: %v", nightShiftName, got)
	}
}

// TestBarbarianClassRollDiceReplacementAddsDieAndIgnoresLowest is the Wyll
// leaf for Barbarian Class (level 1: the replacement is its printed base).
func TestBarbarianClassRollDiceReplacementAddsDieAndIgnoresLowest(t *testing.T) {
	t.Parallel()
	e, class, kang, rolls := kangarooRollFixture(t, "Barbarian Class")
	rollReplacementOnBoard(t, e, class, "Barbarian Class")
	if n := len(rolls); n != 2 {
		t.Fatalf("want two dice rolled (1 + the replacement's one), got %d (%v)", n, rolls)
	}
	if rolls[0] == rolls[1] {
		t.Fatalf("precondition: the two dice tie (%v), so ignoring the lowest is unobservable", rolls)
	}
	hi := rolls[0]
	if rolls[1] > hi {
		hi = rolls[1]
	}
	if got := e.G.Obj(kang).Counter("P1P1"); got != hi {
		t.Fatalf("Kangaroo put %d +1/+1 counters, want %d (dice %v, lowest ignored)", got, hi, rolls)
	}
}
