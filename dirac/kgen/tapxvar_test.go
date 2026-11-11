package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapXVarTriggerCost: a trigger's own tapXType<X/Spec> cost (a Myr
// Battlesphere-shaped attack trigger) lowers to an IfPaid PayTapX carrying
// PayBSubTapXVar, its spec the filter; every other home of the X form (an
// activated ability, a cost beside mana, an SVar:X that is not the plain
// Count$xPaid) refuses by name.
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

	for _, p := range []struct{ name, src, want string }{
		{"activated", head + "A:AB$ Pump | Cost$ T tapXType<X/Myr> | Defined$ Self | NumAtt$ +X\nSVar:X:Count$xPaid\n", "outside a printed trigger's own cost"},
		{"beside mana", trig("2 tapXType<X/Myr>", "Count$xPaid"), "resolution tapXType<X> cost shape"},
		{"other X", trig("tapXType<X/Myr>", "Count$CardPower"), "other than Count$xPaid"},
	} {
		_, rs := kwardpowBuild(t, "Tap Probe", p.src)
		if all := kpaylifexReasons(rs); !strings.Contains(all, p.want) {
			t.Errorf("%s: refusals %q, want %q", p.name, all, p.want)
		}
	}
}
