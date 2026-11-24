package kgen

import "testing"

// exiledWithSliceCarriers read the exiled-with link (CR 607.2a) in a hidden
// pick out of exile into a hand, a resolution-time mover: the pick's
// ExiledWithSource reads the resolving source (kernel exwFlag), the same
// predicate the battlefield-bound exile pick already lowers.
var exiledWithSliceCarriers = []string{"Roads Go Ever, Ever On"}

func TestExiledWithSlice(t *testing.T) {
	for _, name := range exiledWithSliceCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		if !c.ExiledWith {
			t.Errorf("%s lowers without reading the exiled-with link", name)
		}
	}
}

// A hidden pick out of exile into a hand is a resolution-time mover for any
// card, not only one that names the link: it is the game-wide exile walk.
func TestExiledWithHandPickLowers(t *testing.T) {
	refusals, ops := czLowered(t, "SP$ ChangeZone | Cost$ B | Hidden$ True | Origin$ Exile | Destination$ Hand | ChangeType$ Creature")
	if len(refusals) != 0 {
		t.Errorf("refusals = %v, want none", refusals)
	}
	if len(ops) != 1 || ops[0] != "ChooseMove:from=Exile" {
		t.Errorf("ops = %v, want ChooseMove:from=Exile", ops)
	}
}

// A Delve carrier's reader (Ethereal Forager) reads the cards the cast
// exiled, which gorge never links to the spell: it stays refused.
func TestExiledWithDelveReaderRefuses(t *testing.T) {
	if c := lowerNamedCard(t, "Ethereal Forager"); len(c.Refusals) == 0 {
		t.Error("Ethereal Forager lowers; want the Delve exiled-with refusal")
	}
}
