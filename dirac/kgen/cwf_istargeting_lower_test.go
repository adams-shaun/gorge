package kgen

import (
	"strings"
	"testing"
)

// TestSelfFlashIsTargetingLowers: the `S:Mode$ CastWithFlash | ValidCard$
// Card.Self | ValidSA$ Spell.IsTargeting Valid <spec> | EffectZone$ All`
// self grant lowers (Flash Photography, Timely Ward): the carrier is a
// SelfFlash card whose static carries the target spec as F2. Every other
// non-bare ValidSA$ shape still refuses by name.
func TestSelfFlashIsTargetingLowers(t *testing.T) {
	for _, tc := range []struct{ card, f2 string }{
		{"Flash Photography", "F2={types=0x9f ctrl=1}"},
		{"Timely Ward", "F2={types=0x0 cmp("},
	} {
		c := lowerNamedCard(t, tc.card)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", tc.card, c.Refusals)
			continue
		}
		if !c.SelfFlash {
			t.Errorf("%s: SelfFlash not set", tc.card)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		line := ""
		for _, l := range strings.Split(sb.String(), "\n") {
			if strings.Contains(l, "static CastWithFlash") {
				line = l
			}
		}
		if line == "" || !strings.Contains(line, tc.f2) || strings.Contains(line, "F2=nil") {
			t.Errorf("%s: CastWithFlash line %q lacks %q (F2 must be the target spec, not nil)", tc.card, line, tc.f2)
		}
	}
	// the control: a non-IsTargeting Spell.* word keeps its refusal (Quantum
	// Reduction's Spell.Teamwork since lower: cwf_teamwork_lower_test.go)
	src := "Name:Probe\nManaCost:2 W\nTypes:Enchantment\nS:Mode$ CastWithFlash | ValidCard$ Card | ValidSA$ Spell.Bogus | Caster$ You\nOracle:x\n"
	if got := strings.Join(lowerScript(t, "Probe", src), " | "); !strings.Contains(got, "CastWithFlash shape") {
		t.Errorf("Spell.Bogus probe: want a CastWithFlash shape refusal, got %q", got)
	}
}

// TestSelfFlashIsTargetingSpecShape: only a spec the caster-relative census
// reads is accepted; a source-relative or unknown word refuses.
func TestSelfFlashIsTargetingSpecShape(t *testing.T) {
	for spec, ok := range map[string]bool{
		"Permanent.YouCtrl": true, "Card.IsCommander": true, "Creature.OppCtrl": true,
		"Player": false, "Creature.Other": false, "Permanent.Self": false, "Creature.wibble": false,
	} {
		if got := selfFlashTargetingSpec.MatchString(spec); got != ok {
			t.Errorf("spec %q: accepted = %v, want %v", spec, got, ok)
		}
	}
}
