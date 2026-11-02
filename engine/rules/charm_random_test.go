package rules

// param:api:Charm.Random (Random$ True / Random$ Compare with
// RandomCompareSVar$/RandomCompare$) pinned END TO END on the REAL corpus
// Typhoid Mary, Fractured ("choose one at random. If you discarded a card
// this turn, you choose one instead"): the mode is picked at random with the
// engine's seeded rng while the comparison holds (zero discards this turn --
// RandomCompare$ LT1 over SVar Y = CardsDiscardedThisTurn), and a discard
// reverts the choice to the real KModes ask. The direction is the card's own
// oracle. An authored Random$ True fixture
// pins the always-random spelling, and an unresolvable comparison pins the
// fail-to-the-ask direction. No Forge .txt text is committed; the card comes
// from the compiled corpus registry.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// typhoidEngine seeds the REAL corpus Typhoid Mary, Fractured into seat 0's
// deck (so genesis creates it and the log replays), bridges it onto the
// battlefield ready to attack, and returns the engine at turn 1 with seat 0
// active in the declare-attackers step.
func typhoidEngine(t *testing.T, seed uint64) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	mary, ok := reg.Lookup("Typhoid Mary, Fractured")
	if !ok {
		t.Fatal("corpus fixture: Typhoid Mary, Fractured missing")
	}
	e := New(Config{Seed: seed, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append([]*cards.Card{mary}, mountainDeck(t, 39)...), mountainDeck(t, 40)},
		Tokens: reg.Tokens})
	e.Advance()
	id := moveByName(t, e, 0, "Typhoid Mary, Fractured", state.ZBattlefield)
	if id == 0 {
		t.Fatal("Typhoid Mary was not placed on the battlefield")
	}
	e.G.Obj(id).SummonSick = false
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	return e, id
}

// randomCharmNote returns the text of the random-pick Note ("" when none was
// recorded) and counts how many were recorded.
func randomCharmNotes(evs []events.Event) (string, int) {
	text, n := "", 0
	for _, ev := range evs {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "chose a mode at random: ") {
			text = ev.Text
			n++
		}
	}
	return text, n
}

// TestTyphoidMaryRandomPickWithoutADiscard is the random branch: no discard
// this turn, so the LT1 comparison holds and the pick must be RANDOM -- no
// KModes decision may be posed, exactly one mode must run, and the Note must
// name it. Which mode the seed selects is pinned.
func TestTyphoidMaryRandomPickWithoutADiscard(t *testing.T) {
	t.Parallel()
	e, mary := typhoidEngine(t, 1)
	// Precondition for the branch: the comparison must hold here (zero
	// discards this turn), so this test really exercises the random pick.
	if got := e.CardsDiscardedThisTurn(0); got != 0 {
		t.Fatalf("CardsDiscardedThisTurn(0) = %d, want 0 (no discard: the random branch)", got)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	oppLife, myLife := e.G.Players[1].Life, e.G.Players[0].Life
	before := len(e.L.Events)

	resolveAttackPump(t, e, mary)

	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("the random branch posed a %s decision, want none: %+v", d.Kind, d)
	}
	note, notes := randomCharmNotes(e.L.Events[before:])
	if notes != 1 {
		t.Fatalf("random-pick Notes = %d, want 1", notes)
	}
	tokens := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken {
			tokens++
		}
	}
	drawn := len(e.G.Zone(state.ZHand, 0)) - hand
	drained := oppLife - e.G.Players[1].Life
	gained := e.G.Players[0].Life - myLife
	ran := 0
	which := ""
	switch {
	case tokens > 0:
		ran, which = 1, "token"
	case drawn > 0:
		ran, which = 1, "draw"
	case drained == 2 && gained == 2:
		ran, which = 1, "drain"
	}
	if ran != 1 {
		t.Fatalf("no mode ran (tokens %d, drawn %d, drained %d, gained %d) -- the random pick resolved nothing", tokens, drawn, drained, gained)
	}
	if !strings.Contains(note, which) && !(which == "drain" && strings.Contains(note, "Bloody Mary")) &&
		!(which == "token" && strings.Contains(note, "Treasure")) && !(which == "draw" && strings.Contains(note, "Draw a card")) {
		t.Fatalf("the Note %q does not name the mode that ran (%s)", note, which)
	}
	// The seeded pin: seed 1's rng stream selects exactly this mode. If the
	// engine's rng discipline changes, this pin fails -- update it only with
	// the rng change named.
	if which != "draw" {
		t.Fatalf("seed 1's random pick ran %s, want the pinned draw mode (Note %q)", which, note)
	}
}

