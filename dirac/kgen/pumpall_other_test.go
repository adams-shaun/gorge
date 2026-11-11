package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

func pumpAllOtherLower(t *testing.T, src string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("pumpall_other_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerPool(p).cards[0]
}

// TestPumpAllOtherThanTarget: a Pump of one creature you control, then a
// PumpAll of each OTHER creature you control (the Targeted.Other base: CR
// 608.2c, Forge filterListByType), lowers as one PumpAll over every such
// creature, the target included, plus the target's own pump cut to the
// difference. Any shape beyond it (the target not a creature you control, a
// larger sweep than pump, a counter instead of a pump, a keyword on the sweep)
// refuses by name.
func TestPumpAllOtherThanTarget(t *testing.T) {
	const head = "Name:Probe\nManaCost:1 W\nTypes:Instant\n"
	spell := func(tgt, firstAPI, firstParams, sweep string) string {
		return head + "A:SP$ " + firstAPI + " | ValidTgts$ " + tgt + " | " + firstParams + " | SubAbility$ DBSweep\n" +
			"SVar:DBSweep:DB$ PumpAll | ValidCards$ Targeted.Other+YouCtrl+Creature | " + sweep + "\n"
	}
	c := pumpAllOtherLower(t, spell("Creature.YouCtrl", "Pump", "NumAtt$ +3 | NumDef$ +2", "NumAtt$ +1 | NumDef$ +1"))
	if len(c.Refusals) != 0 {
		t.Fatalf("refused: %v", c.Refusals)
	}
	prog := c.Abilities[0].Prog
	if len(prog) != 2 || prog[0].Op != "PumpUntilEOT" || prog[1].Op != "PumpAll" {
		t.Fatalf("lowered to %v, want PumpUntilEOT then PumpAll", prog)
	}
	if prog[0].A.N != 2 || prog[0].B.N != 1 || prog[1].A.N != 1 || prog[1].B.N != 1 {
		t.Errorf("target pump %d/%d, sweep %d/%d, want the difference 2/1 and the sweep 1/1",
			prog[0].A.N, prog[0].B.N, prog[1].A.N, prog[1].B.N)
	}
	for _, r := range []struct{ name, src, want string }{
		{"a target of any creature", spell("Creature", "Pump", "NumAtt$ +3 | NumDef$ +2", "NumAtt$ +1 | NumDef$ +1"), "target filter is not the PumpAll filter"},
		{"a sweep larger than the pump", spell("Creature.YouCtrl", "Pump", "NumAtt$ +1 | NumDef$ +1", "NumAtt$ +2 | NumDef$ +1"), "exceeds the target's"},
		{"a counter first", spell("Creature.YouCtrl", "PutCounter", "CounterType$ P1P1 | CounterNum$ 2", "NumAtt$ +1 | NumDef$ +1"), "not the target's plain pump"},
		{"a keyword on the sweep", spell("Creature.YouCtrl", "Pump", "NumAtt$ +3 | NumDef$ +2", "NumAtt$ +1 | NumDef$ +1 | KW$ Flying"), ""},
	} {
		var all []string
		for _, rf := range pumpAllOtherLower(t, r.src).Refusals {
			all = append(all, rf.Reason)
		}
		if len(all) == 0 || !strings.Contains(strings.Join(all, "; "), r.want) {
			t.Errorf("%s: refusals %q, want one containing %q", r.name, all, r.want)
		}
	}
}
