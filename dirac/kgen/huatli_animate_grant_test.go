package kgen

import (
	"strings"
	"testing"
)

// TestHuatliPoetAnimateGrant: Roar of the Fifth People (Huatli, Poet of
// Unity's back face) chapter II animates itself with a Continuous body that
// grants creatures you control a mana ability (AddAbility$ Mana). The body
// lowers as a static of the Saga gated by the override record's GrantStatics
// bit (kop.Static.OvGate); the grant walks read the gate through statCond, so
// the ability is live only once the chapter resolved. Huatli lowers whole,
// the Dinosaur token included.
func TestHuatliPoetAnimateGrant(t *testing.T) {
	all, _ := keclBuild(t, []string{"Huatli, Poet of Unity"})
	var back *lCard
	for _, c := range all {
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", c.Name, c.Refusals)
		}
		if strings.HasPrefix(c.Name, "face:Huatli") {
			back = c
		}
	}
	if back == nil {
		t.Fatal("Roar of the Fifth People face not lowered")
	}
	if !strings.Contains(overrideLIR(back), "grantstatics=1") {
		t.Errorf("chapter II lacks the granted static:\n%s", overrideLIR(back))
	}
	gated := false
	for _, st := range back.Statics {
		if st.Kind == "Continuous" && st.OvGate != 0 && len(st.Grant) == 1 {
			gated = true
		}
	}
	if !gated {
		t.Errorf("Roar of the Fifth People has no OvGate Continuous mana grant: %+v", back.Statics)
	}

	// A synthetic anthem granting a mana ability lowers the same way; the
	// same grant on an attached object (the attached-grant walk reads no
	// gate) still refuses by name.
	const base = "Name:Animate Grant Probe\nManaCost:1\nTypes:Creature Human\nPT:1/1\n"
	const mana = "SVar:ABM:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ test\n"
	anthem := "A:AB$ Animate | Cost$ 1 | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
		"SVar:Static:Mode$ Continuous | Affected$ Creature.YouCtrl | AddAbility$ ABM | Description$ test\n" + mana
	if got := keclReason(keclSynth(t, "Animate Grant Probe", base+anthem)); got != "" {
		t.Errorf("anthem granting a mana ability: refused: %s", got)
	}
	equipped := "A:AB$ Animate | Cost$ 1 | Defined$ Self | Duration$ Permanent | staticAbilities$ Static | SpellDescription$ test\n" +
		"SVar:Static:Mode$ Continuous | AffectedDefined$ Equipped | Affected$ Creature | AddAbility$ ABM | Description$ test\n" + mana
	if got := keclReason(keclSynth(t, "Animate Grant Probe", base+equipped)); !strings.Contains(got, "grants an ability the kernel does not gate") {
		t.Errorf("equipped mana grant: refusals %q, want the ungated grant", got)
	}
}
