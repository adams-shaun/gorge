package botpolicy

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// limDulsVaultUnless builds the election Lim-Dûl's Vault's CheckLifePaid
// StoreSVar poses: "Pay 1 life" (option 0) or "Don't pay" (option 1), with
// Forge's UnlessAI$ Never hint, which tells the AI never to pay. The second
// return is the same shape without the hint, so the control pins that the
// decline comes from the hint rather than from an unpayable election.
func limDulsVaultUnless() (never, control *decision.Decision) {
	mk := func(ai string) *decision.Decision {
		return &decision.Decision{
			Seq: 17, Player: 0, Kind: decision.KModes, Min: 1, Max: 1,
			ResumeKind: "unless_pay",
			ResumeSA: &cards.SA{Kind: "DB", API: "StoreSVar", Params: map[string]string{
				"UnlessCost":        "PayLife<1>",
				"UnlessPayer":       "You",
				"UnlessResolveSubs": "WhenPaid",
				"UnlessAI":          ai,
			}},
			Options: []decision.Option{
				{Index: 0, Kind: "mode", Label: "Pay 1 life", Mode: decision.ModeUnlessPay},
				{Index: 1, Kind: "mode", Label: "Don't pay", Mode: decision.ModeUnlessDecline},
			},
		}
	}
	return mk("Never"), mk("")
}

// TestUnlessNeverPayDeclines is the Lim-Dûl's Vault livelock regression
// (hunt seed 279978): the pay option is legal and first, so the ordinary
// KModes arm would pay; UnlessAI$ Never must take the decline marker
// instead, ending the card's voluntary Repeat after one iteration.
func TestUnlessNeverPayDeclines(t *testing.T) {
	t.Parallel()
	never, _ := limDulsVaultUnless()
	// Precondition: this really is the two-option election with pay first, so
	// a policy that ignores the hint would answer pay and loop.
	if never.ResumeKind != "unless_pay" || len(never.Options) != 2 ||
		never.Options[0].Mode != decision.ModeUnlessPay ||
		never.Options[1].Mode != decision.ModeUnlessDecline ||
		never.ResumeSA.Params["UnlessAI"] != "Never" {
		t.Fatalf("unless election precondition failed: %+v", never)
	}
	in := Decide(Board{}, never, rand.New(rand.NewPCG(1, 2)))
	if err := never.Validate(in); err != nil {
		t.Fatalf("decline answer is illegal: %v", err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != never.Options[1].Index {
		t.Fatalf("UnlessAI$ Never answered %v, want the decline marker at index %d", in.Choices, never.Options[1].Index)
	}
}

// TestUnlessWithoutNeverStillPays is the positive control: the same election
// without the hint keeps the ordinary first-option pay answer, so the arm
// above is a hint-driven exception, not a blanket decline of Lim-Dûl's Vault.
func TestUnlessWithoutNeverStillPays(t *testing.T) {
	t.Parallel()
	_, control := limDulsVaultUnless()
	in := Decide(Board{}, control, rand.New(rand.NewPCG(1, 2)))
	if err := control.Validate(in); err != nil {
		t.Fatalf("pay answer is illegal: %v", err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != control.Options[0].Index {
		t.Fatalf("election without UnlessAI$ Never answered %v, want the pay marker at index %d", in.Choices, control.Options[0].Index)
	}
}
