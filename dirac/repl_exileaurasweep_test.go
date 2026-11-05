package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A creature dying under two exile-instead effects poses its CR 616.1 order
// ask inside its state-based check, and gorge's pass ends there. The Aura of
// a host that died earlier in the same check is still on the battlefield at
// the ask (the host's departure left it unattached, but no second pass ran);
// the check after the answer sweeps it. The kernel ran the next pass before
// the ask and swept the Aura early (std-dsk pair 768).
func TestReplLingeringAuraSweepsAfterOrderAsk(t *testing.T) {
	sc, id := newSetScenTweak(t, "rememberdestroyed", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Pacifism", Count: 1},
		{Name: "Savannah Lions", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 4},
	}, nil)
	s, k := sc.s, sc.k
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	host := sc.perm(id["Savannah Lions"], 0)
	aura := sc.perm(id["Pacifism"], 0)
	bears := sc.perm(id["Grizzly Bears"], 1)
	s.perms[sc.slot(aura)].attached = sc.slot(host)
	s.perms[sc.slot(host)].damage = 5
	s.perms[sc.slot(bears)].damage = 5
	if !s.graveMoveCompetes(k, bears) || s.graveMoveCompetes(k, host) {
		t.Fatal("the dying Bears must compete for the two Leylines and p0's own host not")
	}

	if !s.sba(k) {
		t.Fatal("the check did not pose the order ask")
	}
	if s.ctl != ctlExileOrder || !s.parkedWaiting() || s.cold.parked[0].obj != bears {
		t.Fatalf("ctl %d parkedWaiting %v, want the Bears' parked ask", s.ctl, s.parkedWaiting())
	}
	if z := s.objs[host].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the host that died ahead of the asked creature is in zone %d, want the graveyard", z)
	}
	if !sc.onBF(aura) {
		t.Fatal("the orphaned Aura was swept before the ask: gorge's pass ends at the ask")
	}

	var d Decision
	s.exileOrderOptions(k, &d)
	s.applyExileOrder(k, d.Options[0])
	if z := s.objs[bears].zone; z != kop.ZoneExile {
		t.Fatalf("the answered Bears are in zone %d, want exile", z)
	}
	if !sc.onBF(aura) {
		t.Fatal("the answer swept the Aura: that is the next check's work")
	}
	if s.sba(k) {
		t.Fatal("the check after the answer asked again")
	}
	if z := s.objs[aura].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the Aura is in zone %d after the next check, want the graveyard", z)
	}
	if s.Stalled() {
		t.Fatalf("stalled: %s", StallName(s.StallReason()))
	}
}
