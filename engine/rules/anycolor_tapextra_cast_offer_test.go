package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// An any-colour TapsForMana extra (Dawn's Reflection) funds a cast: the
// enchanted Forest taps for {G} and two mana of any colour (CR 605.3b), so
// {2}{B}{B} is payable from a Swamp and that Forest alone.
func TestAnyColourTapExtraFundsCastOffer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name  string
		extra []string
	}{
		{"control four swamps", []string{"Swamp", "Swamp", "Swamp"}},
		{"swamp and enchanted forest", nil},
		{"with a forest and a gate", []string{"Forest", "Citadel Gate"}},
		{"with gwenna", []string{"Forest", "Gwenna, Eyes of Gaea"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := handEngine(t, mustCorpusCard(t, reg, "Diabolic Tutor"))
			tutor := e.G.Zone(state.ZHand, 0)[0]
			sw := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Swamp"))
			forest := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Forest"))
			aura := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Dawn's Reflection"))
			e.G.Obj(aura).AttachedTo = forest
			for _, n := range tc.extra {
				id := onBoardCard(t, e, 0, mustCorpusCard(t, reg, n))
				e.G.Obj(id).SummonSick = false
				if n == "Citadel Gate" {
					e.G.Obj(id).ChosenColor = "G"
				}
			}
			e.G.Obj(sw).SummonSick = false
			e.G.Obj(forest).SummonSick = false
			e.pending = nil
			e.Advance() // the priority decision the prefix search clones from
			found := false
			for _, pp := range e.PotentialPaymentPlans(0) {
				if pp.Action.Kind == "cast" && pp.Action.Obj == tutor {
					found = true
					if pp.Reason != "" {
						t.Errorf("Diabolic Tutor plan reason %q (%s), want payable", pp.Reason, pp.Detail)
					}
				}
			}
			if !found {
				t.Fatal("no potential plan for Diabolic Tutor")
			}
		})
	}
}
