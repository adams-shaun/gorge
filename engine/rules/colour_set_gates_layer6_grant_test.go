package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 613.1e before 613.1f: layer 5 (colour) settles before layer 6
// (abilities), so Aphotic Wisps' chained Animate "target creature becomes
// black" gates an earlier layer-6 static's Affected$ match: Bloodmark
// Mentor's "Red creatures you control have first strike" stops covering the
// Mentor itself once the Wisps make it black, while another red creature
// keeps the grant. The layer-6 match once read the printed colour, so the
// Mentor kept its own first strike after the set.
func TestColourSetGatesAnEarlierLayer6KeywordGrant(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	mentor := lookup(t, reg, "Bloodmark Mentor")
	channeler := lookup(t, reg, "Chaos Channeler")
	wisps := lookup(t, reg, "Aphotic Wisps")
	e := corpusEngine(t, reg, []*cards.Card{mentor, channeler, wisps}, []*cards.Card{})
	mentorID := moveByName(t, e, 0, "Bloodmark Mentor", state.ZBattlefield)
	channelerID := moveByName(t, e, 0, "Chaos Channeler", state.ZBattlefield)
	wispsID := moveByName(t, e, 0, "Aphotic Wisps", state.ZHand)

	// Precondition: the Mentor's own static covers both red creatures and
	// the Mentor's printed colour is red, so the black set below is what
	// the assertions turn on.
	if d := e.Derived(mentorID); d.Colors != "R" {
		t.Fatalf("precondition: Mentor colours = %q, want R", d.Colors)
	}
	if !e.HasKeyword(mentorID, "First Strike") {
		t.Fatal("precondition: the Mentor's own static did not grant it first strike")
	}
	if !e.HasKeyword(channelerID, "First Strike") {
		t.Fatal("precondition: the Mentor's own static did not grant the Channeler first strike")
	}

	addMana(t, e, 0, "B")
	castSpellAtTarget(t, e, wispsID, mentorID)

	if d := e.Derived(mentorID); d.Colors != "B" {
		t.Fatalf("Mentor colours after the Wisps = %q, want B", d.Colors)
	}
	if e.HasKeyword(mentorID, "First Strike") {
		t.Fatalf("the black Mentor kept its own first-strike grant: %v", e.Derived(mentorID).Keywords)
	}
	if !e.HasKeyword(channelerID, "First Strike") {
		t.Fatal("the still-red Channeler lost the Mentor's first-strike grant")
	}
}
