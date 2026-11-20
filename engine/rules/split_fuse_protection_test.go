package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// armedFuseAgainst deals seat 0 Armed // Dangerous in hand with the six mana a
// fused cast costs, and puts the named creatures on seat 1's battlefield.
func armedFuseAgainst(t *testing.T, targets ...string) (*Engine, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	island := searchCorpusCard(t, reg, "Island")
	deck := []*cards.Card{searchCorpusCard(t, reg, "Armed")}
	for len(deck) < 40 {
		deck = append(deck, island)
	}
	var opp []*cards.Card
	for _, n := range targets {
		opp = append(opp, searchCorpusCard(t, reg, n))
	}
	for len(opp) < 40 {
		opp = append(opp, island)
	}
	cfg := seatZeroStart(Config{Seed: 8426, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	for _, n := range targets {
		splitMoveFromLibrary(t, e, 1, n)
	}
	id := searchMoveByName(t, e, "Armed", state.ZHand)
	addMana(t, e, 0, "RRGGGG")
	return e, id
}

func castModes(e *Engine, id state.ObjID) map[string]bool {
	modes := map[string]bool{}
	for _, o := range castOptionsFor(e, id) {
		modes[o.Mode] = true
	}
	return modes
}

// CR 709.4d / 702.102b: a fused spell is red AND green, so a creature with
// protection from red and another with protection from green leave it no legal
// target, though each half alone (red Armed, green Dangerous) still has one.
// The fused offer must probe both halves as one combined spell, not the two
// halves one after the other, or it is offered and then aborts at announcement.
func TestFusedCastTargetsReadBothHalvesColours(t *testing.T) {
	t.Parallel()
	e, id := armedFuseAgainst(t, "Kor Firewalker", "Blightbeetle")
	modes := castModes(e, id)
	if !modes[""] && !modes["front"] && len(modes) == 0 {
		t.Fatalf("precondition: no cast of Armed at all was offered (Blightbeetle is a legal Armed target): %v", modes)
	}
	if !modes["split_alt"] {
		t.Errorf("Dangerous (green) is castable on Kor Firewalker (protection from red) but split_alt was not offered: %v", modes)
	}
	if modes["fuse"] {
		t.Errorf("fused Armed // Dangerous (red and green) has no legal target (pro-red and pro-green creatures only) but fuse was offered: %v", modes)
	}
}

// A creature with no protection is a target for the fused spell too: the
// fused offer is still made, so the combined read does not withhold it.
func TestFusedCastOfferedWithAnUnprotectedTarget(t *testing.T) {
	t.Parallel()
	e, id := armedFuseAgainst(t, "Kor Firewalker", "Blightbeetle", "Grizzly Bears")
	if modes := castModes(e, id); !modes["fuse"] {
		t.Errorf("fused cast not offered although Grizzly Bears is a legal target for both halves: %v", modes)
	}
}
