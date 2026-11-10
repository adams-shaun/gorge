package kgen

import (
	"strings"
	"testing"
)

// TestSecondFixedManaAbility: a second mana ability beside a plain first one
// that the kernel cannot play as gorge's planner does stays refused, and the
// refusal names what the second ability needs: a cost beyond {T} (Weather
// Maker's counters, Scene of the Crime's tapped creature), a spend-restricted
// fixed colour of several mana (Tablet of Discovery's {R}{R}) and an
// evaluated amount of the first ability's colour (Itlimoc, Cradle of the Sun,
// the back of Growing Rites of Itlimoc). Lowering any of them diverged from
// gorge in lockstep, or the kernel's mana walk refuses the cost, so each
// refusal is by name.
func TestSecondFixedManaAbility(t *testing.T) {
	want := map[string]string{
		"Weather Maker":            "a cost beyond {T} the kernel's mana walk cannot activate: its tap-another and counter gap kinds read only the first mana ability",
		"Scene of the Crime":       "a cost beyond {T} the kernel's mana walk cannot activate: its tap-another and counter gap kinds read only the first mana ability",
		"Abandoned Outpost":        "a cost beyond {T} the kernel's mana walk cannot activate",
		"Tablet of Discovery":      "a restricted fixed colour of several mana",
		"Growing Rites of Itlimoc": "an evaluated amount of the first ability's colour",
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	lw := lowerNamedPool(t, names)
	// a double-faced card is several entries (the front, the back's face
	// entry): the back's carries the mana refusal
	got := map[string]string{}
	for _, c := range append(append([]*lCard(nil), lw.cards...), lw.tokens...) {
		name := c.Name
		if n, _, ok := parseFaceKey(name); ok {
			name = n
		}
		if _, ok := want[name]; !ok {
			continue
		}
		for _, r := range c.Refusals {
			got[name] += r.Reason + "; "
		}
	}
	seen := 0
	for n, w := range want {
		if got[n] == "" {
			t.Errorf("%s lowered clean, want the second-ability refusal %q", n, w)
			continue
		}
		seen++
		if !strings.Contains(got[n], "not modelled beside the first: "+w) {
			t.Errorf("%s refusals %q, want one naming %q", n, got[n], w)
		}
	}
	// the generic wording is a prefix of the tap-another/counter one, so pin
	// that a sacrifice cost does not carry the gap-kind tail
	if g := got["Abandoned Outpost"]; strings.Contains(g, "gap kinds") {
		t.Errorf("Abandoned Outpost refusals %q name the tap-another and counter gap kinds, its cost is a sacrifice", g)
	}
	if seen != len(want) {
		t.Fatalf("refused %d of %d cards", seen, len(want))
	}
}
