package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The last player's picks of an each-player discard are a competing card's
// move like any other: gorge asks the CR 616.1 order inside it, so the pick
// waits for its ask where the kernel moved it at once and stalled
// (StallReplLeave; std-dsk pair 768).
func TestReplChainedDiscardParksCompetingPick(t *testing.T) {
	pc := -1
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Dryad Militant", Count: 1},
		{Name: "Leyline of the Void", Count: 1},
		{Name: "Seeker's Folly", Count: 1},
		{Name: "Burst Lightning", Count: 1},
		{Name: "Plains", Count: 8},
	}, func(tbl *kop.Table, id map[string]CardID) {
		prog := tbl.Cards[id["Seeker's Folly"]].Abilities[0].Prog
		for i := range prog {
			if prog[i].Op == kop.OpDiscard {
				prog[i].Obj = kop.RefEachPlayer // the discard instruction an each-player card runs
				pc = i
			}
		}
	})
	if pc < 0 {
		t.Fatal("Seeker's Folly has no discard instruction to chain")
	}
	s, k := sc.s, sc.k
	sc.perm(id["Dryad Militant"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	instant := sc.hand(id["Burst Lightning"], 1)
	if !s.graveMoveCompetes(k, instant) {
		t.Fatal("the instant's move does not compete")
	}
	s.g.repNext = 255
	s.ctl = ctlResolve
	s.res = resCtx{
		e:      stackEntry{card: id["Seeker's Folly"], ab: 0, ctrl: 0},
		pc:     uint8(pc),
		ask:    raDiscard,
		askP:   1,
		chosen: 1,
		left:   1,
	}
	if !s.discardChained(k, &s.prog(k)[s.res.pc]) {
		t.Fatal("the scenario's discard is not chained")
	}
	s.applyResolve(k, MakeAction(AkChoose, TCode(instant), 0, 0))
	if s.Stalled() {
		t.Fatalf("the chained discard stalled: %s", StallName(s.StallReason()))
	}
	if s.ctl != ctlExileOrder || !s.parkedWaiting() || s.cold.parked[0].kind != parkDiscard || s.cold.parked[0].obj != instant {
		t.Fatalf("ctl %d parkedWaiting %v, want the instant's parked discard ask", s.ctl, s.parkedWaiting())
	}
	if z := s.objs[instant].zone; z != kop.ZoneHand {
		t.Fatalf("the asked instant is in zone %d, want the hand", z)
	}
}