// TestTyphoidMaryAskReplacesThePickAfterADiscard is the player-choose branch,
// with the no-discard control that makes the branch split fail-able: without
// the fix BOTH shapes pose the ordinary ask, and it is the discard that must
// move the branch. After the discard the comparison fails, so the REAL
// KModes ask is posed; the answered mode runs and the other two do not.
func TestTyphoidMaryAskReplacesThePickAfterADiscard(t *testing.T) {
	t.Parallel()
	// The control: the same seed, NO discard -- the random branch ran above,
	// but pin its gate here too so this test fails when the comparison is
	// unread (both branches would then ask identically).
	ctl, ctlMary := typhoidEngine(t, 1)
	resolveAttackPump(t, ctl, ctlMary)
	if d := ctl.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("control (no discard) posed a %s decision, want a random pick: %+v", d.Kind, d)
	}

	e, mary := typhoidEngine(t, 1)
	// Precondition for the branch: a real discard this turn, through the
	// engine's canonical producer event (events.Discard), so the provenance
	// counter the comparison reads has counted it.
	var handCard state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		handCard = id
		break
	}
	if handCard == 0 {
		t.Fatal("seat 0 has no card in hand to discard")
	}
	e.emit(events.Discard(handCard, 0))
	if got := e.CardsDiscardedThisTurn(0); got != 1 {
		t.Fatalf("CardsDiscardedThisTurn(0) = %d, want 1 (the discard must gate the branch)", got)
	}

	// Declare the attack and queue the trigger: the failed comparison poses
	// the REAL CR 603.3c placement ask (no random pick was consumed), the
	// ordinary flow this engine gives every modal trigger.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{mary}})
	e.putTriggersOnStack()
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("after a discard the modes ask was not posed: %+v", d)
	}
	if len(d.Options) != 3 {
		t.Fatalf("mode options = %d, want 3: %+v", len(d.Options), d.Options)
	}
	drawIdx := -1
	for _, o := range d.Options {
		if strings.Contains(o.Label, "Draw a card") {
			drawIdx = o.Index
		}
	}
	if drawIdx < 0 {
		t.Fatalf("the draw mode was not offered: %+v", d.Options)
	}
	if _, notes := randomCharmNotes(e.L.Events); notes != 0 {
		t.Fatal("the player-choose branch recorded a random-pick Note")
	}

	hand := len(e.G.Zone(state.ZHand, 0))
	oppLife, myLife := e.G.Players[1].Life, e.G.Players[0].Life
	submitChoices(t, e, drawIdx)
	e.resolveTop()
	passUntilStackEmpty(t, e, 20)

	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand = %d, want %d (the answered draw mode must run)", got, hand+1)
	}
	if e.G.Players[1].Life != oppLife {
		t.Errorf("opponent life moved to %d, want %d (only the chosen mode may run)", e.G.Players[1].Life, oppLife)
	}
	if e.G.Players[0].Life != myLife {
		t.Errorf("seat-0 life moved to %d, want %d (only the chosen mode may run)", e.G.Players[0].Life, myLife)
	}
	tokens := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken {
			tokens++
		}
	}
	if tokens != 0 {
		t.Errorf("%d tokens created, want 0 (only the chosen mode may run)", tokens)
	}
}

