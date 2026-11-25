package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapXVarTriggerCost: a trigger's own tapXType<X/Spec> cost (a Myr
// Battlesphere-shaped attack trigger) lowers to an IfPaid PayTapX carrying
// PayBSubTapXVar, its spec the filter; a printed activated ability's lowers
// to the announced CostTapX with N 0 (cost_tapxann.go); every other home of
// the X form (an activated ability beside a printed {X}, a trigger's cost
// beside mana, an SVar:X that is not the plain Count$xPaid) refuses by name.
func TestTapXVarTriggerCost(t *testing.T) {
	const head = "Name:Tap Probe\nManaCost:4\nTypes:Artifact Creature Myr\nPT:2/2\n"
	trig := func(cost, x string) string {
		return head + "T:Mode$ Attacks | ValidCard$ Card.Self | OptionalDecider$ You | Execute$ TrigPump\n" +
			"SVar:TrigPump:AB$ Pump | Cost$ " + cost + " | Defined$ Self | NumAtt$ +X\nSVar:X:" + x + "\n"
	}
	d, rs := kwardpowBuild(t, "Tap Probe", trig("tapXType<X/Myr>", "Count$xPaid"))
	if len(rs) != 0 {
		t.Fatalf("refused: %v", rs)
	}
	var got *kop.Instr
	for _, a := range d.Abilities {
		for i := range a.Prog {
			if a.Prog[i].Op == kop.OpIfPaid {
				got = &a.Prog[i]
			}
		}
	}
	if got == nil {
		t.Fatal("no OpIfPaid in the trigger's program")
	}
	if got.C&0xf != kop.PayTapX || got.B.N != kop.PayBSubTapXVar || got.F == 0 {
		t.Errorf("IfPaid kind %d B %d F %d, want PayTapX / PayBSubTapXVar / a spec", got.C&0xf, got.B.N, got.F)
	}

	d, rs = kwardpowBuild(t, "Tap Probe", head+"A:AB$ Pump | Cost$ T tapXType<X/Myr> | Defined$ Self | NumAtt$ +X\nSVar:X:Count$xPaid\n")
	if len(rs) != 0 {
		t.Fatalf("activated: refused: %v", rs)
	}
	if c := d.Abilities[len(d.Abilities)-1].Cost; c.Flags&kop.CostTapX == 0 || c.N != 0 || c.Filter == 0 {
		t.Errorf("activated: cost flags %#x N %d F %d, want the announced CostTapX (N 0) with the spec", c.Flags, c.N, c.Filter)
	}

	for _, p := range []struct{ name, src, want string }{
		{"activated beside {X}", head + "A:AB$ Pump | Cost$ X T tapXType<X/Myr> | Defined$ Self | NumAtt$ +X\nSVar:X:Count$xPaid\n", "beside a printed {X}"},
		{"beside mana", trig("2 tapXType<X/Myr>", "Count$xPaid"), "resolution tapXType<X> cost shape"},
		{"other X", trig("tapXType<X/Myr>", "Count$CardPower"), "other than Count$xPaid"},
	} {
		_, rs := kwardpowBuild(t, "Tap Probe", p.src)
		if all := kpaylifexReasons(rs); !strings.Contains(all, p.want) {
			t.Errorf("%s: refusals %q, want %q", p.name, all, p.want)
		}
	}
}
