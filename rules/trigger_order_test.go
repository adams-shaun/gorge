package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Task 27's card fixtures. Gainer and Drainer are deliberately NOT
// commutative: Drainer loses life equal to the current life total, so which
// of the two resolves first is visible in the final life total and not merely
// in the order two log lines appear.
const gainerSrc = `Name:Gainer
ManaCost:W
Types:Enchantment
T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigGain | TriggerDescription$ gain 5 life
SVar:TrigGain:DB$ GainLife | LifeAmount$ 5 | Defined$ You
Oracle:x
`

const drainerSrc = `Name:Drainer
ManaCost:B
Types:Enchantment
T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigDrain | TriggerDescription$ lose life equal to your life total
SVar:TrigDrain:DB$ LoseLife | LifeAmount$ X | Defined$ You
SVar:X:Count$YourLifeTotal
Oracle:x
`

const mayGainSrc = `Name:Almsgiver
ManaCost:W
Types:Enchantment
T:Mode$ Phase | Phase$ Upkeep | OptionalDecider$ You | Execute$ TrigGain | TriggerDescription$ you may gain 4 life
SVar:TrigGain:DB$ GainLife | LifeAmount$ 4 | Defined$ You
Oracle:x
`

// submit answers the pending decision and fails the test if the engine
// rejects it.
func submit(t *testing.T, e *Engine, choices ...int) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit %s %v: %v", d.Kind, choices, err)
	}
}

// upkeepEngine puts each src on player 0's battlefield and fires an upkeep,
// leaving whatever those triggers matched in e.pendingTriggers.
func upkeepEngine(t *testing.T, srcs ...string) (*Engine, []state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	ids := make([]state.ObjID, 0, len(srcs))
	for _, src := range srcs {
		ids = append(ids, onBoard(t, e, 0, src))
	}
	e.G.Active = 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	return e, ids
}

// TestOneTriggerIsNeverAskedAbout is definition-of-done item 2. A decision
// with a single legal answer is noise on the wire, and CR 603.3b gives the
// player a choice only when there is one to make.
func TestOneTriggerIsNeverAskedAbout(t *testing.T) {
	e, ids := upkeepEngine(t, gainerSrc)
	if e.putTriggersOnStack() {
		t.Fatalf("a lone trigger asked a decision: %+v", e.Pending())
	}
	if e.Pending() != nil {
		t.Fatalf("a lone trigger left a decision pending: %+v", e.Pending())
	}
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Source != ids[0] {
		t.Fatalf("stack = %v, want the one trigger from %d", e.G.Stack, ids[0])
	}
}

// TestTwoSimultaneousTriggersAskTheirController is definition-of-done item 1
// and requirement R1, plus Ruling U2's wire-format claim: the ordering
// decision really is Min == Max == N over that controller's own N triggers,
// so Decision.Validate's existing rules already mean "a permutation".
func TestTwoSimultaneousTriggersAskTheirController(t *testing.T) {
	e, ids := upkeepEngine(t, gainerSrc, drainerSrc)
	if !e.putTriggersOnStack() {
		t.Fatal("two simultaneous triggers did not ask their controller for an order")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder {
		t.Fatalf("pending = %+v, want a trigger-order decision", d)
	}
	if d.Player != 0 {
		t.Fatalf("asked player %d, want the controller (0)", d.Player)
	}
	if d.Min != 2 || d.Max != 2 || len(d.Options) != 2 {
		t.Fatalf("min/max/options = %d/%d/%d, want 2/2/2", d.Min, d.Max, len(d.Options))
	}
	if d.Options[0].Obj != ids[0] || d.Options[1].Obj != ids[1] {
		t.Fatalf("options name %d,%d, want the two trigger sources %d,%d",
			d.Options[0].Obj, d.Options[1].Obj, ids[0], ids[1])
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want nothing placed before the player answered", e.G.Stack)
	}
	// The permutation semantics Ruling U2 leans on, asserted rather than
	// assumed: a repeated index and a short answer are both already rejected.
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0, 0}}); err == nil {
		t.Error("Validate accepted a duplicate index, so this is not a permutation")
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0}}); err == nil {
		t.Error("Validate accepted a short answer, so this is not a permutation")
	}
}

