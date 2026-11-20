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
		{"graveyard card set (Death or Glory)", "SP$ TwoPiles | Defined$ You | Separator$ You | Chooser$ Opponent | ChosenPile$ DBPile | ValidCards$ Creature | Zone$ Graveyard", true},
		{"left/right piles (Raging River)", "SP$ TwoPiles | Defined$ Remembered | Separator$ Remembered | ValidCards$ Creature | Zone$ Battlefield | LeftRightPile$ True | ChosenPile$ DBPile", false},
		{"remembered pool (Bend or Break)", "SP$ TwoPiles | Defined$ Remembered | Separator$ Remembered | Chooser$ ChosenPlayer | ChosenPile$ DBPile | ValidCards$ Land.RememberedPlayerCtrl | Zone$ Battlefield", true},
		{"RememberChosen without a pile body (Make an Example)", "SP$ TwoPiles | Defined$ Remembered | Separator$ Remembered | Chooser$ You | RememberChosen$ True | ValidCards$ Creature.RememberedPlayerCtrl | Zone$ Battlefield", true},
		{"top of library (Sauron's Ransom)", "SP$ TwoPiles | Defined$ You | Separator$ Opponent | Chooser$ You | DefinedCards$ Top_4_OfLibrary | ChosenPile$ DBPile | UnchosenPile$ DBPile | FaceDown$ One", true},
		{"malformed top of library", "SP$ TwoPiles | Defined$ You | Separator$ Opponent | DefinedCards$ Top_X_OfLibrary | ChosenPile$ DBPile", false},
		{"face-down piles over remembered cards (Phyrexian Portal)", "SP$ TwoPiles | Defined$ You | DefinedCards$ Remembered | Separator$ Targeted | FaceDown$ True | ChosenPile$ DBPile | UnchosenPile$ DBPile", true},
		{"imprinted cards (Truth or Tale)", "SP$ TwoPiles | Chooser$ Opponent | DefinedCards$ Imprinted | Separator$ You | ChosenPile$ DBPile", true},
		{"no pile body and no RememberChosen", "SP$ TwoPiles | Defined$ Remembered | Separator$ Remembered | Chooser$ You | ValidCards$ Creature | Zone$ Battlefield", false},
		{"Hostile Negotiations you-choose", "SP$ TwoPiles | Zone$ Exile | DefinedPiles$ Remembered,Imprinted | ChosenPile$ DBPile | KeepRemembered$ True", true},
		{"Hostile Negotiations they-choose", "SP$ TwoPiles | Zone$ Exile | DefinedPiles$ Remembered,Imprinted | Chooser$ Opponent | ChosenPile$ DBPile | UnchosenPile$ DBPile", true},
		{"DefinedPiles one selector", "SP$ TwoPiles | DefinedPiles$ Remembered | ChosenPile$ DBPile", false},
		{"DefinedPiles unknown selector", "SP$ TwoPiles | DefinedPiles$ Remembered,Sacrificed | ChosenPile$ DBPile", false},
		{"DefinedPiles with LeftRightPile", "SP$ TwoPiles | DefinedPiles$ Remembered,Imprinted | LeftRightPile$ True | ChosenPile$ DBPile", false},
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
