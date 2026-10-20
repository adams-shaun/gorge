package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestColourFoldReadsOpponentPublicCards pins the A2 colour fold to the
// TARGET CONTROLLER's public cards, never the deciding seat's deck. The
// opponent (seat 1) is red by public permanents; the deciding seat (0) is
// white; a face-down red card contributes nothing.
func TestColourFoldReadsOpponentPublicCards(t *testing.T) {
	cs := map[state.ObjID]Card{
		10: {PrintedName: "Mountain", Land: true, Colour: cards.ColourMaskRed, CMC: 1},
		11: {PrintedName: "Island", Land: true, Colour: cards.ColourMaskBlue, CMC: 1},
		12: {PrintedName: "Red Creature", Creature: true, Power: 2, Toughness: 2, Colour: cards.ColourMaskRed},
		13: {PrintedName: "Blue Creature", Creature: true, Power: 2, Toughness: 2, Colour: cards.ColourMaskBlue},
	}
	public := map[state.ObjID]PublicCard{
		10: {Controller: 1, Colour: cards.ColourMaskRed},
		11: {Controller: 1, Colour: cards.ColourMaskBlue},
		12: {Controller: 1, Colour: cards.ColourMaskRed},
		13: {Controller: 1, Colour: cards.ColourMaskBlue},
		// A face-down red card: the printed identity is hidden (CR 708.5).
		14: {Controller: 1, Colour: cards.ColourMaskRed, FaceDown: true},
		// The deciding seat's own white public cards must not leak into the
		// opponent's colour posture.
		15: {Controller: 0, Colour: cards.ColourMaskWhite},
		16: {Controller: 1, Colour: cards.ColourMaskRed},
	}
	b := ruleBoard(cs, []state.ObjID{12, 13}, public)

	mix := b.publicColourMix(1)
	if mix[3] != 3 || mix[1] != 2 || mix[0] != 0 {
		t.Fatalf("opponent public mix = %v, want red 3, blue 2, white 0 (face-down and seat-0 cards ignored)", mix)
	}
	if got := dominantColour(mix); got != 3 {
		t.Fatalf("dominant colour = %d, want 3 (red)", got)
	}

	land := removalDecision([]tgt{{kind: "permanent", obj: 11, pl: 1}, {kind: "permanent", obj: 10, pl: 1}})
	if got := Decide(b, land, rng(1)).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("default land choices = %v, want [0]", got)
	}
	b.Targets = TargetRules{LandPriority: true}
	if got := Decide(b, land, rng(1)).Choices; len(got) != 1 || got[0] != 1 {
		t.Fatalf("armed land choices = %v, want [1] (the opponent's red land)", got)
	}

	creature := removalDecision([]tgt{{kind: "permanent", obj: 13, pl: 1}, {kind: "permanent", obj: 12, pl: 1}})
	b.Targets = TargetRules{ProminentColour: true}
	if got := Decide(b, creature, rng(1)).Choices; len(got) != 1 || got[0] != 1 {
		t.Fatalf("armed creature choices = %v, want [1] (the opponent's red creature)", got)
	}
}
