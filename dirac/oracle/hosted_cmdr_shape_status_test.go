package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestHostedCmdrPlayerStatics pins the five hosted-Commander cards of the
// LifeLostAll ticket through the set a deck is built from: Ob Nixilis, Captive
// Kingpin lowers (its LifeLostAll group trigger; TestHostedCmdrLifeLostAll
// plays it), Opposition Agent lowers (its search control and found-card exile;
// TestHostedCmdrSearchControlLockstep plays it), Expert-Level Safe lowers (its
// secret ChooseNumber election; TestHostedCmdrSecretElectionLockstep plays it),
// Nahiri's Lithoforming lowers (its Effect's frozen-number land plays and
// lands-enter-tapped replacement; TestHostedCmdrLithoformingLockstep plays it),
// and the one whose shape needs a kernel model that does not exist yet stays
// not lowered with its own reason, so a deck holding it is turned away rather
// than played wrong:
//
//   - K'rrik, Son of Yawgmoth: the PayLifeInsteadOf:B player static (each {B}
//     of a cost payable with 2 life: the offer faces, the shortfall payment
//     and the mana window's done are not modelled).
func TestHostedCmdrPlayerStatics(t *testing.T) {
	want := map[string]string{
		"Ob Nixilis, Captive Kingpin": "",
		"K'rrik, Son of Yawgmoth":     "player static",
		"Opposition Agent":            "",
		"Expert-Level Safe":           "",
		"Nahiri's Lithoforming":       "",
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
