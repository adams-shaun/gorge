package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Grell Philosopher delivers its grant through an Effect's StaticAbilities$
// (SVar:STSteal ... ManaConversion$ Blue->AnyColor) rather than a printed
// static: the same conversion must reach the gained ability's payment.
func TestGainedManaConversionGrellEffectDelivered(t *testing.T) {
	t.Parallel()
	grell := tokenReplCorpusCard(t, "Grell Philosopher")
	rock := card(t, "Name:Red Rock\nManaCost:2\nTypes:Artifact\nA:AB$ Draw | Cost$ R | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n")
	e, cfg := tokenReplGameSeats(t, 9204, []*cards.Card{grell}, []*cards.Card{rock})
	rockID := moveSeededCard(t, e, 1, rock, state.ZBattlefield)
	grellID := moveSeededCard(t, e, 0, grell, state.ZBattlefield)
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want Grell's target decision", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	addMana(t, e, 0, "U")
	opt, ok := gainedOption(e, grellID)
	if !ok {
		t.Fatalf("Grell does not offer the gained {R} ability with only {U}: %+v", e.Pending().Options)
	}
	if opt.GainedSource != rockID {
		t.Fatalf("gained anchor = %d, want the artifact %d", opt.GainedSource, rockID)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand %d -> %d, want +1 from the gained draw", hand, got)
	}
	replayCheck(t, e, cfg)
}
