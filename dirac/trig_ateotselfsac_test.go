package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAtEOTSelfSacrificeSkipsReturnedObject: gorge's AtEOT$ Sacrifice
// registered on the affected object itself (Creator == 0, no |CR=) is
// incarnation-tracked, like a generated delayed program: the trigger still
// goes on the stack, but its Self names the incarnation the registration saw,
// so a card that left and returned is not sacrificed (CR 603.7c, 400.7).
// Creator-sourced records (a reanimation spell, a CopyPermanent token, a sweep)
// name the affected object separately and still sacrifice whichever
// incarnation is there, so the check must not reach them.
func TestAtEOTSelfSacrificeSkipsReturnedObject(t *testing.T) {
	for _, tc := range []struct {
		name         string
		otherCreator bool
		wantSac      bool
	}{
		{"self registration skips the returned incarnation", false, false},
		{"creator-sourced registration still sacrifices", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deck := []cardset.Entry{{Name: "Plains", Count: 20}}
			set := cardsettest.Set(t, "fdn")
			tbl, _, _, err := set.Matchup(deck, deck)
			if err != nil {
				t.Fatal(err)
			}
			c := CardID(len(tbl.Cards))
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bauble", Types: kop.TArtifact})
			sc := newScenTable(t, tbl, Config{NoElide: true})
			s, k := sc.s, sc.k
			o := sc.perm(c, 0)
			creator := o
			if tc.otherCreator {
				creator = sc.perm(c, 0)
			}

			s.res.e.obj = creator
			s.sacAtEndUntracked(o, 0)
			if s.ndly != 1 || s.dly[0].obj != o || s.dly[0].when != dlyUntracked {
				t.Fatalf("delayed %d %+v, want one untracked sacrifice record on the object", s.ndly, s.dly[0])
			}
			v0 := s.objs[o].ver
			s.leave(k, sc.slot(o), kop.ZoneExile, false)
			s.enter(k, o, 0, false, kop.ZoneExile)
			if s.objs[o].zone != kop.ZoneBattlefield {
				t.Fatalf("precondition: the object is in zone %d after the blink, want the battlefield", s.objs[o].zone)
			}
			if s.objs[o].ver == v0 {
				t.Fatalf("precondition: ver = %d before and after the blink, want a new incarnation", v0)
			}
			s.npend = 0
			s.fireDelayed(k)
			if s.npend != 1 {
				t.Fatalf("%d pending after the fire, want the one stale trigger", s.npend)
			}
			pt := s.pend[0]
			if pt.obj != creator || pt.trigObj != o || pt.srcVer != v0 {
				t.Fatalf("pending obj %d trigObj %d srcVer %d, want source %d, referent %d and the registration version %d",
					pt.obj, pt.trigObj, pt.srcVer, creator, o, v0)
			}
			s.npend = 0
			s.resolveDelayed(k, &stackEntry{kind: seTrigger, obj: pt.obj, trigObj: pt.trigObj, srcVer: pt.srcVer,
				card: pt.card, ab: pt.ab, ctrl: pt.ctrl, trigAmt: pt.trigAmt})
			if got := s.objs[o].zone == kop.ZoneBattlefield; got != !tc.wantSac {
				t.Fatalf("on battlefield after the resolution = %v, want %v (zone %d)", got, !tc.wantSac, s.objs[o].zone)
			}
		})
	}
}
