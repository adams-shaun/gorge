package rules

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
)

// TestLimDulsVaultBotDeclinesUnlessAI is the hunt seed 279978 livelock, reduced
// to its rule: Lim-Dûl's Vault's Repeat re-poses "Pay 1 life" after every paid
// iteration, and the card's CheckLifePaid body carries Forge's UnlessAI$
// Never. A bot that pays the first-option election every time loops until the
// engine's livelock watcher aborts the game; the production policy must
// decline, ending the voluntary Repeat after the first look.
func TestLimDulsVaultBotDeclinesUnlessAI(t *testing.T) {
	t.Parallel()
	e, cfg, id, caster := corpusCardConfig(t, 7311, "Lim-Dûl's Vault")
	addMana(t, e, caster, "UB") // {U}{B}
	d := castFixture(t, e, id, -1)
	r := rand.New(rand.NewPCG(1, 2))
	asks := 0
	for steps := 0; steps < 80 && d != nil && len(e.G.Stack) > 0; steps++ {
		decline := -1
		if d.ResumeKind == "unless_pay" {
			asks++
			if asks > 1 {
				t.Fatalf("the bot paid the voluntary repeat: unless-pay posed %d times, want one decline", asks)
			}
			// Precondition: this is the real card's hinted election, with a
			// decline marker offered. A generic first-option pay answer would
			// repeat forever.
			if d.ResumeSA == nil || d.ResumeSA.Params["UnlessAI"] != "Never" {
				t.Fatalf("unless-pay ResumeSA = %+v, want UnlessAI$ Never", d.ResumeSA)
			}
			for _, o := range d.Options {
				if o.Mode == decision.ModeUnlessDecline {
					decline = o.Index
				}
			}
			if len(d.Options) < 2 || d.Options[0].Mode != decision.ModeUnlessPay || decline < 0 {
				t.Fatalf("election options are not pay-then-decline: %+v", d.Options)
			}
		}
		in := botpolicy.Decide(botpolicy.BoardFromGame(e.G, e, d.Player), d, r)
		if d.ResumeKind == "unless_pay" {
			if len(in.Choices) != 1 || in.Choices[0] != decline {
				t.Fatalf("bot answered %v for UnlessAI$ Never, want the decline marker %d", in.Choices, decline)
			}
		}
		submitChoices(t, e, in.Choices...)
		d = e.Pending()
	}
	if len(e.G.Stack) > 0 {
		t.Fatalf("Lim-Dûl's Vault never finished resolving (pending %+v)", d)
	}
	if asks != 1 {
		t.Fatalf("unless-pay was posed %d times, want exactly one declined election", asks)
	}
	replayCheck(t, e, cfg)
}
