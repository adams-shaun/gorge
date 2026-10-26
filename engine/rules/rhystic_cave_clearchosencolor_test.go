package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRhysticCaveClearsChosenColor drives the real Rhystic Cave: choose blue,
// decline the unless-pay ask, and the Mana link adds {U} (it read the colour)
// before DBCleanup's ClearChosenColor$ empties the recorded choice.
func TestRhysticCaveClearsChosenColor(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := stealEngine(t, 741)
	cave := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Rhystic Cave"))
	e.G.Obj(cave).SummonSick = false
	if e.G.Obj(cave).ChosenColor != "" || e.G.Players[0].Pool[state.MU] != 0 {
		t.Fatal("precondition: cave already has a colour or the pool already holds {U}")
	}
	e.askPriority(0)
	act := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "activate" && o.Obj == cave {
			act = o.Index
		}
	}
	if act < 0 {
		t.Fatalf("no activate option for Rhystic Cave: %+v", e.Pending().Options)
	}
	submitChoices(t, e, act)

	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "choosecolor" {
			blue := optionByLabel(d.Options, "Blue")
			if blue < 0 {
				t.Fatalf("no Blue option: %+v", d.Options)
			}
			submitChoices(t, e, blue)
			break
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision %+v", d)
		}
		submitChoices(t, e, passIndex(t, d))
	}
	d := passUntilNonPriority(t, e, 8)
	if d == nil || d.ResumeKind != "unless_pay" {
		t.Fatalf("pending = %+v, want the unless-pay ask", d)
	}
	// Mid-chain: the choice is still recorded for the Mana link to read.
	if got := e.G.Obj(cave).ChosenColor; got != "U" {
		t.Fatalf("at the unless-pay ask ChosenColor = %q, want U", got)
	}
	// UnlessPayer$ Player asks every player in turn; all decline.
	for i := 0; i < 2; i++ {
		answerUnlessPay(t, e, false)
	}
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision %+v", d)
		}
		submitChoices(t, e, passIndex(t, d))
	}
	if got := e.G.Players[0].Pool[state.MU]; got != 1 {
		t.Fatalf("pool {U} = %d, want 1 (Mana read the chosen colour)", got)
	}
	if got := e.G.Obj(cave).ChosenColor; got != "" {
		t.Fatalf("after the chain ChosenColor = %q, want cleared", got)
	}
}
