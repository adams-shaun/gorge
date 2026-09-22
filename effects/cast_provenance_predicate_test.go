package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The three cast-provenance tokens (castprov1/2/3) are stripped and evaluated
// at every rules match site by castProvenanceAdmits, which holds the event log
// and the pre-push offer window the filter tier cannot reach. This test pins
// the two properties that make them part of the filter GRAMMAR without making
// the filter tier evaluate them:
//
//   - the census recognises them (UnknownPredicates reports nothing), so a
//     layered static or target spec carrying wasCastByYou no longer reads as
//     "unknown predicate" to the card-validation pass;
//   - a direct filter call still fails CLOSED (matches nothing), because the
//     rules layer strips the token before the filter runs and an unstripped
//     call is not entitled to a provenance answer.
//
// Zinnia, Valley's Voice's `Affected$ Creature.wasCastByYou` offspring grant
// is the live carrier: rules' matchesWithTypes strips the token through
// castProvenanceAdmitsWindow, so the layer walk never reaches the filter body
// asserted here.

// TestCastProvenancePredicatesAreRecognised pins the census half: the three
// tokens and their '!' negations are recognised shapes, so a spec carrying one
// is no longer reported unknown.
func TestCastProvenancePredicatesAreRecognised(t *testing.T) {
	for _, spec := range []string{
		"Creature.wasCastByYou",
		"Card.!wasCastByYou",
		"Instant.wasCastFromYourHandByYou",
		"Card.wasCastFromYourHandByYou+Creature",
		"Sorcery.!wasCastFromYourHand",
		"Instant.wasCastByYou+wasCastFromYourHand",
	} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty (recognised cast-provenance token)", spec, un)
		}
	}
}

// TestCastProvenanceFilterBodyFailsClosed pins the matcher half: with the
// token NOT stripped (a direct filter call), the predicate matches nothing --
// the fail-closed direction the rules-side split relies on when a spec reaches
// the filter with no log to consult.
func TestCastProvenanceFilterBodyFailsClosed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := corpusObject(t, reg, g, "Grizzly Bears")

	if MatchesObjectCtx(g, "Creature.wasCastByYou", bear, SpecContext{You: 0}) {
		t.Error("Card.wasCastByYou must match nothing in the filter tier (rules strips it first)")
	}
	// The negation is likewise fail-closed in the tier: it never reaches here
	// through rules, which evaluates the polarity itself.
	if MatchesObjectCtx(g, "Creature.!wasCastByYou", bear, SpecContext{You: 0}) {
		t.Error("Card.!wasCastByYou must match nothing in the filter tier (rules strips it first)")
	}
}
