package kgen

import (
	"slices"
	"strings"
	"testing"
)

// graspProbe is Realmbreaker's Grasp's shape: one compound static line whose
// ValidSA$ belongs to the CantBeActivated sibling but is shared by the
// CantAttack and CantBlock halves through the parse-time split.
const graspProbe = `Name:Probe Grasp
ManaCost:1 W
Types:Enchantment Aura
K:Enchant:Creature
S:Mode$ CantAttack,CantBlock,CantBeActivated | ValidCard$ Card.EnchantedBy | ValidSA$ Activated.!ManaAbility | Description$ Probe.
Oracle:Probe.
`

// TestStaticValidSACantAttackBlock: gorge reads ValidSA$ inert on a CantAttack
// and a CantBlock static (it belongs to the sibling CantBeActivated mode), so
// kgen lowers all three halves instead of refusing the shared param.
func TestStaticValidSACantAttackBlock(t *testing.T) {
	c := lowerScriptCard(t, "Probe", graspProbe)
	if len(c.Refusals) != 0 {
		t.Fatalf("compound line refused: %v", c.Refusals)
	}
	var kinds []string
	for _, st := range c.Statics {
		kinds = append(kinds, st.Kind)
	}
	t.Logf("statics: %v", kinds)
	if len(c.Statics) != 3 {
		t.Fatalf("lowered %d statics %v, want one per mode (CantAttack, CantBlock, CantBeActivated)", len(c.Statics), kinds)
	}
}

// TestStaticValidSAContinuousRefused: the Continuous MayPlay$ shape keeps its
// refusal (the alt-cost cast context has no plain match yet), now labelled with
// its mode so the census separates it from the compound-line carriers.
func TestStaticValidSAContinuousRefused(t *testing.T) {
	src := strings.Join([]string{
		"Name:Probe Phoenix",
		"ManaCost:2 R",
		"Types:Creature Phoenix",
		"PT:2/2",
		"S:Mode$ Continuous | Affected$ Card.Self | MayPlay$ True | ValidSA$ Spell.Blitz | AffectedZone$ Graveyard | EffectZone$ Graveyard | Description$ Probe.",
		"Oracle:Probe.",
	}, "\n")
	c := lowerScriptCard(t, "Probe", src)
	if len(c.Refusals) == 0 {
		t.Fatal("Continuous MayPlay$ ValidSA$ Spell.Blitz lowered")
	}
	got := c.Refusals[0].Reason
	if want := `static spell-ability filter ValidSA Continuous "Spell.Blitz"`; !strings.Contains(got, want) {
		t.Errorf("reason %q, want it to contain %q", got, want)
	}
	const class = `other:static spell-ability filter ValidSA Continuous "_"`
	if cl := refusalClasses(got); !slices.Equal(cl, []string{class}) {
		t.Errorf("classes %v, want [%s]", cl, class)
	}
	if tier := classTier(class); tier != "M5" {
		t.Errorf("classTier(%q) = %q, want M5", class, tier)
	}
}
