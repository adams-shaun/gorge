package rules

import (
	"testing"
)

const madnessConvokePal = "Name:Black Pal\nManaCost:B\nTypes:Creature Zombie\nPT:1/1\nOracle:x\n"

// TestMadnessOfferCreditsConvoke: CR 702.51a with CR 702.35b -- Convoke
// applies to the spell whichever cost it is cast for, so a madness cast whose
// {2}{B} the caster's untapped creatures can tap-pay is offered with no mana
// source at all, and withheld when the creatures fall short.
func TestMadnessOfferCreditsConvoke(t *testing.T) {
	t.Parallel()
	const name = "Markov Baron"
	t.Run("three creatures pay the whole madness cost", func(t *testing.T) {
		_, _, d, _ := madnessAskAfterDiscard(t, 940, name,
			[]string{madnessConvokePal, madnessConvokePal, madnessConvokePal},
			[]string{"Black Pal", "Black Pal", "Black Pal"})
		if len(d.Options) != 2 || d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
			t.Fatalf("madness ask options = %+v, want yes and no", d.Options)
		}
	})
	// The control: two creatures credit {B}{1}, leaving {1} with no source to
	// pay it, so the offer must stay withheld.
	t.Run("two creatures fall one short", func(t *testing.T) {
		_, _, d, _ := madnessAskAfterDiscard(t, 941, name,
			[]string{madnessConvokePal, madnessConvokePal},
			[]string{"Black Pal", "Black Pal"})
		if len(d.Options) != 1 || d.Options[0].Kind != "no" {
			t.Fatalf("madness ask options = %+v, want only no", d.Options)
		}
	})
}
