package kgen

import (
	"strings"
	"testing"
)

// playCondTwoRefusedScript is a peek-then-cast Play that carries
// two Condition keys that both stay outside the gate group
// (ConditionDefined and ConditionTargetValidTargeting), so the refusal loop
// has two candidates and must name the sorted-first one.
const playCondTwoRefusedScript = "Name:Test Peeker\nManaCost:2 U\nTypes:Sorcery\n" +
	"A:SP$ PeekAndReveal | NoReveal$ True | SubAbility$ DBCastTop | SpellDescription$ x\n" +
	"SVar:DBCastTop:DB$ Play | Optional$ True | Defined$ TopOfLibrary | ValidSA$ Spell | WithoutManaCost$ True | ConditionTargetValidTargeting$ Card | ConditionDefined$ TopOfLibrary\n" +
	"Oracle:x\n"

// effectTwoRefusedScript is an Effect whose Continuous static carries a
// library MayPlay (outside the top-card look shape) and ValidSA, so the
// refusal loop has two candidates and must name the sorted-first one.
const effectTwoRefusedScript = "Name:Test Librarian\nManaCost:2 U\nTypes:Sorcery\n" +
	"A:SP$ Effect | StaticAbilities$ STPlay | SpellDescription$ x\n" +
	"SVar:STPlay:Mode$ Continuous | MayPlay$ True | Affected$ Card.YouOwn | AffectedZone$ Library | ValidSA$ Spell\n" +
	"Oracle:x\n"

// TestPlayConditionKeySorted pins the sorted-first key named by the two
// refusal loops that used to iterate a params map. The inline Play carries
// two refused Condition keys (ConditionDefined, ConditionTargetValidTargeting);
// the Effect static Continuous carries MayPlay and ValidSA. Without the sorted iteration the named key changes from run to run.
func TestPlayConditionKeySorted(t *testing.T) {
	collect := func(c *lCard) []string {
		var named []string
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "Play Condition") || strings.Contains(r.Reason, "Continuous param ") {
				named = append(named, r.Reason)
			}
		}
		return named
	}
	check := func(card string, i int, named []string, want string) {
		t.Helper()
		if len(named) != 1 || !strings.HasSuffix(named[0], want) {
			t.Fatalf("%s run %d: named refusals = %q, want exactly one ending %q", card, i, named, want)
		}
	}
	for i := 0; i < 20; i++ {
		c := lowerScriptCard(t, "Test Peeker", playCondTwoRefusedScript)
		check("Test Peeker", i, collect(c), "Play ConditionDefined")
	}
	for i := 0; i < 20; i++ {
		c := lowerScriptCard(t, "Test Librarian", effectTwoRefusedScript)
		check("Test Librarian", i, collect(c), "Effect static Continuous param MayPlay")
	}
}
