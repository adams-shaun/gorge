package botpolicy

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// TestUnlessPaymentPolicyAnswersLegalManaWindow pins the unless_mana window
// arm: the bot activates an offered source while one remains, and settles
// with Done once the engine has closed the source list. Every answer is run
// through Decision.Validate so a re-submittable rejection (a livelock) cannot
// hide here.
func TestUnlessPaymentPolicyAnswersLegalManaWindow(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	activate := &decision.Decision{Seq: 7, Player: 0, Kind: decision.KChoose, Min: 1, Max: 1, ResumeKind: "unless_mana",
		Options: []decision.Option{{Index: 0, Kind: "activate"}, {Index: 1, Kind: "done"}}}
	// Precondition: the decision really is a two-option unless_mana window.
	if activate.ResumeKind != "unless_mana" || len(activate.Options) != 2 {
		t.Fatalf("window ask precondition failed: %+v", activate)
	}
	in := Decide(Board{}, activate, r)
	if err := activate.Validate(in); err != nil {
		t.Fatalf("mana activation answer is illegal: %v", err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("mana window should activate an offered source: %+v", in)
	}

	done := *activate
	done.Options = []decision.Option{{Index: 0, Kind: "done"}}
	in = Decide(Board{}, &done, r)
	if err := done.Validate(in); err != nil {
		t.Fatalf("done answer is illegal: %v", err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("mana window should settle with Done: %+v", in)
	}
}

// TestUnlessPaymentPolicyDeclinesWhenPayIsNotOffered pins the R-9-shaped
// single-option election (the pay branch was never offered because the window
// cannot reach the cost): the bot takes the only legal answer and it passes
// Validate.
func TestUnlessPaymentPolicyDeclinesWhenPayIsNotOffered(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	d := &decision.Decision{Seq: 8, Player: 0, Kind: decision.KModes, Min: 1, Max: 1, ResumeKind: "unless_pay",
		Options: []decision.Option{{Index: 0, Kind: "mode", Label: "Don't pay"}}}
	in := Decide(Board{}, d, r)
	if err := d.Validate(in); err != nil {
		t.Fatalf("decline answer is illegal: %v", err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("bot did not decline the unavailable payment: %+v", in)
	}
}
