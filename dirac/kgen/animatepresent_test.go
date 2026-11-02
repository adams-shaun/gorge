package kgen

import (
	"strings"
	"testing"
)

// kverify-c4: the lowering fixes the verification lockstep proved.

// TestAnimateSetsOnlyThePresentPT: an Animate sets power and toughness
// only when present (gorge effAnimate's SetPowerPresent /
// SetToughnessPresent); a value the evaluator cannot read is refused,
// never read as 0.
func TestAnimateSetsOnlyThePresentPT(t *testing.T) {
	accepted := map[string]string{
		"Symmetry Sage":      "override{pt=2/0 keepT}", // Power$ 2 only: base power 2, toughness kept
		"Chariot of the Sun": "override{pt=0/1 keepP kw=Flying}",
		"Karn, Silver Golem": "override{pt=MVOf(Target0)/MVOf(Target0) addtypes=6}",
		"Toymaker":           "override{pt=MVOf(Target0)/MVOf(Target0) addtypes=6}",
	}
	refused := map[string]string{
		"Cactus Preserve": `Animate Power$ "X"`,
	}
	var names []string
	for n := range accepted {
		names = append(names, n)
	}
	for n := range refused {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		if want, ok := accepted[c.Name]; ok {
			seen++
			for _, r := range c.Refusals {
				t.Errorf("%s: refused: %s", c.Name, r.Reason)
			}
			var sb strings.Builder
			dumpLIR(&sb, c)
			if !strings.Contains(sb.String(), want) {
				t.Errorf("%s: lowered IR lacks %q:\n%s", c.Name, want, sb.String())
			}
		}
		if want, ok := refused[c.Name]; ok {
			seen++
			found := false
			for _, r := range c.Refusals {
				found = found || strings.Contains(r.Reason, want)
			}
			if !found {
				t.Errorf("%s: no refusal containing %q (refusals %v)", c.Name, want, c.Refusals)
			}
		}
	}
	if seen != len(accepted)+len(refused) {
		t.Errorf("lowered %d of %d cards", seen, len(accepted)+len(refused))
	}
}

// TestAnimateAddTypeSupertype: an Animate Types$ supertype joins AddTypes
// (CR 205.4b, 613.1d) instead of being refused as an unknown subtype; the
// residual blockers on the other four cards belong to other refusal classes.
func TestAnimateAddTypeSupertype(t *testing.T) {
	names := []string{"Cacophony Unleashed", "Awakening of Vitu-Ghazi", "The Irencrag",
		"Sarkhan, the Dragonspeaker", "Genju of the Realm"}
	for _, c := range lowerNamed(t, names) {
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, `Animate type "Legendary"`) {
				t.Errorf("%s: supertype still refused: %s", c.Name, r.Reason)
			}
		}
		if c.Name != "Cacophony Unleashed" {
			continue
		}
		if len(c.Refusals) != 0 {
			t.Errorf("Cacophony Unleashed: refused: %v", c.Refusals)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		want := "override{pt=6/6 addtypes=258 addsub=Nightmare,God kw=Menace&Deathtouch}"
		if !strings.Contains(sb.String(), want) {
			t.Errorf("Cacophony Unleashed: lowered IR lacks %q:\n%s", want, sb.String())
		}
	}
}

// TestTriggerXPaidRefused: a trigger reading Count$xPaid without an {X}
// cost of its own binds the X of its trigger object's spell (kxpaid, CR
// 107.3m) only for a self enters trigger and a SpellCast trigger of a
// source with no {X}; the rest stay refused (Hydroid Krasis' own cast
// trigger, which the kernel never fires from the stack; Rampaging War
// Verdeloth's kicker {X}); a card's own cycle trigger binds the cycling
// activation's X (Rampaging War Mammoth, Valor's Flagship).
func TestTriggerXPaidRefused(t *testing.T) {
	want := map[string]bool{"Triceraton Commander": false, "Sporocyst": false, "Zaxara, the Exemplary": false, "Walking Ballista": false,
		"Hydroid Krasis": false, "Old Stickfingers": false, "Rampaging War Mammoth": false, "Verdeloth the Ancient": true} // Krasis, Stickfingers: the spell's own cast trigger (selfcast)
	var names []string
	for n := range want {
		names = append(names, n)
	}
	for _, c := range lowerNamed(t, names) {
		found := false
		for _, r := range c.Refusals {
			found = found || strings.Contains(r.Reason, "Count$xPaid in a trigger without an {X} cost")
		}
		if found != want[c.Name] {
			t.Errorf("%s: xPaid refusal %v, want %v (refusals %v)", c.Name, found, want[c.Name], c.Refusals)
		}
	}
}
