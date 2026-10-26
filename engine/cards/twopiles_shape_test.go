package cards

import (
	"slices"
	"testing"
)

// A TwoPiles shape the resolver Notes and skips must not read as supported:
// the card carries api:TwoPiles.OtherShape. Each body is the printed shape of
// a corpus carrier.
func TestTwoPilesCoverageRejectsOtherShapes(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		supported  bool
	}{
		{"Liliana of the Veil -6", "AB$ TwoPiles | Cost$ SubCounter<6/LOYALTY> | ValidTgts$ Player | Separator$ You | ChosenPile$ DBPile | ValidCards$ Permanent | Zone$ Battlefield", true},
		{"Do or Die", "SP$ TwoPiles | ValidTgts$ Player | Separator$ You | ChosenPile$ DBPile | ValidCards$ Creature | Zone$ Battlefield", true},
		{"Fight or Flight", "SP$ TwoPiles | Defined$ TriggeredPlayer | Chooser$ TriggeredPlayer | ValidCards$ Creature | Zone$ Battlefield | Separator$ You | ChosenPile$ DBPile", true},
		{"Fact or Fiction", "SP$ TwoPiles | Defined$ You | DefinedCards$ Remembered | Separator$ Opponent | ChosenPile$ DBPile | UnchosenPile$ DBPile", true},
		{"graveyard card set (Death or Glory)", "SP$ TwoPiles | Defined$ You | Separator$ You | Chooser$ Opponent | ChosenPile$ DBPile | ValidCards$ Creature | Zone$ Graveyard", false},
		{"left/right piles (Raging River)", "SP$ TwoPiles | Defined$ Remembered | Separator$ Remembered | ValidCards$ Creature | Zone$ Battlefield | LeftRightPile$ True | ChosenPile$ DBPile", false},
		{"remembered pool (Bend or Break)", "SP$ TwoPiles | Defined$ Remembered | Separator$ Remembered | Chooser$ ChosenPlayer | ChosenPile$ DBPile | ValidCards$ Land.RememberedPlayerCtrl | Zone$ Battlefield", false},
		{"RememberChosen without a pile body (Make an Example)", "SP$ TwoPiles | Defined$ Remembered | Separator$ Remembered | Chooser$ You | RememberChosen$ True | ValidCards$ Creature | Zone$ Battlefield", false},
		{"unresolvable pile body", "SP$ TwoPiles | ValidTgts$ Player | Separator$ You | ChosenPile$ NoSuchSVar | ValidCards$ Creature | Zone$ Battlefield", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := ParseBytes("test.txt", []byte("Name:Test\nTypes:Sorcery\nA:"+tc.body+"\nSVar:DBPile:DB$ SacrificeAll | ValidCards$ Permanent.IsRemembered\nOracle:x\n"))
			if err != nil {
				t.Fatal(err)
			}
			c.Link()
			p := c.Primitives()
			if !slices.Contains(p, "api:TwoPiles") {
				t.Fatalf("precondition: no TwoPiles API: %v", p)
			}
			if got := slices.Contains(p, "api:TwoPiles.OtherShape"); got == tc.supported {
				t.Fatalf("unimplemented shape marker = %v, supported shape = %v: %v", got, tc.supported, p)
			}
		})
	}
}