// TestTriggerOrderChoiceDecidesResolutionOrder is Ruling U2's direction
// assertion, and it deliberately asserts the order the two effects RUN rather
// than the order they were pushed -- getting the direction backwards is
// silent, and a push-order assertion would agree with either convention.
//
// Documented direction: choice[0] goes on the stack FIRST and therefore
// resolves LAST. Gainer (+5) and Drainer (lose your whole life total) are
// non-commutative from 20 life:
//
//	Gainer chosen first  -> Drainer resolves first -> 20-20 = 0, then +5 -> 5
//	Drainer chosen first -> Gainer resolves first  -> 20+5 = 25, then -25 -> 0
func TestTriggerOrderChoiceDecidesResolutionOrder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		choices  []int
		wantLife int32
		wantBot  int // which source ends up at Stack[0]
	}{
		{"gainer chosen first resolves last", []int{0, 1}, 5, 0},
		{"drainer chosen first resolves last", []int{1, 0}, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, ids := upkeepEngine(t, gainerSrc, drainerSrc)
			if !e.putTriggersOnStack() {
				t.Fatal("expected an ordering decision")
			}
			submit(t, e, tc.choices...)
			if len(e.G.Stack) != 2 {
				t.Fatalf("stack = %v, want both triggers placed", e.G.Stack)
			}
			// Push order: choice[0] is the bottom of the stack.
			if got := e.G.Obj(e.G.Stack[0]).Source; got != ids[tc.wantBot] {
				t.Errorf("stack bottom is from %d, want %d (choice[0] is pushed first)",
					got, ids[tc.wantBot])
			}
			e.resolveTop()
			e.resolveTop()
			if got := e.G.Players[0].Life; got != tc.wantLife {
				t.Fatalf("life = %d, want %d — the effects resolved in the wrong order",
					got, tc.wantLife)
			}
		})
	}
}

// TestOptionalTriggerNeedsAnExplicitYes is definition-of-done item 3 and
// requirement R2.
func TestOptionalTriggerNeedsAnExplicitYes(t *testing.T) {
	e, ids := upkeepEngine(t, mayGainSrc)
	if !e.putTriggersOnStack() {
		t.Fatal("an optional trigger was resolved without asking anybody")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("pending = %+v, want an optional-trigger decision", d)
	}
	if d.Player != 0 || d.Min != 1 || d.Max != 1 || len(d.Options) != 2 {
		t.Fatalf("decision = player %d min %d max %d %d options, want 0/1/1/2",
			d.Player, d.Min, d.Max, len(d.Options))
	}
	if d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
		t.Fatalf("options = %q,%q, want yes,no in that order",
			d.Options[0].Kind, d.Options[1].Kind)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want nothing placed before the yes", e.G.Stack)
	}
	submit(t, e, 0) // yes
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Source != ids[0] {
		t.Fatalf("stack = %v, want the accepted trigger", e.G.Stack)
	}
	e.resolveTop()
	if got := e.G.Players[0].Life; got != 24 {
		t.Fatalf("life = %d, want 24 — the accepted trigger did not actually resolve", got)
	}
}

// TestDecliningAnOptionalTriggerLeavesTheGameUntouched is definition-of-done
// item 4. Asserted as a whole-game diff rather than "the stack is empty",
// because Ruling T20-d exists precisely because stack-depth assertions missed
// real bugs: a "no" must emit nothing that changes state at all.
func TestDecliningAnOptionalTriggerLeavesTheGameUntouched(t *testing.T) {
	e, _ := upkeepEngine(t, mayGainSrc)
	if !e.putTriggersOnStack() {
		t.Fatal("expected an optional-trigger decision")
	}
	before := e.G.Clone()
	submit(t, e, 1) // no
	if diff := diffGames(before, e.G); diff != "" {
		t.Fatalf("declining an optional trigger changed the game:\n%s", diff)
	}
	if len(e.pendingTriggers) != 0 || e.orderedTriggers != 0 {
		t.Fatalf("queue = %d entries / %d ordered, want the declined trigger discarded",
			len(e.pendingTriggers), e.orderedTriggers)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush {
			t.Fatal("a declined optional trigger still emitted a TriggerPush")
		}
	}
}

