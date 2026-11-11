package kgen

import (
	"slices"
	"strings"
	"testing"
)

// TestPreventDamageRefusedWithReason: a prevent-the-next-N-damage spell stays
// refused, now saying why, and its refusal class is still api:PreventDamage
// (the census and the class tier table are unmoved).
func TestPreventDamageRefusedWithReason(t *testing.T) {
	src := "Name:Probe Salve\nManaCost:W\nTypes:Instant\n" +
		"A:SP$ PreventDamage | ValidTgts$ Any | Amount$ 3 | SpellDescription$ Probe.\n" +
		"Oracle:Probe.\n"
	c := lowerScriptCard(t, "Probe Salve", src)
	if len(c.Refusals) == 0 {
		t.Fatal("PreventDamage lowered: the kernel has no depleting shield")
	}
	got := c.Refusals[0].Reason
	if !strings.Contains(got, "api PreventDamage: no depleting prevent-the-next-N-damage shield") {
		t.Errorf("reason %q does not name the missing shield", got)
	}
	if cl := refusalClasses(got); !slices.Equal(cl, []string{"api:PreventDamage"}) {
		t.Errorf("classes %v, want [api:PreventDamage]", cl)
	}
}
