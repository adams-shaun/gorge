package kgen

import (
	"slices"
	"strings"
	"testing"
)

// TestPreventDamageRefusedWithReason: a prevent-the-next-N-damage ability the
// kernel's one-recipient shield cannot model stays refused, saying why, and its
// refusal class is still api:PreventDamage (the census and the class tier table
// are unmoved).
func TestPreventDamageRefusedWithReason(t *testing.T) {
	src := "Name:Probe Salve\nManaCost:W\nTypes:Instant\n" +
		"A:SP$ PreventDamage | ValidTgts$ Any | Amount$ 3 | TargetMin$ 0 | TargetMax$ 3 | DividedAsYouChoose$ 3 | SpellDescription$ Probe.\n" +
		"Oracle:Probe.\n"
	c := lowerScriptCard(t, "Probe Salve", src)
	if len(c.Refusals) == 0 {
		t.Fatal("a divided PreventDamage lowered: the kernel's shield has one recipient")
	}
	got := c.Refusals[0].Reason
	if !strings.Contains(got, "api PreventDamage: DividedAsYouChoose: a total divided among any number of targets") {
		t.Errorf("reason %q does not name the divided total", got)
	}
	if cl := refusalClasses(got); !slices.Equal(cl, []string{"api:PreventDamage"}) {
		t.Errorf("classes %v, want [api:PreventDamage]", cl)
	}
}