// TestOptionalDeciderCanBeSomeoneOtherThanTheController covers the corpus
// finding that OptionalDecider$ is not always "You": 55 of the 1496 T: lines
// that carry it name a different player, and TriggeredCardController (40 of
// them) is the commonest of those.
func TestOptionalDeciderCanBeSomeoneOtherThanTheController(t *testing.T) {
	const wardenSrc = `Name:Warden
ManaCost:2 W
Types:Enchantment
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature | OptionalDecider$ TriggeredCardController | Execute$ TrigGain | TriggerDescription$ its controller may have you gain 2 life
SVar:TrigGain:DB$ GainLife | LifeAmount$ 2 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	onBoard(t, e, 0, wardenSrc)
	// A creature belonging to player 1 enters the battlefield, so the
	// triggering CARD's controller is player 1 while the trigger's own
	// controller is player 0.
	bear := e.G.AddObject(card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	bear.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{bear.ID})
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear.ID,
		From: state.ZHand, To: state.ZBattlefield})

	if !e.putTriggersOnStack() {
		t.Fatal("expected an optional-trigger decision")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("pending = %+v, want an optional-trigger decision", d)
	}
	if d.Player != 1 {
		t.Fatalf("asked player %d, want player 1 (TriggeredCardController), not the trigger's controller", d.Player)
	}
}

// TestTriggerArrivingDuringADecisionCannotCorruptTheQueue is Ruling U3's own
// hazard, built and measured rather than reasoned about. checkStateBased is a
// fixed-point loop that emits events, every emit runs checkTriggers, and
// Submit runs it AFTER handle -- so a creature dying to a state-based action
// really does append to e.pendingTriggers between an ask and its answer.
//
// The board: three simultaneous upkeep triggers for player 0 (the middle one
// in the chosen order optional, so the drain is interrupted with work still
// settled behind it) and a creature already carrying lethal damage, whose
// death fires a fourth trigger while that optional decision is pending.
//
// What must hold afterwards: nothing lost, nothing pushed twice, the settled
// order preserved across the interruption, the late arrival placed AFTER the
// settled group, and the player NOT asked to order the same triggers again.
func TestTriggerArrivingDuringADecisionCannotCorruptTheQueue(t *testing.T) {
	const mournerSrc = `Name:Mourner
ManaCost:B
Types:Enchantment
T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature | Execute$ TrigMourn | TriggerDescription$ gain 8 life
SVar:TrigMourn:DB$ GainLife | LifeAmount$ 8 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	a := onBoard(t, e, 0, gainerSrc)
	b := onBoard(t, e, 0, mayGainSrc)
	c := onBoard(t, e, 0, drainerSrc)
	mourner := onBoard(t, e, 0, mournerSrc)
	doomed := onBoard(t, e, 0, "Name:Doomed\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	e.G.Obj(doomed).Damage = 5 // lethal, but no state-based action has run yet
	e.G.Active = 0

	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	if got := len(e.pendingTriggers); got != 3 {
		t.Fatalf("queued %d triggers, want the three upkeep triggers", got)
	}
	if !e.putTriggersOnStack() {
		t.Fatal("expected an ordering decision for three simultaneous triggers")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder || len(d.Options) != 3 {
		t.Fatalf("pending = %+v, want a three-way ordering decision", d)
	}
	// Discovery order is Gainer, Almsgiver, Drainer. Choose Drainer,
	// Almsgiver, Gainer: the optional one lands in the middle, so exactly one
	// trigger is pushed before the drain stops and exactly one stays settled
	// behind it.
	submit(t, e, 2, 1, 0)

	// Submit ran handle (which pushed Drainer and asked about Almsgiver) and
	// then checkStateBased, which is what killed the creature.
	if d := e.Pending(); d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("pending = %+v, want the optional decision for the middle trigger", d)
	}
	if o := e.G.Obj(doomed); o.Zone != state.ZGraveyard {
		t.Fatalf("doomed creature is in %s, want the graveyard — no state-based action ran", o.Zone)
	}
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Source != c {
		t.Fatalf("stack = %v, want only Drainer (%d) placed so far", e.G.Stack, c)
	}
	// The hazard itself: the queue grew underneath a half-finished drain.
	if got := len(e.pendingTriggers); got != 3 {
		t.Fatalf("queue = %d, want 3 (Almsgiver being asked, Gainer settled, the death trigger newly arrived)", got)
	}
	if e.orderedTriggers != 2 {
		t.Fatalf("orderedTriggers = %d, want 2 — the settled prefix was not preserved", e.orderedTriggers)
	}
	if e.pendingTriggers[2].Source != mourner {
		t.Fatalf("late arrival landed at index %d's source %d, want it appended after the settled group",
			2, e.pendingTriggers[2].Source)
	}

	submit(t, e, 0) // yes to the optional trigger

	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %+v, want the interrupted priority round to have finished", d)
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("queue = %d entries, want fully drained", len(e.pendingTriggers))
	}
	// Nothing lost, nothing duplicated, settled order preserved, late arrival
	// last: the whole of Ruling U3 in one assertion.
	want := []state.ObjID{c, b, a, mourner}
	if len(e.G.Stack) != len(want) {
		t.Fatalf("stack = %v, want four abilities", e.G.Stack)
	}
	for i, src := range want {
		if got := e.G.Obj(e.G.Stack[i]).Source; got != src {
			t.Fatalf("stack[%d] is from %d, want %d (stack = %v)", i, got, src, e.G.Stack)
		}
	}
	// And the effects, not just the depth (Ruling T20-d): resolving the whole
	// stack must run all four.
	for len(e.G.Stack) > 0 {
		e.resolveTop()
	}
	// 20 -> +8 (Mourner) -> +5 (Gainer) -> +4 (Almsgiver) -> -37 (Drainer) = 0.
	if got := e.G.Players[0].Life; got != 0 {
		t.Fatalf("life = %d, want 0 — the four abilities did not all resolve in stack order", got)
	}
}

