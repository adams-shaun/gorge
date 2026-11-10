package rules

// Hexproof:CardColors (Tam, Mindful First-Year): "each other creature you
// control has hexproof from each of its colors" (CR 702.11d). The static's
// grant expands per recipient to one Hexproof:<Colour> keyword for each colour
// the recipient has when the layer-6 grant applies.

import (
	"testing"
)

const tamSrc = "Name:Test Tam\nManaCost:1 GU\nTypes:Legendary Creature Gorgon Wizard\nPT:2/2\n" +
	"S:Mode$ Continuous | Affected$ Creature.YouCtrl+Other | AddKeyword$ Hexproof:CardColors | Description$ Your other creatures are hexproof from their own colours.\n" +
	"Oracle:x\n"

func TestHexproofFromOwnColours(t *testing.T) {
	t.Parallel()
	red := "Name:Red Bear\nManaCost:1 R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	redBlue := "Name:Izzet Bear\nManaCost:R U\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	colourless := "Name:Golem\nManaCost:3\nTypes:Artifact Creature Golem\nPT:2/2\nOracle:x\n"
	srcRed := "Name:Red Source\nManaCost:R\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"
	srcBlue := "Name:Blue Source\nManaCost:U\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"
	srcGreen := "Name:Green Source\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"

	cases := []struct {
		name      string
		target    string
		source    string
		wantBlock bool
	}{
		{"mono-red vs red source", red, srcRed, true},
		{"mono-red vs blue source", red, srcBlue, false},
		{"two-colour vs red source", redBlue, srcRed, true},
		{"two-colour vs blue source", redBlue, srcBlue, true},
		{"two-colour vs green source", redBlue, srcGreen, false},
		{"colourless vs red source", colourless, srcRed, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := handEngine(t)
			onBoard(t, e, 0, tamSrc)
			tgt := onBoard(t, e, 0, tc.target)
			src := onBoard(t, e, 1, tc.source)
			if got := e.HasKeyword(tgt, "Hexproof"); got != (tc.target != colourless) {
				t.Fatalf("precondition: Hexproof keyword present = %v, keywords %v", got, e.Keywords(tgt))
			}
			if got := e.hexproofBlocksTarget(tgt, 1, src); got != tc.wantBlock {
				t.Fatalf("opponent targeting blocked = %v, want %v (keywords %v)", got, tc.wantBlock, e.Keywords(tgt))
			}
			if e.hexproofBlocksTarget(tgt, 0, src) {
				t.Fatal("hexproof withheld the creature from its own controller")
			}
		})
	}

	t.Run("Tam itself gains nothing", func(t *testing.T) {
		e := handEngine(t)
		tam := onBoard(t, e, 0, tamSrc)
		src := onBoard(t, e, 1, srcBlue)
		if e.hexproofBlocksTarget(tam, 1, src) {
			t.Fatal("Tam is hexproof though the static says other creatures")
		}
	})
	t.Run("opponent's creature gains nothing", func(t *testing.T) {
		e := handEngine(t)
		onBoard(t, e, 0, tamSrc)
		theirs := onBoard(t, e, 1, red)
		src := onBoard(t, e, 0, srcRed)
		if e.hexproofBlocksTarget(theirs, 0, src) {
			t.Fatal("Tam protected an opponent's creature")
		}
	})

	// Hexproof follows the creature's colours as they change (CR 613.1e before
	// 613.1f): a recolour with a LATER timestamp than Tam's grant still counts.
	t.Run("colours changed by a later layer-5 effect", func(t *testing.T) {
		e := handEngine(t)
		onBoard(t, e, 0, tamSrc)
		tgt := onBoard(t, e, 0, colourless)
		src := onBoard(t, e, 1, srcBlue)
		if e.hexproofBlocksTarget(tgt, 1, src) {
			t.Fatal("precondition: colourless creature already hexproof from blue")
		}
		onBoard(t, e, 0, "Name:Test Dye\nManaCost:1 U\nTypes:Enchantment\n"+
			"S:Mode$ Continuous | Affected$ Creature.YouCtrl | SetColor$ Blue | Description$ Your creatures are blue.\nOracle:x\n")
		if !e.hexproofBlocksTarget(tgt, 1, src) {
			t.Fatalf("recoloured creature not hexproof from blue (keywords %v)", e.Keywords(tgt))
		}
	})
}
