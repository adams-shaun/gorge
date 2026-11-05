package kgen

import (
	"strings"
	"testing"
)

// effectAttacksIfAbleCarriers: an Effect's Mode$ MustAttack static over its
// remembered target lowers to OpExt ExtAttacksIfAble (lower_effectattacksifable.go).
// node is the Effect node that must carry no refusal ("" none); whole: the
// card must lower with none at all; lir a fragment of its lowered IR (""
// none). No carrier may still refuse its MustAttack static, nor the
// two-static list whole.
var effectAttacksIfAbleCarriers = map[string]struct {
	node  string
	whole bool
	lir   string
}{
	"Heckling Fiends": {"A[0] AB$ Effect", true, "AttacksIfAble obj=Target0"},
	"Incite":          {"A[0] SP$ Animate", true, "AttacksIfAble obj=Target0"},
	// A[1]'s MustBlock is the blocker duty (mkm-effect-must-block)
	"Courtly Provocateur": {"A[0] AB$ Effect", false, ""},
	// the two-static MustAttack,MustBlock list is no longer refused whole;
	// its MustBlock half lowers with the blocker duty
	// (at this base its A[0] node refuses that half: "Effect static mode
	// MustBlock")
	"Hustle // Bustle": {"", false, ""},
}

// effectAttacksIfAbleRefused: the shapes the per-object mark cannot hold,
// each refused by name.
var effectAttacksIfAbleRefused = map[string]string{
	"Alluring Siren":     `Effect static MustAttack$ "You"`,
	"Boros Battleshaper": `Effect MustAttack Duration$ "UntilEndOfCombat"`,
	"Imaginary Threats":  "Effect MustAttack player scope",
}

func TestEffectAttacksIfAbleAccepted(t *testing.T) {
	var names []string
	for n := range effectAttacksIfAbleCarriers {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := effectAttacksIfAbleCarriers[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			if want.whole || r.Node == want.node {
				t.Errorf("%s: refused at %s: %s", c.Name, r.Node, r.Reason)
			}
			if strings.Contains(r.Reason, "Effect static") && strings.Contains(r.Reason, "MustAttack") {
				t.Errorf("%s: the MustAttack static still refused at %s: %s", c.Name, r.Node, r.Reason)
			}
		}
		if want.lir == "" {
			continue
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), want.lir) {
			t.Errorf("%s: lowered IR lacks %q:\n%s", c.Name, want.lir, sb.String())
		}
	}
	if seen != len(effectAttacksIfAbleCarriers) {
		t.Errorf("lowered %d of %d carriers", seen, len(effectAttacksIfAbleCarriers))
	}
}

func TestEffectAttacksIfAbleRefused(t *testing.T) {
	var names []string
	for n := range effectAttacksIfAbleRefused {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := effectAttacksIfAbleRefused[c.Name]
		if !ok {
			continue
		}
		seen++
		found := false
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no refusal containing %q (refusals %v)", c.Name, want, c.Refusals)
		}
	}
	if seen != len(effectAttacksIfAbleRefused) {
		t.Errorf("lowered %d of %d refused carriers", seen, len(effectAttacksIfAbleRefused))
	}
}
