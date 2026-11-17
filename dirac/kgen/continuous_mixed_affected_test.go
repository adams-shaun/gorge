package kgen

import (
	"strings"
	"testing"
)

// TestContinuousMixedAffectedPTRefused: StPerAffected is ONE flag for both
// P/T amounts, so dirac's walk re-anchors both at each AFFECTED object, while
// gorge anchors per amount (engine/rules/chars/derive.go reads
// AddPowerAffected and AddToughnessAffected separately). A mixed shape -- one
// amount AffectedX, the other a dynamic grantor read -- has no shared
// lowering: the affected creature would feed the grantor's count. It is
// refused (fail-closed), while the shapes dirac does model stay accepted.
func TestContinuousMixedAffectedPTRefused(t *testing.T) {
	const probeCount = "Count$Card" + "Counters.P1P1"
	paired := "Name:Mixed Probe\nManaCost:W\nTypes:Creature Human Cleric\nPT:2/2\n" +
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AffectedZone$ Battlefield | AddPower$ AffectedX | AddToughness$ AffectedX | Description$ probe.\n" +
		"SVar:AffectedX:" + probeCount + "\n" +
		"SVar:Other:" + probeCount + "\n" +
		"Oracle:probe.\n"
	if r := lowerScript(t, "Mixed Probe", paired); len(r) != 0 {
		t.Fatalf("paired AffectedX pump refused: %v", r)
	}
	pc := lowerScriptCard(t, "Mixed Probe", paired)
	if len(pc.Statics) != 1 || !pc.Statics[0].PerAffected {
		t.Fatalf("paired AffectedX pump = %+v, want one PerAffected static", pc.Statics)
	}

	for _, tc := range []struct{ name, from, to string }{
		{"power AffectedX, toughness dynamic", "AddToughness$ AffectedX", "AddToughness$ Other"},
		{"toughness AffectedX, power dynamic", "AddPower$ AffectedX", "AddPower$ Other"},
	} {
		src := strings.Replace(paired, tc.from, tc.to, 1)
		r := lowerScript(t, "Mixed Probe", src)
		if len(r) == 0 || !strings.Contains(strings.Join(r, "\n"), "mixed AffectedX") {
			t.Errorf("%s: refusals %v, want one containing \"mixed AffectedX\"", tc.name, r)
		}
	}

	for _, tc := range []struct{ name, from, to string }{
		{"power AffectedX, toughness constant", "AddToughness$ AffectedX", "AddToughness$ 2"},
		{"both dynamic, none AffectedX", "AddPower$ AffectedX", "AddPower$ Other"},
	} {
		src := strings.Replace(paired, tc.from, tc.to, 1)
		if tc.name == "both dynamic, none AffectedX" {
			src = strings.Replace(src, "AddToughness$ AffectedX", "AddToughness$ Other", 1)
		}
		if r := lowerScript(t, "Mixed Probe", src); len(r) != 0 {
			t.Errorf("%s refused: %v", tc.name, r)
		}
	}

	// one AffectedX amount alone (Blessing of the Nephilim's power-only
	// cousins) stays accepted
	solo := strings.Replace(paired, " | AddToughness$ AffectedX", "", 1)
	if r := lowerScript(t, "Mixed Probe", solo); len(r) != 0 {
		t.Errorf("solo AffectedX amount refused: %v", r)
	}
	sc := lowerScriptCard(t, "Mixed Probe", solo)
	if len(sc.Statics) != 1 || !sc.Statics[0].PerAffected {
		t.Errorf("solo AffectedX amount = %+v, want one PerAffected static", sc.Statics)
	}
}