// randomCharmScript is the authored always-random fixture, in the corpus
// trigger shape (never a copied Forge .txt): one attack trigger whose Charm
// carries Random$ True.
const randomCharmScript = "Name:Random Charm Hound\nManaCost:2 R\nTypes:Creature Dog\nPT:2/2\n" +
	"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigCharm | TriggerZones$ Battlefield | TriggerDescription$ Whenever CARDNAME attacks, choose one at random.\n" +
	"SVar:TrigCharm:DB$ Charm | Random$ True | Choices$ ThenDraw,DBDrain\n" +
	"SVar:ThenDraw:DB$ Draw | Defined$ You | NumCards$ 1\n" +
	"SVar:DBDrain:DB$ LoseLife | Defined$ Opponent | LifeAmount$ 1 | SpellDescription$ Each opponent loses 1 life.\n" +
	"Oracle:x\n"

// TestCharmRandomTrueNeverAsks pins the plain `Random$ True` spelling on an
// authored fixture: the mode is picked at random, no KModes ask is ever
// posed, and exactly one mode's effect runs.
func TestCharmRandomTrueNeverAsks(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	id := onBoardCard(t, e, 0, card(t, randomCharmScript))
	e.G.Obj(id).SummonSick = false
	hand := len(e.G.Zone(state.ZHand, 0))
	oppLife := e.G.Players[1].Life

	resolveAttackPump(t, e, id)

	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("Random$ True posed a %s decision, want none: %+v", d.Kind, d)
	}
	tokens := 0
	for _, tid := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(tid); o != nil && o.IsToken {
			tokens++
		}
	}
	drawn := len(e.G.Zone(state.ZHand, 0)) - hand
	drained := oppLife - e.G.Players[1].Life
	ran := 0
	switch {
	case drawn == 1:
		ran = 1
	case drained == 1:
		ran = 1
	case tokens > 0:
		ran = 1
	}
	if ran != 1 {
		t.Fatalf("exactly one mode must run at random (drawn %d, drained %d)", drawn, drained)
	}
}

// TestCharmRandomUnresolvableComparisonAsks pins the fail direction for a
// comparison this build cannot evaluate: RandomCompareSVar$ naming a missing
// SVar must revert to the real ask, never to a fake random pick.
func TestCharmRandomUnresolvableComparisonAsks(t *testing.T) {
	t.Parallel()
	script := "Name:Broken Compare Hound\nManaCost:2 R\nTypes:Creature Dog\nPT:2/2\n" +
		"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigCharm | TriggerZones$ Battlefield | TriggerDescription$ Whenever CARDNAME attacks, choose one at random.\n" +
		"SVar:TrigCharm:DB$ Charm | Random$ Compare | RandomCompareSVar$ MissingSVar | RandomCompare$ LT1 | Choices$ ThenDraw,DBDrain\n" +
		"SVar:ThenDraw:DB$ Draw | Defined$ You | NumCards$ 1\n" +
		"SVar:DBDrain:DB$ LoseLife | Defined$ Opponent | LifeAmount$ 1 | SpellDescription$ Each opponent loses 1 life.\n" +
		"Oracle:x\n"
	e := combatEngine(t)
	id := onBoardCard(t, e, 0, card(t, script))
	e.G.Obj(id).SummonSick = false

	resolveAttackPump(t, e, id)

	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("an unresolvable comparison must fall to the KModes ask, got %+v", d)
	}
	// Decline nothing further; the ask's existence is the pin.
}

// randomCharmEngine is combatEngine at a chosen seed, so a test can cover the
// draw's outcomes.
func randomCharmEngine(t *testing.T, seed uint64) *Engine {
	t.Helper()
	e := New(seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}))
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	return e
}

