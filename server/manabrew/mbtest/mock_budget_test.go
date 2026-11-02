package mbtest

import (
	"testing"

	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// A free ability stays offered after each use, so the client stops acting once
// its budget is spent, and acts again on the next turn.
func TestMockStopsActingForTheTurnAtItsBudget(t *testing.T) {
	in := mb.ChooseActionInput{Actions: []mb.AvailableAction{{ID: "opt-0", Type: "activateAbility"}}}
	c := NewFirstLegalClient()
	for range maxActsPerTurn {
		if _, act := c.answerChooseAction(in).(mb.ActOutput); !act {
			t.Fatal("the client passed inside its budget")
		}
	}
	if _, pass := c.answerChooseAction(in).(mb.PassOutput); !pass {
		t.Fatal("the client acted past its budget")
	}
	c.SetTurn(2)
	if _, act := c.answerChooseAction(in).(mb.ActOutput); !act {
		t.Fatal("a new turn did not renew the budget")
	}
}

// A window that keeps offering a filter ability (Bog Initiate's "{1}: Add {B}")
// is still left: the client confirms from the pool once its steps are spent.
func TestMockLeavesAPaymentWindowThatKeepsOfferingASource(t *testing.T) {
	in := mb.PayManaCostInput{CardID: "o1", CanConfirmFromPool: true,
		Actions: []mb.PaymentAction{{ID: "opt-0", Type: "activateAbility"}}}
	c := NewSeededRandomClient(1)
	for range maxPaymentSteps {
		if _, step := c.answerPayManaCost(in).(mb.ActOutput); !step {
			t.Fatal("the client left the window inside its budget")
		}
	}
	if got, ok := c.answerPayManaCost(in).(mb.PayOutput); !ok || got.Auto {
		t.Fatalf("answer past the budget = %#v, want a confirm from the pool", got)
	}
	in.CardID = "o2"
	if _, step := c.answerPayManaCost(in).(mb.ActOutput); !step {
		t.Fatal("another card's payment did not start with a fresh budget")
	}
}

// Crucible of the Spirit Dragon offers each colour once per unit of X, so a client
// told it can't repeat a colour can't name six of five colours.
func TestMockFillsAColourAskWithRepeats(t *testing.T) {
	in := mb.ChooseColorInput{ValidColors: []string{"B", "G", "R", "U", "W"}, Amount: 6, RepeatAllowed: true}
	got, ok := NewSeededRandomClient(1).answerChooseColor(in).(mb.ColorDecision)
	if !ok {
		t.Fatalf("answer type %T", got)
	}
	total := 0
	for _, n := range got.ChosenColors {
		total += n
	}
	if total != 6 {
		t.Fatalf("chosen %v, want 6 in all", got.ChosenColors)
	}
}