// TestTriggersOfADepartedControllerAreDropped is CR 800.4a: an ability
// controlled by a player who has left the game ceases to exist. Before Task 27
// these sorted after every living seat and went on the stack anyway.
func TestTriggersOfADepartedControllerAreDropped(t *testing.T) {
	e := newSeats(t, 3)
	onBoard(t, e, 1, gainerSrc)
	onBoard(t, e, 1, drainerSrc)
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	if got := len(e.pendingTriggers); got != 2 {
		t.Fatalf("queued %d triggers, want 2", got)
	}
	e.emit(events.Event{Kind: events.PlayerLost, Player: 1, Text: "test"})
	if e.putTriggersOnStack() {
		t.Fatalf("asked a player who has left the game: %+v", e.Pending())
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want a departed player's triggers to have ceased to exist", e.G.Stack)
	}
}

// TestEliminationDuringATriggerDecisionDoesNotStrandTheEngine is the case the
// brief could not know about. With three seats an elimination does not end the
// game, so a decision pending against the eliminated player can never be
// answered and nothing else will ever run: Advance does nothing while
// e.pending is set, and only that player may Submit. Without
// releaseTriggerDecisionOfDepartedPlayer this test hangs the match forever.
func TestEliminationDuringATriggerDecisionDoesNotStrandTheEngine(t *testing.T) {
	e := newSeats(t, 3)
	onBoard(t, e, 1, gainerSrc)
	onBoard(t, e, 1, mayGainSrc)
	onBoard(t, e, 1, drainerSrc)
	e.G.Active = 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	// Drive player 1 to exactly zero life, but do not run state-based actions
	// yet: the elimination must happen inside the Submit below, between the
	// ask and the answer, which is the only window that exists.
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -20})

	// newSeats already advanced into a priority decision; drop it so the
	// ordering decision below is the one pending.
	e.pending = nil
	if !e.putTriggersOnStack() {
		t.Fatal("expected an ordering decision for player 1")
	}
	if d := e.Pending(); d == nil || d.Player != 1 {
		t.Fatalf("pending = %+v, want an ordering decision for player 1", d)
	}
	submit(t, e, 0, 1, 2) // Gainer, then the optional one, then Drainer

	if e.G.Over {
		t.Fatal("a three-seat game ended on one elimination")
	}
	if !e.G.Players[1].Lost {
		t.Fatal("player 1 was not eliminated by the state-based actions Submit ran")
	}
	d := e.Pending()
	if d == nil {
		t.Fatal("the engine is stranded: no decision pending and nothing left to run it")
	}
	if e.G.Players[d.Player].Lost {
		t.Fatalf("pending decision %s is against player %d, who has left the game", d.Kind, d.Player)
	}
	if d.Kind != decision.KPriority {
		t.Fatalf("pending = %s, want the interrupted priority round to have finished", d.Kind)
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("queue = %d entries, want the departed player's triggers dropped", len(e.pendingTriggers))
	}
	// The match must still be playable, not merely un-stranded.
	if n := passAll(t, e, 6); n == 0 {
		t.Fatal("the match could not take another action after the elimination")
	}
}

