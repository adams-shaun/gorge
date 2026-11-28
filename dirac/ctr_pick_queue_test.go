package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCtrPickTwoQueuedAsks: two Grimdancers enter while both of their
// two-kind counter picks are queued. gorge settles each entry's picks
// completely before the next entry's ask, so the re-posed second pick stays
// at the queue's head: the first Grimdancer gets both of its picks before the
// second Grimdancer's first.
func TestCtrPickTwoQueuedAsks(t *testing.T) {
	sc, id := newSetScenTweak(t, "mbc", []cardset.Entry{
		{Name: "Grimdancer", Count: 2},
		{Name: "Plains", Count: 8},
	}, nil)
	s := sc.s
	if _, ok := id["Grimdancer"]; !ok {
		t.Fatal("precondition: Grimdancer is not in the mbc table")
	}
	a := sc.perm(id["Grimdancer"], 0)
	b := sc.perm(id["Grimdancer"], 0)
	if s.g.nask != 2 || s.g.ask[0].kind != akCtrPick || s.g.ask[1].kind != akCtrPick ||
		s.g.ask[0].sl != sc.slot(a) || s.g.ask[1].sl != sc.slot(b) {
		t.Fatalf("queued asks %d (%+v), want the two counter picks, the first Grimdancer's first", s.g.nask, s.g.ask[:s.g.nask])
	}
	// four picks: the first Grimdancer's two, then the second's two. The
	// second pick of each is re-posed at the queue's head, not behind the
	// other Grimdancer's queued ask.
	steps := []struct {
		who  Obj
		kind uint8
		opts uint16
	}{
		{a, akCtrPick, 3},
		{a, akCtrPick2, 2},
		{b, akCtrPick, 3},
		{b, akCtrPick2, 2},
	}
	var picked [2][2]uint8
	for i, st := range steps {
		if s.g.nask == 0 || s.g.ask[0].sl != sc.slot(st.who) || s.g.ask[0].kind != st.kind {
			t.Fatalf("answer %d: head ask (%d queued) is not the expected Grimdancer's %d", i, s.g.nask, st.kind)
		}
		d := s.Step()
		if d.Sub != ChRemCtrKind {
			t.Fatalf("answer %d: decision sub %d, want the counter-kind pick", i, d.Sub)
		}
		if d.N != st.opts {
			t.Fatalf("answer %d: %d options, want %d", i, d.N, st.opts)
		}
		kind := d.Options[0].Mode()
		for j := 0; j < int(d.N); j++ {
			if d.Options[j].Kind() != AkChoose {
				t.Fatalf("answer %d: option %d kind %d, want a counter kind", i, j, d.Options[j].Kind())
			}
		}
		picked[i/2][i%2] = kind
		s.Apply(d.Options[0])
	}
	if s.Stalled() {
		t.Fatalf("stalled: %s", StallName(s.StallReason()))
	}
	if s.g.nask != 0 {
		t.Fatalf("%d asks still queued", s.g.nask)
	}
	for i, who := range []Obj{a, b} {
		sl := sc.slot(who)
		for _, kind := range picked[i] {
			if n := s.perms[sl].ctrLow(int(kop.CounterKind(kind))); n != 1 {
				t.Errorf("Grimdancer %d has %d counters of kind %d, want 1", i, n, kind)
			}
		}
	}
}
