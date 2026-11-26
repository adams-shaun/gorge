package kgen

import "testing"

// triggerRegProbe is a creature with one trigger line; MODE is its Mode$ body.
const triggerRegProbe = `Name:Trigger Reg Probe
ManaCost:1 G
Types:Creature Bear
PT:2/2
T:MODE | Execute$ TrigDraw | TriggerDescription$ probe
SVar:TrigDraw:DB$ Draw | NumCards$ 1 | Defined$ You
Oracle:probe
`

// TestTriggerModeRegistry covers the trigger-mode registry (trigger_reg.go): a
// duplicate registration panics, a fresh registration is dispatched by
// lowerTriggered ahead of the switch, and both moved modes (ChangesZone,
// SpellCast, BecomesTarget) and an unmoved one (Untaps) still lower.
func TestTriggerModeRegistry(t *testing.T) {
	for _, m := range []string{"ChangesZone", "SpellCast", "SpellCastOrCopy", "BecomesTarget"} {
		if triggerLowers[m] == nil {
			t.Fatalf("mode %s is not registered", m)
		}
		if !regPanic(func() { triggerMode(m, trigChangesZone) }) {
			t.Fatalf("duplicate registration of %s did not panic", m)
		}
	}
	if triggerLowers["Untaps"] != nil {
		t.Fatal("Untaps is registered; the unmoved-mode half of this test needs a switch arm")
	}

	// a fresh registration runs in place of the switch (and of the default refusal)
	ran := false
	triggerMode("RegTestMode", func(lc *lowerCtx, a *trigArm) error {
		ran = true
		a.tr.Event = "ETB"
		a.tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
		return nil
	})
	t.Cleanup(func() { delete(triggerLowers, "RegTestMode") })
	c := lowerProbe(t, replaceMode("Mode$ RegTestMode"), "Trigger Reg Probe", "")
	if !ran {
		t.Fatal("registered mode body never ran: lowerTriggered did not dispatch through triggerLowers")
	}
	if r := probeRefusals(c); r != "" {
		t.Fatalf("registered mode refused: %s", r)
	}

	for _, tc := range []struct{ mode, event string }{
		{"Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self", "ETB"},                // moved
		{"Mode$ SpellCast | ValidCard$ Card | ValidActivatingPlayer$ You | TriggerZones$ Battlefield", "SpellCast"}, // moved
		{"Mode$ BecomesTarget | ValidTarget$ Card.Self | ValidSource$ SpellAbility.OppCtrl | TriggerZones$ Battlefield", "BecomesTarget"},
		{"Mode$ Untaps | ValidCard$ Card.Self", "Untaps"}, // unmoved
	} {
		c := lowerProbe(t, replaceMode(tc.mode), "Trigger Reg Probe", "")
		if r := probeRefusals(c); r != "" {
			t.Errorf("%s: refused: %s", tc.mode, r)
			continue
		}
		var ev []string
		for _, ab := range c.Abilities {
			if ab.Trigger != nil {
				ev = append(ev, ab.Trigger.Event)
			}
		}
		if len(ev) != 1 || ev[0] != tc.event {
			t.Errorf("%s: trigger events %v, want [%s]", tc.mode, ev, tc.event)
		}
	}

	// an unregistered, unswitched mode still refuses by name
	if r := probeRefusals(lowerProbe(t, replaceMode("Mode$ NoSuchMode"), "Trigger Reg Probe", "")); r == "" {
		t.Fatal("an unknown trigger mode lowered")
	}
}

func replaceMode(mode string) string {
	const marker = "MODE"
	for i := 0; i+len(marker) <= len(triggerRegProbe); i++ {
		if triggerRegProbe[i:i+len(marker)] == marker {
			return triggerRegProbe[:i] + mode + triggerRegProbe[i+len(marker):]
		}
	}
	panic("no marker")
}
