package kgen

import (
	"strings"
	"testing"
)

// TestManaRiderBoundedShapes: the mana abilities whose rider does more than
// produce mana lower when the rider is one of the bounded shapes the kernel
// runs at activation (CR 605.3b): a draw (Chromatic Sphere, the eggs), a
// self-sacrifice once the source's last counter is gone (the depletion lands
// and Gemstone Mine), or each opponent gaining life (Grove of the
// Burnwillows). Avid Reclaimer's conditional Nissa rider stays refused by name
// with a true reason.
func TestManaRiderBoundedShapes(t *testing.T) {
	want := map[string]struct {
		rider int
		ctr   string
		each  bool
	}{
		"Chromatic Sphere":         {rider: riderDraw},
		"Shadowblood Egg":          {rider: riderDraw, each: true},
		"Skycloud Egg":             {rider: riderDraw, each: true},
		"Gemstone Mine":            {rider: riderDeplete, ctr: "MINING"},
		"Peat Bog":                 {rider: riderDeplete, ctr: "DEPLETION"},
		"Hickory Woodlot":          {rider: riderDeplete, ctr: "DEPLETION"},
		"Remote Farm":              {rider: riderDeplete, ctr: "DEPLETION"},
		"Sandstone Needle":         {rider: riderDeplete, ctr: "DEPLETION"},
		"Saprazzan Skerry":         {rider: riderDeplete, ctr: "DEPLETION"},
		"Grove of the Burnwillows": {rider: riderOppGainLife},
	}
	names := make([]string, 0, len(want)+1)
	for n := range want {
		names = append(names, n)
	}
	names = append(names, "Avid Reclaimer")
	seen := 0
	for _, c := range lowerNamed(t, names) {
		if c.Name == "Avid Reclaimer" {
			found := false
			for _, r := range c.Refusals {
				found = found || strings.Contains(r.Reason, "conditional or unbounded rider program")
			}
			if !found {
				t.Errorf("Avid Reclaimer: want the conditional-rider refusal, got %v", c.Refusals)
			}
			continue
		}
		w, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
			continue
		}
		if !manaRiderShape(c, w.rider, w.ctr, w.each) {
			t.Errorf("%s: no mana ability with rider %d, counter %q, each %v", c.Name, w.rider, w.ctr, w.each)
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d rider cards", seen, len(want))
	}
}

func manaRiderShape(c *lCard, rider int, ctr string, each bool) bool {
	for _, a := range c.Abilities {
		if a.Kind == akMana && a.Mana != nil && a.Mana.Rider == rider && a.Mana.RiderCtr == ctr && a.Mana.Each == each {
			return true
		}
	}
	return false
}
