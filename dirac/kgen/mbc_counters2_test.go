package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMoveCounterTargetOrigin: a MoveCounter with no Source$, a Defined$
// destination and a target lowers with the chosen target as the origin
// (gorge moveCounterOrigin: moveCounterNamesDestination takes the chosen
// set as the origin), Cytoplast Root-Kin's "{2}: Move a +1/+1 counter from
// target creature you control onto CARDNAME". Without Defined$ (the
// two-target shape) or without a target it stays refused.
func TestMoveCounterTargetOrigin(t *testing.T) {
	head := "Name:Move Probe\nManaCost:2 G\nTypes:Creature Elemental\nPT:1/1\n"
	ok := head + "A:AB$ MoveCounter | Cost$ 2 | Defined$ Self | ValidTgts$ Creature.YouCtrl | TgtPrompt$ Select target creature you control | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ Move a counter.\nOracle:x\n"
	d, rs := kwardpowBuild(t, "Move Probe", ok)
	if len(rs) != 0 {
		t.Fatalf("refused: %v", rs)
	}
	var mv *kop.Instr
	var ab *kop.Ability
	for i := range d.Abilities {
		for j := range d.Abilities[i].Prog {
			if d.Abilities[i].Prog[j].Op == kop.OpMoveCounter {
				ab, mv = &d.Abilities[i], &d.Abilities[i].Prog[j]
			}
		}
	}
	if mv == nil {
		t.Fatalf("no OpMoveCounter in %+v", d.Abilities)
	}
	if ab.NTargets != 1 || kop.Ref(mv.C) == kop.RefSelf || mv.Obj != kop.RefSelf {
		t.Fatalf("origin Ref %d, destination %d, %d targets: want the target as the origin and Self as the destination", mv.C, mv.Obj, ab.NTargets)
	}
	for name, src := range map[string]string{
		"no Defined$": head + "A:AB$ MoveCounter | Cost$ 2 | ValidTgts$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ Move a counter.\nOracle:x\n",
		"no target":   head + "A:AB$ MoveCounter | Cost$ 2 | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ Move a counter.\nOracle:x\n",
	} {
		if _, rs := kwardpowBuild(t, "Move Probe", src); len(rs) == 0 || !strings.Contains(rs[0].Reason, "MoveCounter Source") {
			t.Errorf("%s: want a MoveCounter Source refusal, got %v", name, rs)
		}
	}
}

// TestAnyActivatorLowers: Activator$ Player on a mana-only PutCounter on the
// source lowers to kop.Ability.AnyActivator (Feral Hydra); every other
// spelling of the gate stays refused.
func TestAnyActivatorLowers(t *testing.T) {
	head := "Name:Hydra Probe\nManaCost:G\nTypes:Creature Hydra\nPT:1/1\n"
	line := func(cost, rest string) string {
		return head + "A:AB$ PutCounter | Cost$ " + cost + " | CounterType$ P1P1 | CounterNum$ 1 | " + rest + "SpellDescription$ x.\nOracle:x\n"
	}
	d, rs := kwardpowBuild(t, "Hydra Probe", line("3", "Activator$ Player | "))
	if len(rs) != 0 {
		t.Fatalf("refused: %v", rs)
	}
	got := false
	for _, a := range d.Abilities {
		got = got || a.Kind == kop.AbActivated && a.AnyActivator
	}
	if !got {
		t.Fatalf("no AnyActivator ability in %+v", d.Abilities)
	}
	for name, src := range map[string]string{
		"Opponent":     line("3", "Activator$ Opponent | "),
		"tap cost":     line("3 T", "Activator$ Player | "),
		"X cost":       line("X", "Activator$ Player | "),
		"with a limit": line("3", "Activator$ Player | ActivationLimit$ 1 | "),
		"a target":     line("3", "Activator$ Player | ValidTgts$ Creature | "),
	} {
		if _, rs := kwardpowBuild(t, "Hydra Probe", src); len(rs) == 0 {
			t.Errorf("%s: lowered, want a refusal", name)
		}
	}
}
