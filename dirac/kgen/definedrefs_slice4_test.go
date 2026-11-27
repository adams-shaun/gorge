package kgen

import (
	"strings"
	"testing"
)

// definedRefsSlice4Carriers are this slice's cards whose next blocker was the
// Defined$ selector itself, all lowered by this slice: the single-object
// referents (Artificer's Hex, Wall of Nets) name the source's equipped or
// blocked object; the set spellings lower onto the op-level filter subject
// (ChangeZoneAll, the RepeatEach card loop, or ExtRememberAll for an Effect's
// RememberObjects$); and Loki's IsSingleTarget$ arm reads the triggering
// spell's one object target (kop.RefTriggerSpellTarget).
var definedRefsSlice4Carriers = []string{
	"Loki, God of Lies",
	"Dack Fayden",
	"Storm, Windrider",
	"Aura Barbs",
	"Artificer's Hex",
	"Disarm",
	"Fulgent Distraction",
	"Energybending",
	"Gomazoa",
	"Hakbal of the Surging Soul",
	"Suicidal Charge",
	"Taunt from the Rampart",
	"Vortex Elemental",
	"Wall of Nets",
}

// definedRefsSlice4StillRefused are the cards whose ThisTargetedPlayer refusal
// is gone and whose next blocker is unrelated: PutCounterAll's placer and
// target uniqueness, and the loyalty-ability Counter.
var definedRefsSlice4StillRefused = map[string]string{
	"Shadrix Silverquill":          "PutCounterAll Placer$",
	"Splinter & Leo, Father & Son": "PutCounterAll TargetUnique$",
	"Verdant Command":              `Counter TargetType "Activated" without Triggered`,
}

func TestDefinedRefsSlice4(t *testing.T) {
	for _, name := range definedRefsSlice4Carriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
	for name, want := range definedRefsSlice4StillRefused {
		c := lowerNamedCard(t, name)
		var got strings.Builder
		for _, r := range c.Refusals {
			got.WriteString(r.Reason)
			got.WriteString("; ")
		}
		if !strings.Contains(got.String(), want) {
			t.Errorf("%s refusals %q, want one naming %q", name, got.String(), want)
		}
	}
}

// The lowered shapes: Loki's three body lines read the triggering spell's one
// object target, never a target slot of their own; Artificer's Hex destroys
// the object equipped by the enchanted Equipment; the set forms emit the
// op-level filter subject or a RepeatEach loop.
func TestDefinedRefsSlice4Shapes(t *testing.T) {
	c := lowerNamedCard(t, "Loki, God of Lies")
	got := map[string]bool{}
	for _, ab := range c.Abilities {
		if len(ab.Targets) != 0 {
			t.Errorf("Loki: trigger declares %d target slots, want none", len(ab.Targets))
		}
		for _, in := range ab.Prog {
			if in.Obj.Kind == rTriggeredSpellTarget {
				got[in.Op] = true
			}
		}
	}
	for _, op := range []string{"GainControl", "Untap", "GrantKeyword"} {
		if !got[op] {
			t.Errorf("Loki: no %s over the triggering spell's target (got %v)", op, got)
		}
	}

	if c := lowerNamedCard(t, "Artificer's Hex"); !hasInstr(c, "Destroy", rEquippedByEnchanted, "") {
		t.Errorf("Artificer's Hex: no Destroy over %s", refNames[rEquippedByEnchanted])
	}
	if c := lowerNamedCard(t, "Wall of Nets"); !hasInstr(c, "ChangeZoneAll", rNone, "") {
		t.Errorf("Wall of Nets: no ChangeZoneAll over a filter")
	}
	if c := lowerNamedCard(t, "Hakbal of the Surging Soul"); !hasInstr(c, "RepeatEachCard", rNone, "") || !hasInstr(c, "Explore", rEach, "") {
		t.Errorf("Hakbal: no RepeatEachCard/Explore loop")
	}
	if c := lowerNamedCard(t, "Energybending"); !hasInstr(c, "RememberAll", rNone, "") || !hasInstr(c, "CreateEmblem", rYou, "") {
		t.Errorf("Energybending: no RememberAll then CreateEmblem")
	}
	if c := lowerNamedCard(t, "Dack Fayden"); !hasInstr(c, "GainControl", rTriggeredSpellTargets, "") {
		t.Errorf("Dack Fayden: no GainControl over %s", refNames[rTriggeredSpellTargets])
	}
	if c := lowerNamedCard(t, "Storm, Windrider"); !hasInstr(c, "GrantKeyword", rTriggeredSpellTargets, "") {
		t.Errorf("Storm, Windrider: no GrantKeyword over %s", refNames[rTriggeredSpellTargets])
	}
	if c := lowerNamedCard(t, "Aura Barbs"); !hasInstr(c, "DealDamageFrom", rEach, "") {
		t.Errorf("Aura Barbs: no per-source DealDamageFrom loop")
	}
}
