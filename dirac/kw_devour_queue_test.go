package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDevourTwoQueuedAsks: two Famished Worldsires enter while both of their
// Devour asks are queued. gorge settles each entry's Devour completely before
// the next, so the re-posed pick stays at the queue's head: the first
// Worldsire gets three counters per land it picked, the second three per land
// it picked, and neither sees the other's picks.
func TestDevourTwoQueuedAsks(t *testing.T) {
	sc, id := newSetScenTweak(t, "hosted-cmdr", []cardset.Entry{
		{Name: "Famished Worldsire", Count: 2},
		{Name: "Plains", Count: 8},
	}, nil)
	s := sc.s
	for i := 0; i < 4; i++ {
		sc.perm(id["Plains"], 0)
	}
	a := sc.perm(id["Famished Worldsire"], 0)
	b := sc.perm(id["Famished Worldsire"], 0)
	if s.g.nask != 2 || s.g.ask[0].kind != akDevour || s.g.ask[1].kind != akDevour ||
		s.g.ask[0].sl != sc.slot(a) || s.g.ask[1].sl != sc.slot(b) {
		t.Fatalf("queued asks %d (%+v), want the two Devour asks, the first Worldsire's first", s.g.nask, s.g.ask[:s.g.nask])
	}
	// the first Worldsire picks one land, the second two; each answer is
	// asked of the Worldsire whose ask heads the queue
	steps := []struct {
		who  Obj
		want ActionKind
	}{{a, AkChoose}, {a, AkDone}, {b, AkChoose}, {b, AkChoose}, {b, AkDone}}
	for i, st := range steps {
		want := st.want
		if s.g.nask == 0 || s.g.ask[0].sl != sc.slot(st.who) {
			t.Fatalf("answer %d: the head ask (%d queued) is not the expected Worldsire's", i, s.g.nask)
		}
		d := s.Step()
		if d.Sub != ChSacrifice {
			t.Fatalf("answer %d: decision sub %d, want the Devour pick", i, d.Sub)
		}
		found := false
		for j := 0; j < int(d.N); j++ {
			if d.Options[j].Kind() == want {
				s.Apply(d.Options[j])
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("answer %d: option kind %d not offered: %v", i, want, d.Options[:d.N])
		}
	}
	if s.Stalled() {
		t.Fatalf("stalled: %s", StallName(s.StallReason()))
	}
	if s.g.nask != 0 {
		t.Fatalf("%d asks still queued", s.g.nask)
	}
	if n := s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)); n != 3 {
		t.Errorf("the first Worldsire has %d +1/+1 counters, want 3 (one land)", n)
	}
	if n := s.perms[sc.slot(b)].ctrLow(int(kop.CtrP1P1)); n != 6 {
		t.Errorf("the second Worldsire has %d +1/+1 counters, want 6 (two lands)", n)
	}
}
