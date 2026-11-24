package kgen

import (
	"slices"
	"strings"
	"testing"
)

// TestPreventDamageRefusedWithReason: a prevent-the-next-N-damage ability the
// kernel cannot model stays refused, saying why, and its refusal class is
// still api:PreventDamage (the census and the class tier table are unmoved).
// The random-recipient shape is the corpus's one unmodelled form once the
// divided, set and rider shapes lower.
func TestPreventDamageRefusedWithReason(t *testing.T) {
	src := "Name:Probe Salve\nManaCost:W\nTypes:Instant\n" +
		"A:SP$ PreventDamage | ValidTgts$ Any | Amount$ 3 | Random$ True | SpellDescription$ Probe.\n" +
		"Oracle:Probe.\n"
	c := lowerScriptCard(t, "Probe Salve", src)
	if len(c.Refusals) == 0 {
		t.Fatal("a random-recipient PreventDamage lowered: the engine forbids ambient randomness")
	}
	got := c.Refusals[0].Reason
	if !strings.Contains(got, "api PreventDamage: Random: a random recipient") {
		t.Errorf("reason %q does not name the random recipient", got)
	}
	if cl := refusalClasses(got); !slices.Equal(cl, []string{"api:PreventDamage"}) {
		t.Errorf("classes %v, want [api:PreventDamage]", cl)
	}
}
