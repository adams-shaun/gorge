package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A surveil's rest card moving to the graveyard under two any-zone
// exile-instead replacements is parked for its CR 616.1 order ask, stays in
// the library (off its list) while the ask is posed, and goes to exile on the
// answer. Before the park it moved at once and the kernel stopped at
// StallReplLeave.
func TestSurveilRestParksExileOrder(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Chart a Course", Count: 1},
		{Name: "Plains", Count: 4},
	}, nil)
	s, k := sc.s, sc.k
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	pl := &s.pl[1]
	o := pl.lib[pl.libLo] // the surveilled top card
	lo := pl.libLo
	s.ctl = ctlResolve
	s.res = resCtx{
		e:      stackEntry{card: id["Chart a Course"], ab: 0, ctrl: 1},
		active: true,
		ask:    raSurveil,
		askP:   1,
	}
	s.res.peek[0], s.res.peekN = o, 1
	s.applyResolve(k, MakeAction(AkDone, 0, 0, 0))

	if s.Stalled() {
		t.Fatalf("the surveil rest move stalled: %s", StallName(s.StallReason()))
	}
	if s.ctl != ctlExileOrder || !s.parkedWaiting() || s.cold.parked[0].kind != parkRest {
		t.Fatalf("ctl %d parkedWaiting %v kind %d, want the parked rest ask", s.ctl, s.parkedWaiting(), s.cold.parked[0].kind)
	}
	if z := s.objs[o].zone; z != kop.ZoneLibrary {
		t.Fatalf("the asked card is in zone %d, want the library", z)
	}
	if pl.libLo != lo+1 || pl.lib[pl.libLo] == o {
		t.Fatalf("the asked card is still in the library list (libLo %d, want %d)", pl.libLo, lo+1)
	}
	var d Decision
	s.exileOrderOptions(k, &d)
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
		t.Fatalf("decision kind %d sub %d player %d n %d, want DkChoose/ChGenReplOrder for its owner over two sources", d.Kind, d.Sub, d.Player, d.N)
	}
	s.applyExileOrder(k, d.Options[0])
	if z := s.objs[o].zone; z != kop.ZoneExile {
		t.Fatalf("the answered card is in zone %d, want exile", z)
	}
	if s.parkedWaiting() || s.Stalled() {
		t.Fatalf("after the answer: parkedWaiting %v stalled %v", s.parkedWaiting(), s.Stalled())
	}
}
