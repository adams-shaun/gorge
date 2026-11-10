package kgen

import (
	"strings"
	"testing"
)

// TestThorinAttachLowers: Thorin, Mountain-king's ETB lowers whole -- the
// plural Object$ ParentTarget AttachObj that remembers what it attached, the
// reflexive trigger handed the Attach's own creature target (slot 1), and the
// reflexive Bite whose source is that captured creature.
func TestThorinAttachLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Thorin, Mountain-king"})
	if len(cs) != 1 || cs[0].Name != "Thorin, Mountain-king" {
		t.Fatalf("lowered %d cards, want Thorin", len(cs))
	}
	c := cs[0]
	for _, r := range c.Refusals {
		t.Errorf("refused: %s", r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	lir := sb.String()
	for _, want := range []string{
		"AttachObj obj=Target1 obj2=Target0",
		"remember attached",
		"ReflexiveTrigger obj=Target1",
		"Bite obj=DelayedRemembered",
	} {
		if !strings.Contains(lir, want) {
			t.Errorf("lowered IR lacks %q:\n%s", want, lir)
		}
	}
}

// TestThorinAttachRefusals: the shapes around Thorin's that stay refused.
func TestThorinAttachRefusals(t *testing.T) {
	pump := "SP$ Pump | ValidTgts$ Artifact.YouCtrl | TargetMin$ 0 | TargetMax$ M | SubAbility$ DBA"
	reflex := "DB$ ImmediateTrigger | ConditionDefined$ Remembered | ConditionPresent$ Card | ConditionCompare$ GE1 | Execute$ TD | RememberObjects$ ParentTarget | SubAbility$ DBC"
	dmg := "DB$ DealDamage | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 1 | DamageSource$ DelayTriggerRemembered | NumDmg$ X"
	base := func() map[string]string {
		return map[string]string{
			"R":   pump,
			"DBA": "DB$ Attach | Object$ ParentTarget | ValidTgts$ Creature.YouCtrl | RememberAttached$ True | SubAbility$ DBI",
			"DBI": reflex,
			"TD":  dmg,
			"DBC": "DB$ Cleanup | ClearRemembered$ True",
			"X":   "TriggerRemembered$CardPower",
			"M":   "Count$Valid Artifact.YouCtrl",
		}
	}
	t.Run("synthetic Thorin lowers", func(t *testing.T) {
		if got := kremw4Lowered(t, base()); len(got) != 0 {
			t.Fatalf("refused: %q", got)
		}
	})
	t.Run("Object Targeted over several", func(t *testing.T) {
		kdefWant(t, kremw4Lowered(t, map[string]string{
			"R": "SP$ Attach | Object$ Targeted | ValidTgts$ Artifact.YouCtrl | TargetMin$ 0 | TargetMax$ M | Defined$ Self",
			"M": "Count$Valid Artifact.YouCtrl"}), "takes several objects")
	})
	t.Run("RememberAttached False", func(t *testing.T) {
		sv := base()
		sv["DBA"] = strings.Replace(sv["DBA"], "RememberAttached$ True", "RememberAttached$ False", 1)
		kdefWant(t, kremw4Lowered(t, sv), `RememberAttached$ "False"`)
	})
	t.Run("RememberObjects ParentTarget over a several-object slot", func(t *testing.T) {
		sv := base()
		// no own target: the nearest targeting link is the Pump's several-object slot
		sv["DBA"] = "DB$ Attach | Object$ ParentTarget | Defined$ Self | RememberAttached$ True | SubAbility$ DBI"
		kdefWant(t, kremw4Lowered(t, sv), "ParentTarget over a several-object slot")
	})
	t.Run("CardPower without a captured object", func(t *testing.T) {
		sv := base()
		sv["DBI"] = strings.Replace(reflex, " | RememberObjects$ ParentTarget", "", 1)
		kdefWant(t, kremw4Lowered(t, sv), "TriggerRemembered$CardPower outside a reflexive trigger handed one object")
	})
}