// Scholar and Grinder are the replay proof's non-commutative pair: one draws
// the top card of the library and the other mills it, so which resolves first
// decides which ObjID ends up in the hand and which in the graveyard. A replay
// that got the order wrong would be caught by diffGames as a zone difference,
// not merely as a different ability wrapper.
const scholarSrc = `Name:Scholar
ManaCost:U
Types:Enchantment
T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigDraw | TriggerDescription$ draw a card
SVar:TrigDraw:DB$ Draw | NumCards$ 1 | Defined$ You
Oracle:x
`

const grinderSrc = `Name:Grinder
ManaCost:B
Types:Enchantment
T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigMill | TriggerDescription$ mill a card
SVar:TrigMill:DB$ Mill | NumCards$ 1 | Defined$ You
Oracle:x
`

// findAndPlay moves the named card of p's onto the battlefield through
// ordinary logged events, so a log-only replay can follow it. Genesis's
// shuffle decides whether it landed in the hand or the library.
func findAndPlay(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == name {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZHand, To: state.ZBattlefield})
			return id
		}
	}
	for _, id := range e.G.Zone(state.ZLibrary, p) {
		if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == name {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZLibrary, To: state.ZHand})
			e.emit(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZHand, To: state.ZBattlefield})
			return id
		}
	}
	t.Fatalf("%s not found in player %d's hand or library", name, p)
	return 0
}

// driveTriggerGame plays the match forward, passing every priority, answering
// each ordering decision with the next permutation from orders (identity once
// they run out) and each optional decision with the next answer from yesNo
// (declining once they run out). Reports how many of each it answered.
func driveTriggerGame(t *testing.T, e *Engine, limit int, orders [][]int, yesNo []bool) (nOrder, nOpt int) {
	t.Helper()
	for i := 0; i < limit && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			return
		}
		var choices []int
		switch d.Kind {
		case decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			choices = []int{idx}
		case decision.KTriggerOrder:
			if nOrder < len(orders) {
				choices = orders[nOrder]
				if len(choices) != len(d.Options) {
					t.Fatalf("scripted order %v does not fit %d options", choices, len(d.Options))
				}
			} else {
				for k := range d.Options {
					choices = append(choices, k)
				}
			}
			nOrder++
		case decision.KTriggerOptional:
			pick := 1 // no
			if nOpt < len(yesNo) && yesNo[nOpt] {
				pick = 0 // yes
			}
			nOpt++
			choices = []int{pick}
		default:
			t.Fatalf("unexpected decision kind %q", d.Kind)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
			t.Fatalf("submit %s %v: %v", d.Kind, choices, err)
		}
	}
	return
}

