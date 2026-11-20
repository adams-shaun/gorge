package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHiddenLandEntryOrderAsk: a land played off a Hideaway land that enters
// under both its own enters-tapped line and a carrier's "lands you control
// enter untapped" is asked the CR 616.1 order with the card still in exile,
// after the pick (gorge poses it on the move, before it emits the card); the
// answer plays the land, the last body applied decides the tapped bit, and
// the land play is counted with the move (gorge counts it after the ask,
// where playLandNow's route counts before). An unsupported competition still
// stalls loud (StallEntryOrder), and an as-enters choice is asked before the
// order ask.
func TestHiddenLandEntryOrderAsk(t *testing.T) {
	t.Run("ask_before_move_both_orders", func(t *testing.T) {
		for _, tapLast := range []bool{false, true} {
			e := newEntFix(t)
			sc := e.sc
			c := sc.perm(e.untapC, 0)
			l := sc.obj(e.tapLand, 0)
			sc.s.place(l, kop.ZoneExile, false)
			sc.s.res.e.ctrl = 0
			// a pick offer with a card left: the tail keeps it open instead
			// of closing the resolution
			sc.s.g.free[0] = l
			sc.s.g.nfree = 1
			if !sc.s.hideLand(sc.k, l) {
				t.Fatal("the play did not park on the order ask")
			}
			sc.s.ctl = ctlResolve // the resolution is suspended on the ask, as the real flow leaves it
			if sc.s.objs[l].zone != kop.ZoneExile {
				t.Fatalf("zone %d, want the card still in exile", sc.s.objs[l].zone)
			}
			if sc.s.pl[0].landsPlayed != 0 {
				t.Fatalf("%d land plays at the ask, want 0 (gorge counts it at the move)", sc.s.pl[0].landsPlayed)
			}
			d := e.ask(t)
			if d.Player != 0 || d.N != 2 {
				t.Fatalf("player %d options %d, want player 0 with both sources", d.Player, d.N)
			}
			// the card is in exile, not the hand: its own line is scanned
			// after the battlefield's bodies
			if got := srcs(d); got[0] != c || got[1] != l {
				t.Fatalf("options %v, want the carrier then the land (scan order) [%d %d]", got, c, l)
			}
			first := l // tap first: the untap applies last, the land enters untapped
			if tapLast {
				first = c
			}
			sc.s.hideEntryPick(sc.k, pick(d, first))
			if !sc.onBF(l) || e.tapped(l) != tapLast {
				t.Fatalf("tapLast=%v: on battlefield %v tapped %v", tapLast, sc.onBF(l), e.tapped(l))
			}
			if sc.s.pl[0].landsPlayed != 1 {
				t.Fatalf("%d land plays after the answer, want 1", sc.s.pl[0].landsPlayed)
			}
			if sc.s.res.ask != raGenCastFree || sc.s.g.nfree != 1 || sc.s.g.stalled != 0 {
				t.Fatalf("after the answer: ask %d nfree %d stalled %d, want the pick offer open", sc.s.res.ask, sc.s.g.nfree, sc.s.g.stalled)
			}
			if sc.s.g.ro.n != 0 || sc.s.g.ro.pcol != 0 {
				t.Fatalf("ro %+v, want the answered order consumed", sc.s.g.ro.replHdr)
			}
		}
	})
	t.Run("own_choice_is_asked_first", func(t *testing.T) {
		e := newEntFix(t)
		sc := e.sc
		c := sc.perm(e.untapC, 0)
		l := sc.obj(e.choiceLand, 0)
		sc.s.place(l, kop.ZoneExile, false)
		sc.s.res.e.ctrl = 0
		sc.s.g.free[0] = l
		sc.s.g.nfree = 1
		if !sc.s.hideLandPark(sc.k, l) {
			t.Fatal("the as-enters choice was not asked in exile")
		}
		sc.s.ctl = ctlResolve // the resolution is suspended on the choice ask, as the real flow leaves it
		if sc.s.objs[l].zone != kop.ZoneExile || sc.s.g.pre.obj != l {
			t.Fatalf("zone %d pre %+v, want the card in exile with its choice parked", sc.s.objs[l].zone, sc.s.g.pre)
		}
		sc.s.hideLandAnswer(sc.k)
		if sc.s.objs[l].zone != kop.ZoneExile || sc.s.g.pre.obj != l {
			t.Fatalf("zone %d pre %+v, want the choice kept and the order ask parked", sc.s.objs[l].zone, sc.s.g.pre)
		}
		d := e.ask(t)
		// the choice land also enters tapped, and its answered choice joins
		// the ask as a record candidate: carrier, tap, record (scan order)
		if d.Player != 0 || d.N != 3 {
			t.Fatalf("player %d options %d, want the order ask after the choice", d.Player, d.N)
		}
		sc.s.hideEntryPick(sc.k, pick(d, c)) // the land's tap applies last
		if !sc.onBF(l) || !e.tapped(l) {
			t.Fatalf("on battlefield %v tapped %v, want the tapped line applied last", sc.onBF(l), e.tapped(l))
		}
		if sc.s.pl[0].landsPlayed != 1 || sc.s.g.pre.state != 0 || sc.s.g.stalled != 0 {
			t.Fatalf("landsPlayed %d pre %+v stalled %d", sc.s.pl[0].landsPlayed, sc.s.g.pre, sc.s.g.stalled)
		}
	})
	t.Run("counters_competition_stalls_loud", func(t *testing.T) {
		e := newEntFix(t)
		sc := e.sc
		sc.perm(e.untapC, 0)
		l := sc.obj(e.ctrLand, 0)
		sc.s.place(l, kop.ZoneExile, false)
		sc.s.res.e.ctrl = 0
		if sc.s.hideLand(sc.k, l) {
			t.Fatal("the play parked on an ask it cannot pose")
		}
		if sc.s.g.stalled != StallEntryOrder {
			t.Fatalf("stalled %d, want StallEntryOrder", sc.s.g.stalled)
		}
	})
}
