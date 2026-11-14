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
	t.Run("one rest card", func(t *testing.T) {
		sc := newSurveilRestScen(t, 1)
		s, k, pl, o := sc.s, sc.k, &sc.s.pl[1], sc.rest[0]
		if s.Stalled() {
			t.Fatalf("the surveil rest move stalled: %s", StallName(s.StallReason()))
		}
		if s.ctl != ctlExileOrder || !s.parkedWaiting() || s.cold.parked[0].kind != parkRest {
			t.Fatalf("ctl %d parkedWaiting %v kind %d, want the parked rest ask", s.ctl, s.parkedWaiting(), s.cold.parked[0].kind)
		}
		if z := s.objs[o].zone; z != kop.ZoneLibrary {
			t.Fatalf("the asked card is in zone %d, want the library", z)
		}
		if pl.libLo != sc.lo+1 || pl.lib[pl.libLo] == o {
			t.Fatalf("the asked card is still in the library list (libLo %d, want %d)", pl.libLo, sc.lo+1)
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
		if pl.lib[pl.libLo] != sc.kept {
			t.Fatalf("the kept card is not back on top of the library (libLo %d)", pl.libLo)
		}
		if s.parkedWaiting() || s.Stalled() {
			t.Fatalf("after the answer: parkedWaiting %v stalled %v", s.parkedWaiting(), s.Stalled())
		}
	})

	// With two rest cards the first answer only settles its own move: the
	// second card gets its own ask and moves on its own answer.
	t.Run("two rest cards ask one at a time", func(t *testing.T) {
		sc := newSurveilRestScen(t, 2)
		s, k, pl, rest := sc.s, sc.k, &sc.s.pl[1], sc.rest
		if s.Stalled() {
			t.Fatalf("the surveil rest move stalled: %s", StallName(s.StallReason()))
		}
		if s.ctl != ctlExileOrder || !s.parkedWaiting() || s.cold.parked[0].kind != parkRest {
			t.Fatalf("ctl %d parkedWaiting %v kind %d, want the parked rest ask", s.ctl, s.parkedWaiting(), s.cold.parked[0].kind)
		}
		if pl.libLo != sc.lo+2 {
			t.Fatalf("the rest cards are still in the library list (libLo %d, want %d)", pl.libLo, sc.lo+2)
		}
		for i, o := range rest {
			if z := s.objs[o].zone; z != kop.ZoneLibrary {
				t.Fatalf("rest card %d is in zone %d, want the library", i, z)
			}
			if s.cold.parked[0].obj != o {
				t.Fatalf("rest card %d is not the one asked (asked %d)", i, s.cold.parked[0].obj)
			}
			var d Decision
			s.exileOrderOptions(k, &d)
			if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
				t.Fatalf("rest card %d: decision kind %d sub %d player %d n %d, want DkChoose/ChGenReplOrder for its owner over two sources", i, d.Kind, d.Sub, d.Player, d.N)
			}
			s.applyExileOrder(k, d.Options[0])
			if z := s.objs[o].zone; z != kop.ZoneExile {
				t.Fatalf("rest card %d is in zone %d after its answer, want exile", i, z)
			}
			if i == 0 && !s.parkedWaiting() {
				t.Fatal("the second rest card lost its ask after the first answer")
			}
		}
		if pl.lib[pl.libLo] != sc.kept {
			t.Fatalf("the kept card is not back on top of the library (libLo %d)", pl.libLo)
		}
		if s.parkedWaiting() || s.Stalled() {
			t.Fatalf("after the answers: parkedWaiting %v stalled %v", s.parkedWaiting(), s.Stalled())
		}
	})

	// The queue's later cards are re-checked after an earlier answer
	// (massMovesGoOn): a rest card whose second source has left moves then
	// and there, with no second ask.
	t.Run("a rest card with one source left moves without an ask", func(t *testing.T) {
		sc := newSurveilRestScen(t, 2)
		s, k := sc.s, sc.k
		if !s.parkedWaiting() || s.cold.parked[0].obj != sc.rest[0] {
			t.Fatalf("ctl %d parkedWaiting %v, want the first rest card's ask", s.ctl, s.parkedWaiting())
		}
		s.leave(k, s.objs[sc.leys[1]].slot, kop.ZoneGraveyard, false)
		if z := s.objs[sc.leys[1]].zone; z != kop.ZoneGraveyard {
			t.Fatalf("the second Leyline is in zone %d, want the graveyard", z)
		}
		var d Decision
		s.exileOrderOptions(k, &d)
		if d.N != 2 { // the ask still offers the sources that competed when it parked
			t.Fatalf("the first ask offers %d sources, want the two that parked it", d.N)
		}
		s.applyExileOrder(k, d.Options[0])
		if z := s.objs[sc.rest[0]].zone; z != kop.ZoneExile {
			t.Fatalf("the answered card is in zone %d, want exile", z)
		}
		if s.parkedWaiting() {
			t.Fatalf("the second rest card is still waiting for an ask with one source left (kind %d)", s.cold.parked[0].kind)
		}
		if z := s.objs[sc.rest[1]].zone; z != kop.ZoneExile {
			t.Fatalf("the second rest card is in zone %d, want exile", z)
		}
		if s.Stalled() {
			t.Fatalf("the surveil rest move stalled: %s", StallName(s.StallReason()))
		}
	})
}

