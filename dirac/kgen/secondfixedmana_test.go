package kgen

import (
	"strings"
	"testing"
)

// TestSecondFixedManaAbility: a second mana ability beside a plain first one
// that the kernel cannot play as gorge's planner does stays refused, and the
// refusal names what the second ability needs: a cost beyond {T} (Mirrodin's
// Core's counter and any colour, Abandoned Outpost's sacrifice), a
// spend-restricted fixed colour of several mana other than the first ability's
// or above maxRestrictedAmount (Tablet of Discovery's {R}{R} is modelled).
// Lowering any of them diverged from gorge in lockstep, or the kernel's mana
// walk refuses the cost, so each refusal is by name. An evaluated amount of
// the first ability's colour that tapping cannot change (Itlimoc, Cradle of
// the Sun, the back of Growing Rites of Itlimoc) lowers clean (the kernel's
// per-count pair view), while Vault of Catlacan (a second ability of another
// colour) and Elementalist's Palette (a spend-restricted counter count) stay
// refused.
func TestSecondFixedManaAbility(t *testing.T) {
	want := map[string]string{
		"Mirrodin's Core":   "a cost beyond {T} the kernel's mana walk cannot activate: its counter gap kind reads a second ability only for a fixed unrestricted colour",
		"Abandoned Outpost": "a cost beyond {T} the kernel's mana walk cannot activate",
		"Vault of Catlacan": "a shape beyond the restricted, filter and pain gaps",
	}
	const modelled = "Growing Rites of Itlimoc"
	const palette = "Elementalist's Palette" // refused earlier, for its CostContainsX restriction
	var names []string
	for n := range want {
		names = append(names, n)
	}
	lw := lowerNamedPool(t, append(names, modelled, palette))
	// a double-faced card is several entries (the front, the back's face
	// entry): the back's carries the mana refusal
	got := map[string]string{}
	twoMana := 0
	for _, c := range append(append([]*lCard(nil), lw.cards...), lw.tokens...) {
		name := c.Name
		if n, _, ok := parseFaceKey(name); ok {
			name = n
		}
		if name == modelled {
			// the back's face entry is the one with the two mana abilities
			if countMana(c) != 2 {
				continue
			}
			twoMana++
			for _, r := range c.Refusals {
				t.Errorf("%s (%s) refused: %s", modelled, c.Name, r.Reason)
			}
			continue
		}
		if name == palette && len(c.Refusals) == 0 {
			t.Errorf("%s lowered clean", palette)
		}
		if _, ok := want[name]; !ok {
			continue
		}
		for _, r := range c.Refusals {
			got[name] += r.Reason + "; "
		}
	}
	if twoMana != 1 {
		t.Errorf("%d lowered entries of %s carry two mana abilities, want its back face", twoMana, modelled)
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
	if g := got["Abandoned Outpost"]; strings.Contains(g, "gap kind") {
		t.Errorf("Abandoned Outpost refusals %q name the counter gap kind, its cost is a sacrifice", g)
	}
	if seen != len(want) {
		t.Fatalf("refused %d of %d cards", seen, len(want))
	}
}

// countMana is the number of mana abilities of c's lowered IR.
func countMana(c *lCard) int {
	n := 0
	for _, a := range c.Abilities {
		if a.Kind == akMana && !a.Latent {
			n++
		}
	}
	return n
}
