// cr-g55: CR 506.3c / 508.4a-b. A creature put onto the battlefield
// attacking a player who has left the game, or a planeswalker that is no
// longer controlled by that defending player, enters but is never an
// attacking creature. Ninjutsu (CR 702.49c) names its defender -- the one
// the returned creature was attacking -- so the defender is fixed when the
// ability is activated and can change before it resolves. Real corpus ninja
// (Walker of Secret Ways); the attacker is authored.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ninjaAfter builds a seats-player game (seat 0 starts), optionally gives
// seat 1 a loyal planeswalker, declares seat 0's Haste Bear at seat 1 (or at
// the planeswalker), activates Walker of Secret Ways' ninjutsu returning the
// unblocked Bear, runs between while the ability is on the stack, drains
// the stack, and returns the engine, the ninja and the planeswalker (0 when
// none).
func ninjaAfter(t *testing.T, seats int, atWalker bool, between func(e *Engine, pw state.ObjID)) (*Engine, *state.Object, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	ninja := searchCorpusCard(t, reg, "Walker of Secret Ways")
	if d := ninja.Link(); len(d) != 0 {
		t.Fatalf("link Walker of Secret Ways: %v", d)
	}
	s0 := []*cards.Card{ninja, card(t, ninjutsuBearSrc)}
	s0 = append(s0, mountainDeck(t, 38)...)
	names := []string{"ninja", "defender", "third"}[:seats]
	decks := [][]*cards.Card{s0}
	for len(decks) < seats {
		decks = append(decks, mountainDeck(t, 40))
	}
	e := New(seatZeroStart(Config{Seed: 9403, Names: names, Decks: decks, Tokens: map[string]*cards.Card{}}))
	e.Advance()
	toMain1(t, e)
	var pw state.ObjID
	if atWalker {
		pw = onBoard(t, e, 1, combatPlaneswalker)
		e.emit(events.Event{Kind: events.CounterChange, Obj: pw, Counter: "LOYALTY", Amount: 5})
	}
	ninjaID := searchMoveByName(t, e, "Walker of Secret Ways", state.ZHand)
	bear := putCreature(t, e, 0, ninjutsuBearSrc)
	e.priorityRound()
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("declare attackers decision = %+v", d)
	}
	var chosen []decision.Option
	for _, opt := range d.Options {
		if opt.Obj == bear && opt.Player == 1 && opt.Battle == pw {
			chosen = append(chosen, opt)
		}
	}
	if len(chosen) != 1 {
		t.Fatalf("attack option for the Bear at seat 1 / battle %d: %d found in %+v", pw, len(chosen), d.Options)
	}
	e.finishAttackers(chosen, 0)
	driveToBlockersPriority(t, e, 0)
	fundPool(t, e, "CU") // Walker of Secret Ways' ninjutsu cost is {1}{U}
	opt := abilityFor(t, e, 0, ninjaID)
	if opt == nil {
		t.Fatalf("ninjutsu ability not offered: %+v", e.Pending())
	}
	submitChoices(t, e, opt.Index)
	if d = e.Pending(); d == nil || d.Kind != decision.KChoose {
		t.Fatalf("ninjutsu return ask = %+v", d)
	}
	submitChoices(t, e, indexOfObjOption(d, bear))
	if len(e.G.Stack) == 0 {
		t.Fatal("the ninjutsu ability is not on the stack")
	}
	between(e, pw)
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(ninjaID)
	if o == nil || o.Zone != state.ZBattlefield || !o.Tapped {
		t.Fatalf("Walker of Secret Ways must enter the battlefield tapped: %+v", o)
	}
	return e, o, pw
}

// TestCR508_4bNinjaDoesNotAttackADepartedPlayer: three seats; the Bear
// attacks seat 1 and seat 1 leaves the game with the ninjutsu ability on the
// stack. The ninja enters tapped but is not attacking. Guard: without the
// departure it attacks seat 1.
func TestCR508_4bNinjaDoesNotAttackADepartedPlayer(t *testing.T) {
	t.Parallel()
	_, ninja, _ := ninjaAfter(t, 3, false, func(e *Engine, _ state.ObjID) {
		e.emit(events.Event{Kind: events.PlayerLost, Player: 1, Text: "test departure"})
	})
	if ninja.IsAttacking {
		t.Fatalf("CR 508.4b: ninja attacking=%v defender=%d after its defender left the game, want not attacking", ninja.IsAttacking, ninja.Attacking)
	}
	_, ninja, _ = ninjaAfter(t, 3, false, func(*Engine, state.ObjID) {})
	if !ninja.IsAttacking || ninja.Attacking != 1 {
		t.Fatalf("guard: ninja attacking=%v defender=%d, want attacking seat 1", ninja.IsAttacking, ninja.Attacking)
	}
}

// TestCR508_4bNinjaDoesNotAttackAPlaneswalkerThatChangedControl: the Bear
// attacks seat 1's planeswalker; seat 0 gains control of the planeswalker
// with the ability on the stack, so it is no longer controlled by a
// defending player and the ninja is not attacking it.
func TestCR508_4bNinjaDoesNotAttackAPlaneswalkerThatChangedControl(t *testing.T) {
	t.Parallel()
	_, ninja, _ := ninjaAfter(t, 2, true, func(e *Engine, pw state.ObjID) {
		e.emit(events.Event{Kind: events.ControlChange, Obj: pw, Player: 0, Text: "GainControl"})
	})
	if ninja.IsAttacking {
		t.Fatalf("CR 508.4b: ninja attacking=%v battle=%d at a planeswalker its defender no longer controls, want not attacking", ninja.IsAttacking, ninja.AttackingBattle)
	}
}
