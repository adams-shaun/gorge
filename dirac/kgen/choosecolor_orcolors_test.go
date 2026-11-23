package kgen

import (
	"strings"
	"testing"
)

// chooseColorResInstrs is every ChooseColorRes instruction of the lowered card.
func chooseColorResInstrs(lc *lCard) []lInstr {
	var out []lInstr
	for _, ab := range lc.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "ChooseColorRes" {
				out = append(out, in)
			}
		}
	}
	return out
}

// TestChooseColorOrColorsLowers pins the OrColors$ True spelling of a
// mid-resolution ChooseColor (Quickchange, Prismatic Lace, Shyft: "becomes
// the color or colors of your choice"). gorge effChooseColor cannot ask the
// shape (chooseColorOptions reports OrColors$ unaskable and the answer is the
// first-WUBRG fallback "W", with a Note), so the lowered instruction is the
// resolution's no-ask white answer (C = 1), never the five-colour ask; the
// plain spelling keeps its ask (C = 0).
func TestChooseColorOrColorsLowers(t *testing.T) {
	const orColors = "Name:OrColors Probe\nManaCost:1 U\nTypes:Instant\n" +
		"A:SP$ ChooseColor | Defined$ You | OrColors$ True | SubAbility$ Animate | SpellDescription$ x.\n" +
		"SVar:Animate:DB$ Animate | ValidTgts$ Creature | Colors$ ChosenColor | OverwriteColors$ True | SpellDescription$ y.\n"
	lc := lowerSynth(t, orColors)
	if r := lc.Refusals; len(r) != 0 {
		t.Fatalf("the OrColors probe refuses: %q", keclReason(r))
	}
	ins := chooseColorResInstrs(lc)
	if len(ins) != 1 || ins[0].C != 1 {
		t.Fatalf("OrColors$ lowers to %+v, want one ChooseColorRes with C = 1 (the no-ask white answer)", ins)
	}

	// The control: the same chain without OrColors$ asks (C = 0), so the
	// assertion above can fail.
	plain := strings.Replace(orColors, " | OrColors$ True", "", 1)
	ins = chooseColorResInstrs(lowerSynth(t, plain))
	if len(ins) != 1 || ins[0].C != 0 {
		t.Fatalf("the plain ChooseColor lowers to %+v, want one ChooseColorRes with C = 0", ins)
	}

	// An OrColors$ the lowering cannot place refuses loudly, not silently:
	// any sub-ability but the chosen-colour Animate, and any value but True.
	for name, src := range map[string]string{
		"value": strings.Replace(orColors, "OrColors$ True", "OrColors$ False", 1),
		"sub": strings.Replace(strings.Replace(orColors, "SubAbility$ Animate", "SubAbility$ Draw", 1),
			"SVar:Animate:DB$ Animate | ValidTgts$ Creature | Colors$ ChosenColor | OverwriteColors$ True",
			"SVar:Draw:DB$ Draw | NumCards$ 1", 1),
	} {
		if len(lowerSynth(t, src).Refusals) == 0 {
			t.Errorf("OrColors$ %s variant lowers, want a refusal", name)
		}
	}
}