// TestReplayFromLogAloneReconstructsOrderedAndOptionalTriggers is Ruling U4.
// A real multi-turn game that exercises both a reordering and an optional
// trigger (once accepted, once declined) is folded from its log ALONE into a
// fresh state.Game -- no rules.Engine behind it, exactly what a real
// replay-from-log does -- and must match the live game exactly.
//
// Both decisions reach replay with no new event kind and no new Event field:
// the ordering choice is carried purely by the ORDER of the TriggerPush
// events, and the optional choice purely by whether a TriggerPush was emitted
// at all. The two controls below are what prove the harness discriminates
// rather than trivially agreeing, and the second of them is aimed at exactly
// that claim: swap two TriggerPush events and the replay must diverge.
func TestReplayFromLogAloneReconstructsOrderedAndOptionalTriggers(t *testing.T) {
	deck0 := append([]*cards.Card{card(t, scholarSrc), card(t, grinderSrc), card(t, mayGainSrc)},
		mountainDeck(t, 37)...)
	cfg := Config{Seed: 11, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck0, mountainDeck(t, 40)}}
	e := New(cfg)
	scholar := findAndPlay(t, e, 0, "Scholar")
	grinder := findAndPlay(t, e, 0, "Grinder")
	alms := findAndPlay(t, e, 0, "Almsgiver")
	e.Advance()

	// Discovery order is battlefield order: Scholar, Grinder, Almsgiver.
	// [2,0,1] puts the optional one first (so it is asked before anything is
	// pushed) and reverses the draw/mill pair relative to discovery order.
	nOrder, nOpt := driveTriggerGame(t, e, 400, [][]int{{2, 0, 1}}, []bool{true, false})
	if nOrder < 2 {
		t.Fatalf("answered %d ordering decisions, want at least 2", nOrder)
	}
	if nOpt < 2 {
		t.Fatalf("answered %d optional decisions, want at least 2 (one yes, one no)", nOpt)
	}

	// The choices really did reach the log, and the effects really landed.
	var pushes []state.ObjID
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush {
			pushes = append(pushes, ev.Obj)
		}
	}
	if len(pushes) < 5 {
		t.Fatalf("TriggerPush sources = %v, want 3 from the accepted upkeep and 2 from the declined one", pushes)
	}
	if pushes[0] != alms || pushes[1] != scholar || pushes[2] != grinder {
		t.Fatalf("first three TriggerPush sources = %v, want %d,%d,%d — the chosen order did not reach the log",
			pushes[:3], alms, scholar, grinder)
	}
	if pushes[3] == alms || pushes[4] == alms {
		t.Fatalf("the declined optional trigger reached the stack: %v", pushes[3:5])
	}

	fresh := replayFromLog(t, cfg, e.L.Events)
	if diff := diffGames(e.G, fresh); diff != "" {
		t.Fatalf("replay from the log alone diverged from the live game:\n%s", diff)
	}

	// Control 1: a truncated log must be detected as a divergence, or this
	// test could pass vacuously.
	truncated := replayFromLog(t, cfg, e.L.Events[:len(e.L.Events)-1])
	if diff := diffGames(e.G, truncated); diff == "" {
		t.Fatal("control: a truncated log should diverge, but diffGames found no difference")
	}

	// Control 2, aimed at this task's own claim: the ORDER of the TriggerPush
	// events is what carries the ordering choice. Swap the first two and the
	// replay must diverge.
	swapped := append([]events.Event(nil), e.L.Events...)
	var first, second = -1, -1
	for i, ev := range swapped {
		if ev.Kind != events.TriggerPush {
			continue
		}
		if first < 0 {
			first = i
		} else {
			second = i
			break
		}
	}
	if first < 0 || second < 0 {
		t.Fatal("control: fewer than two TriggerPush events in the log")
	}
	swapped[first], swapped[second] = swapped[second], swapped[first]
	if diff := diffGames(e.G, replayFromLog(t, cfg, swapped)); diff == "" {
		t.Fatal("control: swapping two TriggerPush events should diverge, but diffGames found no difference — the ordering choice is not actually carried by event order")
	}
}

// TestOrderingDecisionInTheDrawStepDoesNotDrawTwice is the reason
// putTriggersOnStack resumes into grantPriority rather than into
// priorityRound. priorityRound is NOT idempotent: nothing between its
// draw-step draw and the priority emit that follows changes Passes or
// Priority, so re-entering it mid-round -- which is what Advance would do if
// the trigger handlers simply returned and let the engine take another step --
// draws the active player a second card and mills a card off their library
// for free. That failure is completely silent: the stack, the log's event
// kinds, and every decision the client sees are all exactly right.
func TestOrderingDecisionInTheDrawStepDoesNotDrawTwice(t *testing.T) {
	const drawWatchSrc = `Name:Chronicler
ManaCost:U
Types:Enchantment
T:Mode$ ChangesZone | Origin$ Library | Destination$ Hand | ValidCard$ Card | Execute$ TrigGain | TriggerDescription$ gain 1 life
SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You
Oracle:x
`
	e := layerEngine(t)
	onBoard(t, e, 0, drawWatchSrc)
	onBoard(t, e, 0, drawWatchSrc)
	// Stand the engine in turn 2's draw step with the active player yet to
	// draw, which is the one point in a round where priorityRound does
	// irreversible work before it reaches the trigger drain.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDraw})
	e.emit(events.Event{Kind: events.Priority, Player: 0, Amount: 0})
	hand := len(e.G.Zone(state.ZHand, 0))
	lib := len(e.G.Zone(state.ZLibrary, 0))

	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder {
		t.Fatalf("pending = %+v, want the two draw triggers to ask for an order", d)
	}
	submit(t, e, 1, 0)

	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand = %d, want %d — the draw step ran %d times", got, hand+1, got-hand)
	}
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != lib-1 {
		t.Fatalf("library = %d, want %d — the draw step ran more than once", got, lib-1)
	}
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %+v, want the interrupted draw step to have finished", d)
	}
	if len(e.G.Stack) != 2 {
		t.Fatalf("stack = %v, want both draw triggers placed", e.G.Stack)
	}
}
