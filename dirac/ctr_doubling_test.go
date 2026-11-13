package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestZimoneDoublesPast255: Zimone's {G}{U},{T} ability doubles every
// counter kind; 141 +1/+1 counters double to 282, so her projected P/T is
// 283/286 (gorge's counters are int32). Before the 16-bit +1/+1 storage,
// OpDoubleCounters clamped the count at 255 (pair 1179).
func TestZimoneDoublesPast255(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{
		{Name: "Zimone, Paradox Sculptor", Count: 1},
		{Name: "Island", Count: 1},
		{Name: "Forest", Count: 1},
	})
	z := sc.perm(id["Zimone, Paradox Sculptor"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Forest"], 0)
	p := &sc.s.perms[sc.slot(z)]
	p.addCtrN(sc.s, int(kop.CtrP1P1), 141)

	d := sc.s.Step()
	activated := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(z)) {
			sc.s.Apply(a)
			activated = true
			break
		}
	}
	if !activated {
		t.Fatalf("Zimone's ability not offered: kind %d opts %v", d.Kind, d.Options[:d.N])
	}
	for i := 0; i < 20; i++ {
		d = sc.s.Step()
		if d.Kind == DkNone || d.Kind == DkPriority {
			break
		}
		if d.Kind != DkTarget {
			sc.s.Apply(d.Options[0])
			continue
		}
		picked := false
		for j := 0; j < int(d.N); j++ {
			if d.Options[j].Target() == TCode(uint8(z)) {
				sc.s.Apply(d.Options[j])
				picked = true
				break
			}
		}
		if picked {
			continue
		}
		for j := 0; j < int(d.N); j++ {
			if a := d.Options[j]; a.Kind() == AkDone || a.Target() == NoTarget {
				sc.s.Apply(a)
				picked = true
				break
			}
		}
		if !picked {
			t.Fatalf("no target/done option: %v", d.Options[:d.N])
		}
	}
	pass := func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	}
	sc.stepTo(t, 20, func() bool { return sc.s.sp == 0 && sc.s.ctl == ctlPriority }, pass)
	if n := sc.s.perms[sc.slot(z)].ctrLow(int(kop.CtrP1P1)); n != 282 {
		t.Fatalf("+1/+1 count %d, want 282", n)
	}
	pw, tg := sc.s.pt(sc.k, sc.slot(z))
	if pw != 283 || tg != 286 {
		t.Fatalf("P/T %d/%d, want 283/286", pw, tg)
	}
	var pr Projection
	sc.s.Project(&pr)
	if o := pr.Objs[z]; o.Power != 283 || o.Tough != 286 || o.Counters[kop.CtrP1P1] != 282 {
		t.Fatalf("projected %d/%d with %d counters, want 283/286 with 282", o.Power, o.Tough, o.Counters[kop.CtrP1P1])
	}
}
