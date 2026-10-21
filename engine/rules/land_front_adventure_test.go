package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLandFrontAdventureCastable pins CR 715.3a for an adventurer card whose
// main face is a LAND: only the Adventure face's characteristics are
// evaluated to see whether it can be cast, so the hand walk must offer the
// Adventure spell even though a land front is never cast from hand.
func TestLandFrontAdventureCastable(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	for _, tc := range []struct {
		front string
		spell string
		mana  string
	}{
		{"Lindblum, Industrial Regency", "Mage Siege", "RRR"},
		{"Midgar, City of Mako", "Reactor Raid", "BBB"},
		{"Zanarkand, Ancient Metropolis", "Lasting Fayth", "GGGGGG"},
	} {
		tc := tc
		t.Run(tc.front, func(t *testing.T) {
			front := searchCorpusCard(t, reg, tc.front)
			mountain := searchCorpusCard(t, reg, "Mountain")
			bear := searchCorpusCard(t, reg, "Grizzly Bears")
			deck := []*cards.Card{front}
			for len(deck) < 40 {
				deck = append(deck, mountain)
			}
			opp := make([]*cards.Card, 40)
			for i := range opp {
				opp[i] = bear
			}
			cfg := seatZeroStart(Config{Seed: 7402, Names: []string{"adventure", "opponent"},
				Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
			e := New(cfg)
			e.Advance()
			toMain1(t, e)
			id := searchMoveByName(t, e, tc.front, state.ZHand)

			o := e.G.Obj(id)
			if o == nil || o.Face() == nil || !o.Face().IsLand() {
				t.Fatalf("precondition: %q front face is not a land: %+v", tc.front, o)
			}
			af := adventureSpellFace(o)
			if af == nil || af.Name != tc.spell {
				t.Fatalf("precondition: %q has no Adventure face %q: %+v", tc.front, tc.spell, af)
			}
			if af.IsLand() || (!af.IsInstant() && !af.IsSorcery()) {
				t.Fatalf("precondition: Adventure face %q is not a spell: %+v", tc.spell, af)
			}

			addMana(t, e, 0, tc.mana)
			alt := adventureOption(t, e, id, "adventure_alt")
			if alt == nil || alt.Label != "Cast "+tc.spell {
				t.Fatalf("land-front Adventure %q not offered from hand: %+v", tc.spell, castOptions(t, e))
			}
			submitChoices(t, e, alt.Index)
			passUntilStackEmpty(t, e, 20)
			got := e.G.Obj(id)
			if got.Zone != state.ZExile || int(got.FaceIdx) != 0 {
				t.Fatalf("after %q resolved: zone=%s faceIdx=%d, want ZExile/0",
					tc.spell, got.Zone, got.FaceIdx)
			}
			replayCheck(t, e, cfg)
		})
	}
}