// surveilRestScen is a p1 surveil of a Chart a Course over one kept card and
// the given number of rest cards, with two Leylines of the Void controlled by
// p0 exiling anything put into p1's graveyard.
type surveilRestScen struct {
	s    *State
	k    *prep
	kept Obj
	rest []Obj
	leys []Obj
	lo   uint8
}

// newSurveilRestScen applies the surveil's arrange: the kept card, then Done
// sends the rest to the graveyard.
func newSurveilRestScen(t *testing.T, rests int) surveilRestScen {
	t.Helper()
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Chart a Course", Count: 1},
		{Name: "Plains", Count: 4},
	}, nil)
	s, k := sc.s, sc.k
	leys := []Obj{sc.perm(id["Leyline of the Void"], 0), sc.perm(id["Leyline of the Void"], 0)}
	pl := &s.pl[1]
	lo := pl.libLo
	peek := make([]Obj, rests+1)
	for i := range peek {
		peek[i] = pl.lib[int(lo)+i]
	}
	s.ctl = ctlResolve
	s.res = resCtx{
		e:      stackEntry{card: id["Chart a Course"], ab: 0, ctrl: 1},
		active: true,
		ask:    raSurveil,
		askP:   1,
	}
	copy(s.res.peek[:], peek)
	s.res.peekN = uint8(len(peek))
	// keep the first card, then Done sends the rest to the graveyard
	s.applyResolve(k, MakeAction(AkChoose, TCode(peek[0]), 0, 0))
	if got := s.res.peekI; got != 1 {
		t.Fatalf("peekI %d after the kept pick, want 1", got)
	}
	s.applyResolve(k, MakeAction(AkDone, 0, 0, 0))
	return surveilRestScen{s: s, k: k, kept: peek[0], rest: peek[1:], leys: leys, lo: lo}
}

// An optional discard of two cards, one of whose moves competes for two
// exile-instead replacements and one does not: gorge asks the order inside the
// competing card's move, so every later answered card is still in hand at the
// ask and moves on the answer. Parking only the competing picks moved a later
// one early.
func TestOptDiscardParksLaterPicksExileOrder(t *testing.T) {
	run := func(t *testing.T, competingFirst bool) {
		sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
			{Name: "Dryad Militant", Count: 1},
			{Name: "Leyline of the Void", Count: 1},
			{Name: "Burst Lightning", Count: 1},
			{Name: "Grizzly Bears", Count: 1},
			{Name: "Plains", Count: 8},
		}, nil)
		s, k := sc.s, sc.k
		sc.perm(id["Dryad Militant"], 0)
		sc.perm(id["Leyline of the Void"], 0)
		instant := sc.hand(id["Burst Lightning"], 1)
		creature := sc.hand(id["Grizzly Bears"], 1)
		if !s.graveMoveCompetes(k, instant) {
			t.Fatal("the instant's move does not compete: it is not the competing pick")
		}
		if s.graveMoveCompetes(k, creature) {
			t.Fatal("the creature's move competes: it is not the non-competing pick")
		}
		s.ctl = ctlResolve
		s.res = resCtx{
			e:      stackEntry{card: id["Seeker's Folly"], ab: 0, ctrl: 0},
			ask:    raOptDisc,
			askP:   1,
			chosen: 1,
		}
		if competingFirst {
			s.res.arr[0], s.res.arr[1] = instant, creature
		} else {
			s.res.arr[0], s.res.arr[1] = creature, instant
		}
		s.res.peekI = 2
		s.applyResolve(k, MakeAction(AkDone, 0, 0, 0))
		if s.Stalled() {
			t.Fatalf("the optional discard stalled: %s", StallName(s.StallReason()))
		}
		if s.ctl != ctlExileOrder || !s.parkedWaiting() || s.cold.parked[0].kind != parkDiscard || s.cold.parked[0].obj != instant {
			t.Fatalf("ctl %d parkedWaiting %v head %d/%d, want the instant's parked discard ask", s.ctl, s.parkedWaiting(), s.cold.parked[0].kind, s.cold.parked[0].obj)
		}
		if z := s.objs[instant].zone; z != kop.ZoneHand {
			t.Fatalf("the asked instant is in zone %d, want the hand", z)
		}
		wantCreature := kop.ZoneExile // the earlier non-competing pick moved before the ask
		if competingFirst {
			wantCreature = kop.ZoneHand // the later pick waits for the ask
		}
		if z := s.objs[creature].zone; z != wantCreature {
			t.Fatalf("the creature is in zone %d at the ask, want %d", z, wantCreature)
		}
		var d Decision
		s.exileOrderOptions(k, &d)
		if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
			t.Fatalf("decision kind %d sub %d player %d n %d, want DkChoose/ChGenReplOrder for its owner over two sources", d.Kind, d.Sub, d.Player, d.N)
		}
		s.applyExileOrder(k, d.Options[0])
		if z := s.objs[instant].zone; z != kop.ZoneExile {
			t.Fatalf("the answered instant is in zone %d, want exile", z)
		}
		if z := s.objs[creature].zone; z != kop.ZoneExile {
			t.Fatalf("the creature is in zone %d after the answer, want exile", z)
		}
		if s.parkedWaiting() || s.Stalled() {
			t.Fatalf("after the answer: parkedWaiting %v stalled %v", s.parkedWaiting(), s.Stalled())
		}
	}
	t.Run("competing pick first", func(t *testing.T) { run(t, true) })
	t.Run("competing pick second", func(t *testing.T) { run(t, false) })
}
