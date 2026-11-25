package kgen

import (
	"strings"
	"testing"
)

// TestCopySpellController: a spell copy's Controller$ (gorge
// copyControllersFor) lowers into the CopySpell's W. Meletis Charlatan's
// TargetedController is the targeted spell's controller, Bonus Round's
// TriggeredCardController the caster, and Hive Mind's
// NonTriggeredCardController is C&4: one copy for each other living player.
func TestCopySpellController(t *testing.T) {
	want := map[string]struct {
		w    refKind
		each bool
	}{
		"Meletis Charlatan": {rTargetController, false},
		"Bonus Round":       {rTriggeredPlayer, false},
		"Hive Mind":         {rTriggeredPlayer, true},
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	cs := lowerNamed(t, names)
	if len(cs) != len(want) {
		t.Fatalf("%d cards, want %d", len(cs), len(want))
	}
	for _, c := range cs {
		w := want[c.Name]
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		copies := 0
		walk := func(ab *lAbility) {
			for _, in := range ab.Prog {
				if in.Op != "CopySpell" {
					continue
				}
				copies++
				if in.W.Kind != w.w {
					t.Errorf("%s: CopySpell W %v, want %v", c.Name, in.W, w.w)
				}
				if got := in.C&4 != 0; got != w.each {
					t.Errorf("%s: C=%d, each-other copy %v, want %v", c.Name, in.C, got, w.each)
				}
				if in.C&1 == 0 {
					t.Errorf("%s: C=%d, want MayChooseTarget (C&1)", c.Name, in.C)
				}
			}
		}
		for _, ab := range c.Abilities {
			walk(ab)
		}
		if copies != 1 {
			t.Errorf("%s: %d CopySpell instrs, want 1", c.Name, copies)
		}
	}
}

// TestCopySpellControllerUnknownRefuses: a selector the kernel does not
// model stays unread.
func TestCopySpellControllerUnknownRefuses(t *testing.T) {
	c := lowerSynth(t, "Name:Synth Copier\nManaCost:2 U\nTypes:Creature Human\nPT:1/1\n"+
		"A:AB$ CopySpellAbility | Cost$ T | ValidTgts$ Instant,Sorcery | TargetType$ Spell | Controller$ ChosenPlayer | SpellDescription$ Copy.\n")
	found := false
	for _, r := range c.Refusals {
		if strings.Contains(r.Reason, "Controller") {
			found = true
		}
	}
	if !found {
		t.Errorf("Controller$ ChosenPlayer copy lowered (refusals %v), want it unread", c.Refusals)
	}
}
