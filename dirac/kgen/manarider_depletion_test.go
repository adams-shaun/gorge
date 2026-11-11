package kgen

import (
	"strings"
	"testing"
)

// TestManaRiderBoundedShapes: the mana abilities whose rider does more than
// produce mana lower when the rider is one of the bounded shapes the kernel
// runs at activation (CR 605.3b): a self-sacrifice once the source's last
// counter is gone (the depletion lands and Gemstone Mine), or each opponent
// gaining life (Grove of the Burnwillows). Avid Reclaimer's conditional Nissa
// rider stays refused by name with a true reason.
func TestManaRiderBoundedShapes(t *testing.T) {
	want := map[string]struct {
		rider int
		ctr   string
		each  bool
	}{
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

// TestManaDrawStackAbilities: an AB$ Mana whose chain draws is no mana ability
// (CR 605.1a: it moves a card from a library; cards.IsManaAbilitySA), so gorge
// puts it on the stack. Chromatic Sphere and the eggs lower as activated
// abilities whose program adds the mana and then draws, never as an akMana
// source the payment windows could tap.
func TestManaDrawStackAbilities(t *testing.T) {
	want := map[string][]string{
		"Chromatic Sphere": {"ManaChoice", "Draw"},
		"Shadowblood Egg":  {"AddMana", "Draw"},
		"Skycloud Egg":     {"AddMana", "Draw"},
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		ops, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
			continue
		}
		var act *lAbility
		for _, a := range c.Abilities {
			if a.Kind == akMana {
				t.Errorf("%s: lowered as a mana ability", c.Name)
			}
			if a.Kind == akActivated && len(a.Prog) > 0 {
				act = a
			}
		}
		if act == nil {
			t.Errorf("%s: no activated ability with a program", c.Name)
			continue
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		for _, op := range ops {
			if !strings.Contains(sb.String(), op) {
				t.Errorf("%s: %s missing from the program:\n%s", c.Name, op, sb.String())
			}
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d cards", seen, len(want))
	}
}
