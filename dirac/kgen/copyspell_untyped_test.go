package kgen

import (
	"strings"
	"testing"
)

// TestCopySpellUntypedTarget: "copy target instant or sorcery spell" with no
// TargetType$ lowers as the Kitsa TargetType$ Spell form. gorge's targeting
// (effects/params compileTargets) defaults the stack kinds to spells, and a
// ValidTgts$ Instant,Sorcery with no TgtZone$ targets the stack
// (rules/target_legal.go), so an absent TargetType$ means Spell when every
// clause is bounded to instant|sorcery.
func TestCopySpellUntypedTarget(t *testing.T) {
	plain := []string{"Sigil Tracer", "Uyo, Silent Prophet", "Rootha, Mercurial Artist"}
	granted := []string{"Echo Mage", "Dual Casting"}
	cs := lowerNamed(t, append(plain[:len(plain):len(plain)], granted...))
	if len(cs) != len(plain)+len(granted) {
		t.Fatalf("%d cards, want %d", len(cs), len(plain)+len(granted))
	}
	byName := map[string]*lCard{}
	for _, c := range cs {
		byName[c.Name] = c
	}
	for _, n := range plain {
		c := byName[n]
		if c == nil {
			t.Fatalf("%s: not lowered", n)
		}
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", n, r.Reason)
		}
		copies := 0
		for _, ab := range c.Abilities {
			for _, in := range ab.Prog {
				if in.Op != "CopySpell" {
					continue
				}
				copies++
				if in.Obj.Kind != rTarget || in.Obj.N >= len(ab.Targets) {
					t.Errorf("%s: CopySpell object %+v, want a declared target", n, in.Obj)
					continue
				}
				tg := ab.Targets[in.Obj.N]
				if tg.Zone != zStack || !tg.Spell {
					t.Errorf("%s: target zone %d spell %v, want a stack spell", n, tg.Zone, tg.Spell)
				}
				if in.C&1 == 0 {
					t.Errorf("%s: C=%d, want MayChooseTarget (C&1)", n, in.C)
				}
				if in.C&2 != 0 {
					t.Errorf("%s: C=%d, want no permanent copy (C&2) for an instant/sorcery filter", n, in.C)
				}
			}
		}
		if copies != 1 {
			t.Errorf("%s: %d CopySpell instrs, want 1", n, copies)
		}
	}
	for _, n := range granted {
		c := byName[n]
		if c == nil {
			t.Fatalf("%s: not lowered", n)
		}
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "TargetType") {
				t.Errorf("%s: refused on TargetType: %s", n, r.Reason)
			}
		}
	}
}

// TestCopySpellUntypedTargetUnboundedRefuses: without a type bound an
// absent TargetType$ is not admitted (the untyped filter keeps refusing).
func TestCopySpellUntypedTargetUnboundedRefuses(t *testing.T) {
	c := lowerSynth(t, "Name:Synth Copier\nManaCost:2 U\nTypes:Creature Human\nPT:1/1\n"+
		"A:AB$ CopySpellAbility | Cost$ T | ValidTgts$ Card | MayChooseTarget$ True | SpellDescription$ Copy.\n")
	found := false
	for _, r := range c.Refusals {
		if strings.Contains(r.Reason, "CopySpellAbility TargetType") {
			found = true
		}
	}
	if !found {
		t.Errorf("untyped ValidTgts$ Card copy lowered (refusals %v), want the TargetType refusal", c.Refusals)
	}
}
