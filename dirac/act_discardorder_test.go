package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// discardOrderScen is player 1's hand of a competing instant (two matching
// exile-instead replacements: Dryad Militant and Leyline of the Void) and a
// creature whose move does not compete (Leyline only).
func newDiscardOrderScen(t *testing.T) (s *State, k *prep, id map[string]CardID, instant, creature Obj) {
	t.Helper()
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Dryad Militant", Count: 1},
		{Name: "Leyline of the Void", Count: 1},
		{Name: "Burst Lightning", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 8},
	}, nil)
	s, k = sc.s, sc.k
	sc.perm(id["Dryad Militant"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	instant = sc.hand(id["Burst Lightning"], 1)
	creature = sc.hand(id["Grizzly Bears"], 1)
	if !s.graveMoveCompetes(k, instant) {
		t.Fatal("the instant's move does not compete: it is not the competing pick")
	}
	if s.graveMoveCompetes(k, creature) {
		t.Fatal("the creature's move competes: it is not the non-competing pick")
	}
	return s, k, id, instant, creature
}

// atDiscardOrderAsk checks the state at gorge's CR 616.1 ask, posed inside the
// first answered (competing) card's move: its parked discard is the head, and
// both answered cards are still in hand. It answers the ask and checks both
// cards are exiled and the queue drains.
func atDiscardOrderAsk(t *testing.T, s *State, k *prep, instant, creature Obj) {
	t.Helper()
	if s.Stalled() {
		t.Fatalf("the discard stalled: %s", StallName(s.StallReason()))
	}
	if s.ctl != ctlExileOrder || !s.parkedWaiting() || s.cold.parked[0].kind != parkDiscard || s.cold.parked[0].obj != instant {
		t.Fatalf("ctl %d parkedWaiting %v head %d/%d, want the instant's parked discard ask", s.ctl, s.parkedWaiting(), s.cold.parked[0].kind, s.cold.parked[0].obj)
	}
	if z := s.objs[instant].zone; z != kop.ZoneHand {
		t.Fatalf("the asked instant is in zone %d, want the hand", z)
	}
	if z := s.objs[creature].zone; z != kop.ZoneHand {
		t.Fatalf("the later pick is in zone %d at the ask, want the hand", z)
	}
	var d Decision
	s.exileOrderOptions(k, &d)
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
		t.Fatalf("decision kind %d sub %d player %d n %d, want DkChoose/ChGenReplOrder for its owner over two sources", d.Kind, d.Sub, d.Player, d.N)
	}
	s.applyExileOrder(k, d.Options[0])
	for _, o := range []Obj{instant, creature} {
		if z := s.objs[o].zone; z != kop.ZoneExile {
			t.Fatalf("object %d is in zone %d after the answer, want exile", o, z)
		}
	}
	if s.parkedWaiting() || s.parkedDiscardWaiting() || s.Stalled() {
		t.Fatalf("after the answer: parkedWaiting %v parkedDiscard %v stalled %v", s.parkedWaiting(), s.parkedDiscardWaiting(), s.Stalled())
	}
}

// A targeted discard of two cards, the competing one picked first: gorge asks
// the order inside its move with the later pick still in hand.
func TestTargetedDiscardParksLaterPicksExileOrder(t *testing.T) {
	s, k, id, instant, creature := newDiscardOrderScen(t)
	s.ctl = ctlResolve
	s.res = resCtx{
		e:      stackEntry{card: id["Seeker's Folly"], ab: 0, ctrl: 0},
		ask:    raDiscard,
		askP:   1,
		chosen: 1,
		left:   2,
	}
	s.applyResolve(k, MakeAction(AkChoose, TCode(instant), 0, 0))
	if s.res.peekI != 1 || s.res.left != 1 {
		t.Fatalf("peekI %d left %d after the first pick, want 1 and 1", s.res.peekI, s.res.left)
	}
	s.applyResolve(k, MakeAction(AkChoose, TCode(creature), 0, 0))
	atDiscardOrderAsk(t, s, k, instant, creature)
}

// The cleanup discard to hand size (CR 514.1) takes its picks one decision at a
// time; a pick arriving after a parked competing one waits behind it.
func TestCleanupDiscardParksLaterPicksExileOrder(t *testing.T) {
	s, k, _, instant, creature := newDiscardOrderScen(t)
	s.ctl = ctlCleanupDiscard
	s.discardLeft = 2
	s.Apply(MakeAction(AkChoose, TCode(instant), 0, 0))
	if !s.parkedDiscardWaiting() || s.discardLeft != 1 || s.ctl != ctlCleanupDiscard {
		t.Fatalf("after the first pick: parked %v left %d ctl %d, want its parked discard and one more pick", s.parkedDiscardWaiting(), s.discardLeft, s.ctl)
	}
	s.Apply(MakeAction(AkChoose, TCode(creature), 0, 0))
	atDiscardOrderAsk(t, s, k, instant, creature)
	if s.ctl == ctlCleanupDiscard || s.ctl == ctlExileOrder {
		t.Fatalf("ctl %d after the last answer, want the cleanup finished", s.ctl)
	}
}

// A held discard (the first player's picks, kept until the last player has
// chosen) flushes in answer order, the ask inside the first competing card's move.
func TestHeldDiscardParksLaterPicksExileOrder(t *testing.T) {
	s, k, _, instant, creature := newDiscardOrderScen(t)
	if len(s.cold.discHeld) == 0 {
		s.cold.discHeld = make([]Obj, discHeldLen) // the scenario table has no Discard op
	}
	s.holdDiscard([]Obj{instant, creature})
	s.flushHeldDiscard(k)
	if !s.parkedWaiting() {
		t.Fatal("no parked discard after the flush")
	}
	s.ctl = ctlExileOrder
	atDiscardOrderAsk(t, s, k, instant, creature)
}
