package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestThorinAttachLowers: Thorin, Mountain-king's ETB attach and reflexive
// capture lower -- the plural Object$ ParentTarget AttachObj that remembers
// what it attached, the reflexive trigger handed the Attach's own creature
// target (slot 1) and its body reading that creature -- shown on the card's
// chain with a Fight body (Curse of the Werefox's DelayTriggerRemembered read). The real card stays
// refused on its X = TriggerRemembered$CardPower: gorge reads the captured
// creature off the battlefield (the printed power of the card in its new
// zone), where the kernel skips the damage.
func TestThorinAttachLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Thorin, Mountain-king"})
	if len(cs) != 1 || cs[0].Name != "Thorin, Mountain-king" {
		t.Fatalf("lowered %d cards, want Thorin", len(cs))
	}
	var reasons []string
	for _, r := range cs[0].Refusals {
		reasons = append(reasons, r.Reason)
	}
	kdefWant(t, reasons, "TriggerRemembered$CardPower of a captured creature")
	for _, r := range reasons {
		if strings.Contains(r, "Attach") || strings.Contains(r, "RememberObjects") {
			t.Errorf("the attach or the capture refused: %s", r)
		}
	}

	c := kremw4Card(t, thorinFightSVars())
	for _, r := range c.Refusals {
		t.Errorf("Thorin's chain with a Fight body refused: %s", r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	lir := sb.String()
	for _, want := range []string{
		"AttachObj obj=Target1 obj2=Target0 // remember attached",
		"ReflexiveTrigger obj=Target1",
		"DelayedRemembered",
	} {
		if !strings.Contains(lir, want) {
			t.Errorf("lowered IR lacks %q:\n%s", want, lir)
		}
	}
}

// thorinSVars is Thorin, Mountain-king's ETB chain as a synthetic sorcery
// (Artifact for Equipment: the synthetic pool has no subtype registry).
func thorinSVars() map[string]string {
	return map[string]string{
		"R":   "SP$ Pump | ValidTgts$ Artifact.YouCtrl | TargetMin$ 0 | TargetMax$ M | SubAbility$ DBA",
		"DBA": "DB$ Attach | Object$ ParentTarget | ValidTgts$ Creature.YouCtrl | RememberAttached$ True | SubAbility$ DBI",
		"DBI": thorinReflex,
		"TD":  "DB$ DealDamage | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 1 | DamageSource$ DelayTriggerRemembered | NumDmg$ X",
		"DBC": "DB$ Cleanup | ClearRemembered$ True",
		"X":   "TriggerRemembered$CardPower",
		"M":   "Count$Valid Artifact.YouCtrl",
	}
}

// thorinFightSVars is thorinSVars with the reflexive body a Fight of the
// captured creature (no TriggerRemembered$CardPower read).
func thorinFightSVars() map[string]string {
	sv := thorinSVars()
	sv["TD"] = "DB$ Fight | Defined$ DelayTriggerRemembered | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 1"
	delete(sv, "X")
	return sv
}

const thorinReflex = "DB$ ImmediateTrigger | ConditionDefined$ Remembered | ConditionPresent$ Card | ConditionCompare$ GE1 | Execute$ TD | RememberObjects$ ParentTarget | SubAbility$ DBC"

// kremw4Card is kremw4Lowered's lowered card itself.
func kremw4Card(t *testing.T, svars map[string]string) *lCard {
	t.Helper()
	r := cards.ResolveSVar(svars, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", svars["R"])
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "B", Types: []string{"Sorcery"}, SVars: svars, Abilities: []*cards.SA{r}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}

// TestThorinAttachRefusals: the shapes around Thorin's that stay refused.
func TestThorinAttachRefusals(t *testing.T) {
	base := thorinFightSVars
	t.Run("synthetic Thorin with a Fight body lowers", func(t *testing.T) {
		if got := kremw4Lowered(t, base()); len(got) != 0 {
			t.Fatalf("refused: %q", got)
		}
	})
	t.Run("CardPower of a captured creature", func(t *testing.T) {
		kdefWant(t, kremw4Lowered(t, thorinSVars()), "TriggerRemembered$CardPower of a captured creature")
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
		sv["DBI"] = strings.Replace(thorinReflex, " | RememberObjects$ ParentTarget", "", 1)
		sv["TD"] = "DB$ DealDamage | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 1 | NumDmg$ X"
		sv["X"] = "TriggerRemembered$CardPower"
		kdefWant(t, kremw4Lowered(t, sv), "TriggerRemembered$CardPower outside a reflexive trigger handed one object")
	})
}
