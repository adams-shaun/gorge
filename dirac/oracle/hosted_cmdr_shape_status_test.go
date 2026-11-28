package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestHostedCmdrPlayerStatics pins the five hosted-Commander cards of the
// LifeLostAll ticket through the set a deck is built from: Ob Nixilis, Captive
// Kingpin lowers (its LifeLostAll group trigger; TestHostedCmdrLifeLostAll
// plays it), and the four whose shape needs a kernel model that does not exist
// yet stay not lowered, each with its own reason, so a deck holding one is
// turned away rather than played wrong:
//
//   - K'rrik, Son of Yawgmoth: the PayLifeInsteadOf:B player static (each {B}
//     of a cost payable with 2 life: the offer faces, the shortfall payment
//     and the mana window's done are not modelled);
//   - Opposition Agent: ControlOpponentsSearchingLibrary$ (a player
//     controlling another's search) and the found-card exile replacement;
//   - Expert-Level Safe: the secret ChooseNumber election (two players answer
//     hidden, then MatchedAbility$ / UnmatchedAbility$ branch);
//   - Nahiri's Lithoforming: an Effect's SetChosenNumber$ over an
//     AdjustLandPlays$ static beside a lands-enter-tapped replacement (the
//     kernel freezes the number only for an emblem ReduceCost, and has no
//     emblem enters-tapped replacement).
func TestHostedCmdrPlayerStatics(t *testing.T) {
	want := map[string]string{
		"Ob Nixilis, Captive Kingpin": "",
		"K'rrik, Son of Yawgmoth":     "player static",
		"Opposition Agent":            "ControlOpponentsSearchingLibrary",
		"Expert-Level Safe":           "the secret election is not modelled",
		"Nahiri's Lithoforming":       "Effect SetChosenNumber$ outside an emblem ReduceCost",
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	set := printedDelveSetIn(t, corpusDirOrSkip(t), [2][]string{names})
	for n, w := range want {
		i, ok := set.Lookup(n)
		if !ok {
			t.Fatalf("%s is not in the set", n)
		}
		st, why := set.Status(i), set.Reason(i)
		if w == "" {
			if st == cardset.StatusNotLowered || st == cardset.StatusRefused {
				t.Errorf("%s: %s (%s), want it lowered", n, st, why)
			}
			continue
		}
		if st != cardset.StatusNotLowered || !strings.Contains(why, w) {
			t.Errorf("%s: %s (%q), want not lowered with a reason containing %q", n, st, why, w)
		}
	}
}
