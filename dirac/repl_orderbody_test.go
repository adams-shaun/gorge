package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReplOrdBodyUntouchedByHeaderResets: a noncombat parked order ask, then a
// combat freeze in the same game. The body (replBody) shares scr with the
// other arms (scratch.go), so it is all-zero only after
// replOrderEnd; the header resets (the parked ask and its answer) never touch
// it, and replOrderFreeze clears it before it fills (the body is scribbled
// after the park, so a freeze that skips its clear leaves the scribble
// behind).
func TestReplOrdBodyUntouchedByHeaderResets(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twinflame Tyrant", Count: 2}, {Name: "Burst Lightning", Count: 1},
		{Name: "Erudite Wizard", Count: 2}, {Name: "Mountain", Count: 12}}
	sc, id := replScen(t, deck)
	s := sc.s
	sc.perm(id["Twinflame Tyrant"], 0)
	sc.perm(id["Twinflame Tyrant"], 0)
	w := sc.perm(id["Erudite Wizard"], 0)
	zero := func(when string) {
		t.Helper()
		if *s.roBody() != (replBody{}) {
			t.Fatalf("%s: the body is not all-zero", when)
		}
	}
	d := burnToAsk(t, sc, id, TCode(PlayerTarget1))
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || s.g.ro.on || s.g.ro.n != 2 {
		t.Fatalf("want the noncombat order ask parked (on %v n %d)", s.g.ro.on, s.g.ro.n)
	}
	s.Apply(d.Options[0])
	s.Step()
	if s.g.ro.replHdr != (replHdr{}) {
		t.Fatalf("the answered noncombat ask left a header: %+v", s.g.ro.replHdr)
	}

	// scribble the body's far end (past any frozen event): only the freeze's own clear removes it
	s.roBody().fz[MaxPerm-1].n = 7
	s.roBody().ord[MaxPerm-1] = 3
	s.roBody().srcDone = s.roBody().srcDone.with(5)

	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(w), scAny)
	sc.do(t, AkDone, scAny, scAny)
	asks := 0
	for i := 0; i < 40; i++ {
		d := s.Step()
		switch {
		case d.Kind == DkChoose && d.Sub == ChGenReplOrder:
			asks++
			if !s.g.ro.on || s.g.ro.nfz == 0 {
				t.Fatalf("combat ask without a frozen pass (on %v nfz %d)", s.g.ro.on, s.g.ro.nfz)
			}
			if s.roBody().fz[MaxPerm-1].n != 0 || s.roBody().ord[MaxPerm-1] != 0 || s.roBody().srcDone != (slotMask{}) {
				t.Fatalf("the freeze left the scribbled body behind")
			}
			s.Apply(d.Options[0])
		case d.Kind == DkPriority:
			if asks == 0 {
				if s.step >= kop.StepEndCombat {
					t.Fatal("combat ended without an order ask")
				}
				s.Apply(MakeAction(AkPass, 0, 0, 0))
				continue
			}
			zero("after replOrderEnd")
			if s.g.ro.replHdr != (replHdr{}) {
				t.Fatalf("a header after the pass: %+v", s.g.ro.replHdr)
			}
			return
		default:
			t.Fatalf("unexpected decision kind %d sub %d", d.Kind, d.Sub)
		}
	}
	t.Fatal("no end of combat")
}