// TestRandomCharmModeIsChosenAsTheTriggerGoesOnTheStack is CR 700.2b on a mode
// that targets: the draw and the target ask both come before anyone can
// respond, and the drawn mode resolves against that target.
func TestRandomCharmModeIsChosenAsTheTriggerGoesOnTheStack(t *testing.T) {
	t.Parallel()
	script := "Name:Random Target Hound\nManaCost:2 R\nTypes:Creature Dog\nPT:2/2\n" +
		"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigCharm | TriggerZones$ Battlefield | TriggerDescription$ Whenever CARDNAME attacks, choose one at random.\n" +
		"SVar:TrigCharm:DB$ Charm | Random$ True | Choices$ DBDraw,DBBolt\n" +
		"SVar:DBDraw:DB$ Draw | Defined$ You | NumCards$ 1 | SpellDescription$ Draw a card.\n" +
		"SVar:DBBolt:DB$ DealDamage | ValidTgts$ Player | NumDmg$ 3 | SpellDescription$ Deal 3 damage to target player.\n" +
		"Oracle:x\n"
	targeted := 0
	for seed := uint64(1); seed <= 16; seed++ {
		e := randomCharmEngine(t, seed)
		id := onBoardCard(t, e, 0, card(t, script))
		e.G.Obj(id).SummonSick = false
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{id}})
		e.putTriggersOnStack()
		if len(e.G.Stack) != 1 {
			t.Fatalf("seed %d: stack %v, want the trigger", seed, e.G.Stack)
		}
		chosen := e.G.Obj(e.G.Stack[0]).ChosenModes
		if len(chosen) != 1 {
			t.Fatalf("seed %d: the trigger is on the stack with modes %v, want one drawn", seed, chosen)
		}
		if chosen[0] != "DBBolt" {
			continue
		}
		targeted++
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("seed %d: the drawn targeted mode posed %+v, want its target ask", seed, d)
		}
		life := e.G.Players[1].Life
		pick := -1
		for _, o := range d.Options {
			if o.Player == 1 {
				pick = o.Index
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{pick}}); err != nil {
			t.Fatalf("seed %d: answering the target ask: %v", seed, err)
		}
		e.resolveTop()
		if got := life - e.G.Players[1].Life; got != 3 {
			t.Fatalf("seed %d: the targeted player lost %d, want 3", seed, got)
		}
	}
	if targeted == 0 {
		t.Fatal("no seed in 1..16 drew the targeted mode")
	}
}

// TestRandomCharmDrawsOnlyChoosableModes is CR 700.2b's "that mode can't be
// chosen": with no creature to target, every seed draws the other mode.
func TestRandomCharmDrawsOnlyChoosableModes(t *testing.T) {
	t.Parallel()
	script := "Name:Random Choosy Hound\nManaCost:2 R\nTypes:Creature Dog\nPT:2/2\n" +
		"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigCharm | TriggerZones$ Battlefield | TriggerDescription$ Whenever CARDNAME attacks, choose one at random.\n" +
		"SVar:TrigCharm:DB$ Charm | Random$ True | Choices$ DBDraw,DBBolt\n" +
		"SVar:DBDraw:DB$ Draw | Defined$ You | NumCards$ 1 | SpellDescription$ Draw a card.\n" +
		"SVar:DBBolt:DB$ DealDamage | ValidTgts$ Creature.OppCtrl | NumDmg$ 3 | SpellDescription$ Deal 3 damage to target creature an opponent controls.\n" +
		"Oracle:x\n"
	for seed := uint64(1); seed <= 16; seed++ {
		e := randomCharmEngine(t, seed)
		id := onBoardCard(t, e, 0, card(t, script))
		e.G.Obj(id).SummonSick = false
		hand := len(e.G.Zone(state.ZHand, 0))
		resolveAttackPump(t, e, id)
		if got := len(e.G.Zone(state.ZHand, 0)) - hand; got != 1 {
			t.Fatalf("seed %d: drew %d, want the only choosable mode's 1", seed, got)
		}
	}
}

// TestSummonMagusSistersChapterDrawsWithItsTarget runs the corpus Saga: each
// chapter's random mode is drawn with the chapter ability going on the stack,
// and that mode's target is asked before the ability can be responded to.
func TestSummonMagusSistersChapterDrawsWithItsTarget(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Summon: Magus Sisters")}, []*cards.Card{})
	moveByName(t, e, 0, "Summon: Magus Sisters", state.ZBattlefield)
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack %v, want chapter I", e.G.Stack)
	}
	if chosen := e.G.Obj(e.G.Stack[0]).ChosenModes; len(chosen) != 1 {
		t.Fatalf("chapter I is on the stack with modes %v, want one drawn", chosen)
	}
	if d := e.Pending(); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("chapter I posed %+v, want the drawn mode's target ask", d)
	}
}
