package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The AddsKeywords$ mana-spend rider (cr-g144; CR 106.6, 400.7a, 702.10).
// Inline script SHAPES of Carnelian Orb of Dragonkind and Hall of the Bandit
// Lord; never a .cards/ .txt.

const akOrbSrc = `Name:Carnelian Orb of Dragonkind
ManaCost:2
Types:Artifact
A:AB$ Mana | Cost$ T | Produced$ R | AddsKeywords$ Haste | AddsKeywordsValid$ Spell.Dragon+Creature | AddsKeywordsUntil$ UntilEOT | SpellDescription$ Add {R}. If that mana is spent on a Dragon creature spell, it gains haste until end of turn.
Oracle:x
`

const akDragonSrc = `Name:Shivan Dragon
ManaCost:4 R R
Types:Creature Dragon
PT:5/5
Oracle:x
`

const akBearSrc = `Name:Grizzly Bears
ManaCost:1 G
Types:Creature Bear
PT:2/2
Oracle:x
`

const akHallSrc = `Name:Hall of the Bandit Lord
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ T PayLife<3> | Produced$ C | AddsKeywords$ Haste | AddsKeywordsValid$ Spell.Creature | SpellDescription$ Add {C}. If that mana is spent on a creature spell, it gains haste.
Oracle:x
`

const akGolemSrc = `Name:Plain Golem
ManaCost:1
Types:Artifact Creature Golem
PT:1/1
Oracle:x
`

func akNoNote(t *testing.T, e *Engine) {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "AddsKeywords") {
			t.Fatalf("unexpected AddsKeywords note: %q", ev.Text)
		}
	}
}

// akBeginNextTurn passes through the rest of the turn (declaring no
// attackers) to the opponent's next Main1, so the cleanup step's
// until-end-of-turn expiry runs.
func akBeginNextTurn(t *testing.T, e *Engine) {
	t.Helper()
	turn := e.G.Turn
	for i := 0; i < 400 && !(e.G.Turn == turn+1 && e.G.Step == state.StepMain1); i++ {
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while passing the turn")
		}
		if d.Kind == decision.KPriority {
			passOnce(t, e)
			continue
		}
		submitChoices(t, e)
	}
	if e.G.Turn != turn+1 {
		t.Fatalf("turn = %d, want %d", e.G.Turn, turn+1)
	}
}

func TestCarnelianOrbManaGivesDragonHaste(t *testing.T) {
	t.Parallel()
	e, cfg := riderGame(t, 701, card(t, akOrbSrc), card(t, akDragonSrc))
	orb := moveToBattlefieldByName(t, e, 0, "Carnelian Orb of Dragonkind")
	dragon := moveSeededToHand(t, e, 0, "Shivan Dragon")
	e.priorityRound()
	activateMana(t, e, orb)
	if !e.G.Obj(orb).Tapped || e.G.Players[0].Pool[state.MR] != 1 || len(e.G.Players[0].RestrictedMana) != 1 {
		t.Fatalf("precondition: Orb tapped=%v pool R=%d batches=%d, want tapped, 1, 1",
			e.G.Obj(orb).Tapped, e.G.Players[0].Pool[state.MR], len(e.G.Players[0].RestrictedMana))
	}
	addMana(t, e, 0, "RRRRR")
	castSeeded(t, e, dragon)
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(dragon); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Shivan Dragon = %+v, want on the battlefield", o)
	}
	if len(e.G.Players[0].RestrictedMana) != 0 {
		t.Fatalf("precondition: the Orb's batch was not spent: %+v", e.G.Players[0].RestrictedMana)
	}
	if !e.HasKeyword(dragon, "Haste") {
		t.Fatal("Shivan Dragon has no haste after the Orb's mana paid for it")
	}
	akNoNote(t, e)
	akBeginNextTurn(t, e)
	if e.HasKeyword(dragon, "Haste") {
		t.Fatal("the UntilEOT haste survived the turn boundary")
	}
	replayCheck(t, e, cfg)
}

