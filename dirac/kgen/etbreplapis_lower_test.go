package kgen

import (
	"strings"
	"testing"
)

// etbReplAPIsCards are the as-enters replacement bodies this slice lowers:
// Animate, ChoosePlayer, Reveal and StoreSVar chains. Each is otherwise
// unblocked, so a refusal here is one of this slice's classes.
var etbReplAPIsCards = []string{
	"Arachne, Psionic Weaver", "Arsenal Thresher", "Canker Abomination",
	"Captive Audience", "Dragon's Disciple", "Fortified Beachhead",
	"Master Biomancer", "Pouncing Wurm", "Voidpouncer", "Benalish Lancer",
	"Duskwalker", "Kavu Titan", "Minion of the Wastes", "Nameless Race",
	"Phyrexian Processor", "Pouncing Kavu",
}

func TestETBReplApisSlice(t *testing.T) {
	for _, name := range etbReplAPIsCards {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// These three carry, beside the classes this slice lowers, a distinct shape:
// a second as-enters choice with the spell still on the stack (Jihad), a
// second as-enters replacement whose CR 616.1 order decides the tap (Temple of
// the Dragon Queen), and a Play whose ValidSA$ Spell.Artifact alternative the
// head reader cannot name as a card filter (Skyway Robber, lower_zone.go's
// spell-ability filter guard). Each stays refused by its own class.
func TestETBReplApisOtherBlockers(t *testing.T) {
	for _, tc := range []struct{ card, reason string }{
		{"Jihad", "ChoosePlayer after ChooseColor"},
		{"Temple of the Dragon Queen", "Reveal beside an as-enters ChooseColor"},
		{"Skyway Robber", "spell-ability filter ValidSA"},
	} {
		c := lowerNamedCard(t, tc.card)
		if len(c.Refusals) == 0 {
			t.Errorf("%s lowers; want the %s blocker", tc.card, tc.reason)
			continue
		}
		for _, r := range c.Refusals {
			if !strings.Contains(r.Reason, tc.reason) {
				t.Errorf("%s refusal %q is not the %s class", tc.card, r.Reason, tc.reason)
			}
		}
	}
}
