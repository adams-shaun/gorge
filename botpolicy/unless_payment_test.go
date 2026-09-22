package botpolicy

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

func TestUnlessPaymentPolicyAnswersLegalManaWindow(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	activate := &decision.Decision{Seq: 7, Player: 0, Kind: decision.KChoose, Min: 1, Max: 1, ResumeKind: "unless_mana",
		Options: []decision.Option{{Index: 0, Kind: "activate"}, {Index: 1, Kind: "done"}}}
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
