package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A TwoPiles unchosen pile moving to the graveyard under two any-zone
// exile-instead replacements is parked card by card for its CR 616.1 order ask
// (Fact or Fiction beside two Rest in Peace; gorge's ChangeZone Defined$
// Remembered body asks the cards' owner before each move). Before the park the
// first card's move stalled at StallReplLeave.
func TestTwoPilesUnchosenPileParksExileOrder(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Plains", Count: 4},
	}, nil)
	s, k := sc.s, sc.k
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	pl := &s.pl[1]
	top := [3]Obj{pl.lib[pl.libLo], pl.lib[pl.libLo+1], pl.lib[pl.libLo+2]}
	for _, o := range top {
		if !s.graveMoveCompetes(k, o) {
			t.Fatal("precondition: a revealed card's graveyard move is not contested by two exile-instead sources")
		}
	}
	prog := []kop.Instr{
		{Op: kop.OpPeek, Obj: kop.RefController, A: kop.Const(3), C: int16(kop.PeekPublic | kop.PeekRemember)},
		{Op: kop.OpTwoPiles, Obj: kop.RefController, B: kop.Const(kop.TpOppSeparates | kop.TpUnchosenPileOrder), C: int16(kop.ZoneHand)},
	}
	src := CardID(len(k.t.Cards))
	k.t.Cards = append(k.t.Cards, kop.CardDef{Name: "Test Piles", Types: kop.TInstant,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	s.res = resCtx{e: stackEntry{card: src, ab: 0, ctrl: 1}, active: true}
	s.ctl = ctlResolve
	s.sp = 1
	s.exec(k, &prog[0])
	s.res.pc = 1
	s.exec(k, &prog[1])
	if s.res.ask != raGenPileSplit || s.g.nrem != 3 {
		t.Fatalf("ask %d nrem %d, want the pile split over three cards", s.res.ask, s.g.nrem)
	}
	pick := func(match func(a Action) bool) {
		t.Helper()
		var d Decision
		s.resolveOptionsGen(k, &d)
		for i := 0; i < int(d.N); i++ {
			if match(d.Options[i]) {
				s.applyResolveGen(k, d.Options[i])
				return
			}
		}
		t.Fatalf("no matching option among %d at ask %d", d.N, s.res.ask)
	}
	for _, o := range top[:2] { // pile A: the first two cards
		pick(func(a Action) bool { return a.Kind() == AkChoose && Obj(a.Source()) == o })
	}
	pick(func(a Action) bool { return a.Kind() == AkDone })
	if s.res.ask != raGenPileChoose {
		t.Fatalf("ask %d, want the chooser's pick", s.res.ask)
	}
	pick(func(a Action) bool { return a.Mode() == 1 }) // the face-up pile (the third card) to hand

	if s.Stalled() {
		t.Fatalf("the unchosen pile's move stalled: %s", StallName(s.StallReason()))
	}
	if z := s.objs[top[2]].zone; z != kop.ZoneHand {
		t.Fatalf("the chosen pile's card is in zone %d, want hand", z)
	}
	for _, o := range top[:2] {
		if z := s.objs[o].zone; z != kop.ZoneLibrary {
			t.Fatalf("an unchosen card is in zone %d before its answer, want the library", z)
		}
	}
	for n := 0; n < 2; n++ {
		if s.ctl != ctlExileOrder || !s.parkedWaiting() {
			t.Fatalf("ask %d: ctl %d parkedWaiting %v, want the parked order ask", n, s.ctl, s.parkedWaiting())
		}
		var od Decision
		s.exileOrderOptions(k, &od)
		if od.Kind != DkChoose || od.Sub != ChGenReplOrder || od.Player != 1 || od.N != 2 {
			t.Fatalf("ask %d: decision kind %d sub %d player %d n %d, want DkChoose/ChGenReplOrder for the owner over two sources",
				n, od.Kind, od.Sub, od.Player, od.N)
		}
		if z := s.objs[top[1]].zone; n == 0 && z != kop.ZoneLibrary {
			t.Fatalf("ask %d: the later card moved before its own ask (zone %d)", n, z)
		}
		s.applyExileOrder(k, od.Options[0])
		if z := s.objs[top[n]].zone; z != kop.ZoneExile {
			t.Fatalf("ask %d: the answered card is in zone %d, want exile", n, z)
		}
	}
	if s.parkedWaiting() || s.Stalled() {
		t.Fatalf("after both answers: parkedWaiting %v stalled %v", s.parkedWaiting(), s.Stalled())
	}
	if s.res.active {
		t.Fatal("the resolution did not finish after the last answer")
	}
}