func TestCarnelianOrbManaOnNonDragonGrantsNothing(t *testing.T) {
	t.Parallel()
	e, cfg := riderGame(t, 702, card(t, akOrbSrc), card(t, akBearSrc))
	orb := moveToBattlefieldByName(t, e, 0, "Carnelian Orb of Dragonkind")
	bears := moveSeededToHand(t, e, 0, "Grizzly Bears")
	e.priorityRound()
	activateMana(t, e, orb)
	if e.G.Players[0].Pool[state.MR] != 1 || len(e.G.Players[0].RestrictedMana) != 1 {
		t.Fatalf("precondition: pool R=%d batches=%d, want 1, 1", e.G.Players[0].Pool[state.MR], len(e.G.Players[0].RestrictedMana))
	}
	addMana(t, e, 0, "G")
	castSeeded(t, e, bears)
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(bears); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Grizzly Bears = %+v, want on the battlefield", o)
	}
	if len(e.G.Players[0].RestrictedMana) != 0 {
		t.Fatalf("precondition: the Orb's {R} was not spent on the Bears: %+v", e.G.Players[0].RestrictedMana)
	}
	if e.HasKeyword(bears, "Haste") {
		t.Fatal("a non-Dragon spell gained haste from the Orb's mana")
	}
	replayCheck(t, e, cfg)
}

func TestCarnelianOrbFundsTheCastOffer(t *testing.T) {
	t.Parallel()
	e, cfg := riderGame(t, 703, card(t, akOrbSrc), card(t, akDragonSrc))
	orb := moveToBattlefieldByName(t, e, 0, "Carnelian Orb of Dragonkind")
	dragon := moveSeededToHand(t, e, 0, "Shivan Dragon")
	addMana(t, e, 0, "RRRRR")
	if e.G.Obj(orb).Tapped || e.G.Players[0].Pool[state.MR] != 5 {
		t.Fatalf("precondition: Orb tapped=%v pool R=%d, want untapped, 5", e.G.Obj(orb).Tapped, e.G.Players[0].Pool[state.MR])
	}
	// The planner funds the cast from the untapped Orb: the Dragon costs six
	// and the pool holds five.
	d := e.Pending()
	d.PaymentActions = e.PaymentActionsForPriority(0, d.Seq)
	var act *decision.PaymentAction
	for i := range d.PaymentActions {
		if d.PaymentActions[i].Cast.Object == dragon {
			act = &d.PaymentActions[i]
		}
	}
	if act == nil || len(act.Plans) == 0 {
		t.Fatalf("no payment plan offered for Shivan Dragon: %+v", d.PaymentActions)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Payment: &decision.PaymentSelection{ActionID: act.ID, Plan: act.Plans[0]}}); err != nil {
		t.Fatalf("submit the planned cast: %v", err)
	}
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(dragon); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Shivan Dragon = %+v, want on the battlefield", o)
	}
	if !e.G.Obj(orb).Tapped {
		t.Fatal("the payment did not tap the Orb")
	}
	if !e.HasKeyword(dragon, "Haste") {
		t.Fatal("Shivan Dragon has no haste after the Orb funded its cast")
	}
	replayCheck(t, e, cfg)
}

func TestHallOfTheBanditLordHasteIsPermanent(t *testing.T) {
	t.Parallel()
	e, cfg := riderGame(t, 704, card(t, akHallSrc), card(t, akGolemSrc))
	hall := moveToBattlefieldByName(t, e, 0, "Hall of the Bandit Lord")
	golem := moveSeededToHand(t, e, 0, "Plain Golem")
	e.priorityRound()
	activateMana(t, e, hall)
	if e.G.Players[0].Pool[state.MC] != 1 || len(e.G.Players[0].RestrictedMana) != 1 {
		t.Fatalf("precondition: pool C=%d batches=%d, want 1, 1", e.G.Players[0].Pool[state.MC], len(e.G.Players[0].RestrictedMana))
	}
	castSeeded(t, e, golem)
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(golem); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Plain Golem = %+v, want on the battlefield", o)
	}
	if !e.HasKeyword(golem, "Haste") {
		t.Fatal("Plain Golem has no haste after the Hall's mana paid for it")
	}
	akBeginNextTurn(t, e)
	if !e.HasKeyword(golem, "Haste") {
		t.Fatal("the Hall's haste (no AddsKeywordsUntil$) did not survive the turn boundary")
	}
	replayCheck(t, e, cfg)
}
