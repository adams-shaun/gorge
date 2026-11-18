package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// An either-or additional cost whose mana-free part taps an untapped artifact
// (Disruption Protocol's shape: tap an untapped artifact you control, or pay a
// generic amount) is paid beside the spell's mana (CR 601.2b, 601.2g, 601.2h):
// when the only artifact is also one of the two sources of {U}{U}, paying the
// spell's mana taps it, so nothing is left to tap for the cost part and the cast
// cannot be completed. The fixture's alternative is {2}, not the card's {1}:
// the geometry below never pays that part, and a verbatim script line may not
// be committed.
func TestAltAddTapPartIsNotAlsoTheManaSource(t *testing.T) {
	t.Parallel()
	const (
		spellSrc = "Name:Tap Share Spell\nManaCost:U U\nTypes:Instant\nK:AlternateAdditionalCost:tapXType<1/Artifact>:2\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
		rockSrc  = "Name:Tap Share Rock\nManaCost:3\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ U\nOracle:x\n"
		landSrc  = "Name:Tap Share Island\nManaCost:no cost\nTypes:Basic Land Island\nOracle:x\n"
	)
	plans := func(spareIsland bool) bool {
		e, _, spell := newFixtureDeck(t, 9411, spellSrc)
		rock := onBoard(t, e, 0, rockSrc)
		onBoard(t, e, 0, landSrc)
		if spareIsland {
			onBoard(t, e, 0, landSrc)
		}
		if o := e.G.Obj(rock); o == nil || o.Tapped {
			t.Fatal("precondition: the artifact must be untapped on the battlefield")
		}
		e.G.Step = state.StepMain1
		ppPriority(t, e)
		got := e.potentialModeCastPlan(0, decision.Option{Kind: "cast", Obj: spell})
		return got.Plan != nil && got.Reason == ""
	}
	if !plans(true) {
		t.Fatal("precondition: the cast was not planned with a spare Island beside the artifact")
	}
	if plans(false) {
		t.Fatal("the cast was planned with the artifact both a source of {U}{U} and the one thing to tap")
	}
}
