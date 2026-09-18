package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestSteamVentsPayLifeElectionNamesTheLife pins the wording of the shock
// land's pay/decline election on Steam Vents' REAL compiled replacement body
// (task fb-20260917T233137Z). The player report: the election's prompt read
// "Pay the cost, or decline" without naming the cost, and it presented as the
// mana bubble. This test pins the engine half -- unlessCostLabel must render
// UnlessCost$ PayLife<2> as "2 life", so the ask the seat receives is
//
//	prompt        "Pay 2 life, or decline"
//	option 0      "Pay 2 life"
//	option 1      "Don't pay"
//
// (the web half -- that the election no longer auto-opens the radial mana
// wheel -- is pinned in web/src/lib/cardoptions.test.ts). The fixture is the
// real corpus SA, not a synthetic map: Steam Vents carries the UnlessCost on
// a DB$ Tap replacement body (R:Event$ Moved | ReplaceWith$ DBTap |
// ReplacementResult$ Updated), reached here through the Repls walk.
func TestSteamVentsPayLifeElectionNamesTheLife(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup("Steam Vents")
	if !ok {
		t.Fatal("corpus has no \"Steam Vents\"")
	}
	var found *cards.SA
	for _, f := range c.Faces {
		for _, r := range f.Repls {
			for w := r.With; w != nil && found == nil; w = w.Sub {
				if strings.TrimSpace(w.Params["UnlessCost"]) == "PayLife<2>" {
					found = w
				}
			}
		}
	}
	if found == nil {
		t.Fatal("Steam Vents has no compiled replacement body with UnlessCost$ PayLife<2> -- the corpus changed under this test")
	}
	if found.API != "Tap" {
		t.Fatalf("found SA API = %q, want Tap", found.API)
	}

	h := &askHost{}
	h.g = state.NewGame(names(2))
	src := h.g.AddObject(mkCard(t, "Name:Steam Vents\nManaCost:no cost\nTypes:Land Island Mountain\nOracle:x\n"), 0).ID
	Resolve(h, &Ctx{Source: src, Controller: 0}, found)

	if h.asked == nil {
		t.Fatal("no pay election was posed for UnlessCost$ PayLife<2>")
	}
	if h.asked.Prompt != "Pay 2 life, or decline" {
		t.Fatalf("prompt = %q, want %q", h.asked.Prompt, "Pay 2 life, or decline")
	}
	if got := h.asked.Options[0].Label; got != "Pay 2 life" {
		t.Fatalf("pay label = %q, want %q", got, "Pay 2 life")
	}
	if got := h.asked.Options[1].Label; got != "Don't pay" {
		t.Fatalf("decline label = %q, want %q", got, "Don't pay")
	}
}

// TestUnlessCostLabelDegradations pins the boundary of the PayLife rendering:
// a fixed PayLife<N> names the life; the announced PayLife<X> and every other
// non-mana token still degrade to "the cost" (they must never leak script
// syntax), and plain mana costs still come back verbatim.
func TestUnlessCostLabelDegradations(t *testing.T) {
	for _, tc := range []struct{ cost, want string }{
		{"PayLife<2>", "2 life"},
		{"PayLife<1>", "1 life"},
		{"PayLife<7>", "7 life"},
		{"1 PayLife<3>", "1 and 3 life"},
		{"PayLife<X>", "the cost"},
		{"PayLife<Y>", "the cost"},
		// signs: Atoi would accept them but rules' lifeCost (bare digits only)
		// hard-declines, so the label must not promise a payable cost
		{"PayLife<-2>", "the cost"},
		{"PayLife<+2>", "the cost"},
		{"PayLife<>", "the cost"},
		{"PayLife<2e3>", "the cost"},
		{"X", "the cost"},
		{"Discard<1/Card>", "the cost"},
		{"3", "3"},
		{"2 U", "2 U"},
		{"R R", "R R"},
	} {
		if got := unlessCostLabel(tc.cost); got != tc.want {
			t.Errorf("unlessCostLabel(%q) = %q, want %q", tc.cost, got, tc.want)
		}
	}
}
