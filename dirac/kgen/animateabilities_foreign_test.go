package kgen

import (
	"strings"
	"testing"
)

// animateAbilityCarriers print an Animate whose Abilities$ names the granting
// card's activated ability while the record lands on another permanent: a
// target (Banishing Knack, Retraction Helix), a parent target (Defiling
// Tears, Evolution Vat, Greater Stone Spirit, Run Wild, Touch of Vitae) or
// the enchanted permanent (Genju of the Fens). The record keeps the making
// card's object (kernel ovRec.mk), which the activation action carries.
var animateAbilityCarriers = []string{
	"Banishing Knack", "Retraction Helix", "Run Wild", "Evolution Vat",
	"Defiling Tears", "Genju of the Fens", "Greater Stone Spirit", "Touch of Vitae",
}

func TestAnimateAbilitiesCarriers(t *testing.T) {
	for _, name := range animateAbilityCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// The shapes the grant to another permanent still refuses: Kitesail
// Larcenist's grant rides beside a RemoveAllAbilities$ over an any-number
// slot; Ogre Marauder's Animate lowers its staticAbilities$ Unblockable but
// stays behind an unless-payment gate (the defending player may sacrifice a
// creature instead).
func TestAnimateAbilitiesRefusals(t *testing.T) {
	joined := func(rs []refusal) string {
		var b strings.Builder
		for i, r := range rs {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(r.Reason)
		}
		return b.String()
	}
	c := lowerNamedCard(t, "Kitesail Larcenist")
	if len(c.Refusals) == 0 || !strings.Contains(joined(c.Refusals), "beside RemoveAllAbilities$") {
		t.Errorf("Kitesail Larcenist refusals = %v, want the RemoveAllAbilities$ one", c.Refusals)
	}
	c = lowerNamedCard(t, "Ogre Marauder")
	if strings.Contains(joined(c.Refusals), "staticAbilities") {
		t.Errorf("Ogre Marauder refusals = %v, want no staticAbilities shape refusal", c.Refusals)
	}
	if len(c.Refusals) == 0 || !strings.Contains(joined(c.Refusals), "DefendingPlayer") {
		t.Errorf("Ogre Marauder refusals = %v, want the unless-payment gate", c.Refusals)
	}
}
