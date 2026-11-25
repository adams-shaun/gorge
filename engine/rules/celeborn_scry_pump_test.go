package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Celeborn the Wise: "Whenever you attack with one or more Elves, scry 1"
// and "Whenever you scry, Celeborn gets +1/+1 until end of turn for each card
// you looked at while scrying this way" (`SVar:X:TriggerCount$ScryNum`).
// CR 701.22a: scry 1 looks at one card, so the pump is +1/+1 whatever the
// player does with it (here: keeps it on top, so the bottom count is 0 and
// a ScryBottom-sized pump would read 0).

// celebornDrive answers every decision with the first legal shape (pass,
// keep every arranged card on top, the first trigger order / choice) until
// stop reports true.
func celebornDrive(t *testing.T, e *Engine, limit int, stop func() bool) {
	t.Helper()
	for i := 0; i < limit; i++ {
		if stop() {
			return
		}
		if e.G.Over {
			t.Fatal("game ended while driving")
		}
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch d.Kind {
		case decision.KPriority:
			passPriorityOnce(t, e)
		case decision.KAttackers, decision.KBlockers:
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player}); err != nil {
				t.Fatalf("submit %s: %v", d.Kind, err)
			}
		case decision.KArrange, decision.KTriggerOrder:
			choices := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				choices = append(choices, o.Index)
			}
			submitChoices(t, e, choices...)
		case decision.KChoose:
			submitChoices(t, e, d.Options[0].Index)
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
	}
	t.Fatalf("stop condition not reached within %d decisions", limit)
}

func TestCelebornScryPumpsPerCardLooked(t *testing.T) {
	e := New(Config{Seed: 1209, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	celeborn := onBoardCard(t, e, 0, corpusAlternativeCard(t, "Celeborn the Wise"))
	elf := onBoardCard(t, e, 0, corpusAlternativeCard(t, "Pelt Collector"))
	e.G.Obj(celeborn).SummonSick = false
	e.G.Obj(elf).SummonSick = false
	if p, x := e.Power(celeborn), e.Toughness(celeborn); p != 3 || x != 3 {
		t.Fatalf("precondition: Celeborn is %d/%d, want 3/3", p, x)
	}
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	turn := e.G.Turn
	e.askAttackers()
	submitAttackersOnly(t, e, elf)

	scried := func() bool {
		for _, ev := range e.L.Events {
			if ev.Kind == events.Scry && ev.Player == 0 {
				return true
			}
		}
		return false
	}
	celebornDrive(t, e, 200, scried)
	celebornDrive(t, e, 200, func() bool { return len(e.G.Stack) == 0 })
	if p, x := e.Power(celeborn), e.Toughness(celeborn); p != 4 || x != 4 {
		t.Fatalf("after scry 1 Celeborn is %d/%d, want 4/4 (one card looked at)", p, x)
	}
	celebornDrive(t, e, 400, func() bool { return e.G.Turn != turn })
	if p, x := e.Power(celeborn), e.Toughness(celeborn); p != 3 || x != 3 {
		t.Fatalf("after cleanup Celeborn is %d/%d, want 3/3", p, x)
	}
}
