package rules

// CR 700.13 / Mode$ BecomesTarget: a modal ACTIVATED ability chooses its modes
// at resolution (rules' castModeAsk skips abilities), so the targets its modes
// name are asked mid-resolution and were never recorded as TargetsChosen. The
// ability therefore committed no crime and fired no BecomesTarget trigger at
// all. These two leaves drive REAL corpus cards end to end: the mid-resolution
// ask records the ability's targets exactly once, so the watcher fires once.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// driveModalActivation answers the mode and target asks an activated Charm
// poses as it is activated, then drains the stack. target is the object the
// caller wants named (the mode's only legal target in both leaves).
func driveModalActivation(t *testing.T, e *Engine, modeLabel string, target state.ObjID) {
	t.Helper()
	for i := 0; i < 8 && e.Pending() != nil && e.Pending().Kind != decision.KPriority; i++ {
		d := e.Pending()
		switch d.Kind {
		case decision.KModes:
			idx := -1
			for _, o := range d.Options {
				if o.Label == modeLabel {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("mode %q not offered: %+v", modeLabel, d.Options)
			}
			submitChoices(t, e, idx)
		case decision.KChoose, decision.KTarget:
			idx := -1
			for _, o := range d.Options {
				if o.Obj == target {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("target %d not offered: %+v", target, d.Options)
			}
			submitChoices(t, e, idx)
		default:
			submitChoices(t, e, 0)
		}
	}
	passUntilStackEmpty(t, e, 40)
}

func countKindEvents(e *Engine, kind events.Kind, pred func(events.Event) bool) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == kind && pred(ev) {
			n++
		}
	}
	return n
}

// TestModalActivationCommitsOneCrime drives Cankerbloom's real activated Charm
// ("{1}, Sacrifice Cankerbloom: Choose one — ... Destroy target artifact") at
// an opponent's Sol Ring. The crime is CR 700.13's targeting an opponent's
// permanent, and Hardbristle Bandit's Mode$ CommitCrime is the watcher.
func TestModalActivationCommitsOneCrime(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, ids := becomesTargetOnceBoard(t, reg,
		[]string{"Cankerbloom", "Hardbristle Bandit"},
		[]string{"Sol Ring"})
	canker, bandit, sol := ids["Cankerbloom"], ids["Hardbristle Bandit"], ids["Sol Ring"]

	// Setup-only tap: the crime trigger's payout is Bandit's untap, so an
	// already-untapped Bandit would hide a second firing.
	e.G.Obj(bandit).Tapped = true
	e.G.Active, e.G.Priority = 0, 0
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "C", Amount: 1})
	e.askPriority(0)
	opt := abilityOption(t, e, canker, 0)
	submitChoices(t, e, opt.Index)
	driveModalActivation(t, e, "Destroy target artifact.", sol)

	if got := countKindEvents(e, events.TargetsChosen, func(ev events.Event) bool {
		return len(ev.IDs) == 1 && ev.IDs[0] == sol
	}); got != 1 {
		t.Fatalf("TargetsChosen events naming the artifact = %d, want 1", got)
	}
	if got := countKindEvents(e, events.TriggerPush, func(ev events.Event) bool {
		return ev.Obj == bandit
	}); got != 1 {
		t.Fatalf("Hardbristle Bandit CommitCrime TriggerPush = %d, want 1", got)
	}
	if !e.CommittedCrimeThisTurn(0) {
		t.Fatal("CommittedCrimeThisTurn(0) = false, want true")
	}
	if o := e.G.Obj(sol); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Sol Ring zone = %v, want graveyard", o.Zone)
	}
	if e.G.Obj(bandit).Tapped {
		t.Fatal("Hardbristle Bandit is still tapped: the crime trigger did not resolve")
	}
}

// TestModalActivationBecomesTargetOnce drives Umezawa's Jitte's real activated
// Charm at an opponent's Illusionary Servant ("When CARDNAME becomes the target
// of a spell or ability, sacrifice it"). One activation names one target, so
// the watcher must fire exactly once and the Servant must be sacrificed.
func TestModalActivationBecomesTargetOnce(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, ids := becomesTargetOnceBoard(t, reg,
		[]string{"Umezawa's Jitte"},
		[]string{"Illusionary Servant"})
	jitte, servant := ids["Umezawa's Jitte"], ids["Illusionary Servant"]

	// The Jitte's Charm cost is SubCounter<1/CHARGE>.
	e.G.Obj(jitte).Counters = append(e.G.Obj(jitte).Counters, state.Counter{Kind: "CHARGE", N: 1})
	e.askPriority(0)
	opt := abilityOption(t, e, jitte, 0)
	submitChoices(t, e, opt.Index)
	driveModalActivation(t, e, "Target creature gets -1/-1 until end of turn.", servant)

	if got := countKindEvents(e, events.TargetsChosen, func(ev events.Event) bool {
		return len(ev.IDs) == 1 && ev.IDs[0] == servant
	}); got != 1 {
		t.Fatalf("TargetsChosen events naming the Servant = %d, want 1", got)
	}
	if got := countKindEvents(e, events.TriggerPush, func(ev events.Event) bool {
		return ev.Obj == servant
	}); got != 1 {
		t.Fatalf("Illusionary Servant BecomesTarget TriggerPush = %d, want 1", got)
	}
	if o := e.G.Obj(servant); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Illusionary Servant zone = %v, want graveyard", o.Zone)
	}
}

// TestSpellCommitsOneCrime pins the spell half: a spell's targets are recorded
// at cast time and must stay a single TargetsChosen set, so a crime watcher
// still fires once (and the activated-Charm recording above does not reach a
// spell's stack object).
func TestSpellCommitsOneCrime(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := New(Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	bandit := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Hardbristle Bandit"))
	sol := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Sol Ring"))
	shatter := e.G.AddObject(mustCorpusCard(t, reg, "Shatter"), 0)
	shatter.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{shatter.ID})
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Players[0].Pool[state.MR] = 2
	e.askPriority(0)
	castCardByName(t, e, shatter.ID)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Shatter target ask = %+v, want a target ask", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == sol {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("Sol Ring not offered as a Shatter target: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 40)

	if got := countKindEvents(e, events.TargetsChosen, func(ev events.Event) bool {
		return len(ev.IDs) == 1 && ev.IDs[0] == sol
	}); got != 1 {
		t.Fatalf("TargetsChosen events naming the artifact = %d, want 1", got)
	}
	if got := countKindEvents(e, events.TriggerPush, func(ev events.Event) bool {
		return ev.Obj == bandit
	}); got != 1 {
		t.Fatalf("Hardbristle Bandit CommitCrime TriggerPush = %d, want 1", got)
	}
	if !e.CommittedCrimeThisTurn(0) {
		t.Fatal("CommittedCrimeThisTurn(0) = false, want true")
	}
	if o := e.G.Obj(sol); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Sol Ring zone = %v, want graveyard", o.Zone)
	}
}
