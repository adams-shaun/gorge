package effects

import "testing"

// TestSharesColorWithBareSource pins the bare `Card.SharesColorWith` filter
// predicate, Dream Halls' alternative discard's candidate test: the candidate
// shares a colour with the filter's own source (the spell being cast). Before
// the form was recognised the whole predicate was unknown, so discardAsk
// offered no candidate and the alternative cost was never payable. Colourless
// objects share nothing, and an absent source is a resolved non-match.
func TestSharesColorWithBareSource(t *testing.T) {
	g, id := board(t)
	red, red2 := g.Obj(id["myBattle"]), g.Obj(id["mySorcery"])
	blue, colourless := g.Obj(id["myInstant"]), g.Obj(id["myArtifact"])
	if red == nil || red2 == nil || blue == nil || colourless == nil {
		t.Fatal("precondition: the board's fixture objects are missing")
	}
	if !MatchesSpecFrom(g, "Card.SharesColorWith", red2.ID, 0, red.ID) {
		t.Error("two red cards did not share a colour")
	}
	if MatchesSpecFrom(g, "Card.SharesColorWith", blue.ID, 0, red.ID) {
		t.Error("a blue card shared a colour with a red source")
	}
	if MatchesSpecFrom(g, "Card.SharesColorWith", colourless.ID, 0, colourless.ID) {
		t.Error("a colourless card shared a colour with a colourless source")
	}
	if MatchesSpecFrom(g, "Card.SharesColorWith", red.ID, 0, 0) {
		t.Error("an absent source matched")
	}
	if un := UnknownPredicates("Card.SharesColorWith"); len(un) != 0 {
		t.Errorf("UnknownPredicates(Card.SharesColorWith) = %v, want empty", un)
	}
}
