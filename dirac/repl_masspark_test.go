package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDestroyAllParksLaterVictimsExileOrder: a destroy-all's first victim whose
// move competes for two exile-instead replacements suspends the whole batch
// (CR 616.1), so a later victim whose own move has nothing to order is still on
// the battlefield at that ask. Parking only the competing victims moved the
// later one early.
func TestDestroyAllParksLaterVictimsExileOrder(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Day of Judgment", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 4},
	}, nil)
	s, k := sc.s, sc.k
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	bear := sc.perm(id["Grizzly Bears"], 1)
	tok := s.newToken(id["Grizzly Bears"], 1)
	s.enter(k, tok, 1, false, kop.ZoneNone)
	s.npend = 0
	if !s.graveMoveCompetes(k, bear) {
		t.Fatal("precondition: the bear's move does not compete for the two Leylines")
	}
	if s.graveMoveCompetes(k, tok) {
		t.Fatal("precondition: the token's move competes although the Leyline excludes tokens")
	}
	s.ctl = ctlResolve
	s.res = resCtx{e: stackEntry{card: id["Day of Judgment"], ctrl: 0}, active: true}
	all := firstInstr(t, sc, id["Day of Judgment"], kop.OpDestroyAll)
	s.exec(k, &all)
	s.ctl = ctlExileOrder // what runProg does when the instruction parks
	if s.Stalled() {
		t.Fatalf("the destroy-all stalled: %s", StallName(s.StallReason()))
	}
	if !s.parkedWaiting() || s.cold.parked[0].kind != parkMass {
		t.Fatalf("parkedWaiting %v head kind %d, want the bear's parkMass ask", s.parkedWaiting(), s.cold.parked[0].kind)
	}
	if !sc.onBF(bear) || !sc.onBF(tok) {
		t.Fatalf("at the ask: bear on battlefield %v, token %v; want both", sc.onBF(bear), sc.onBF(tok))
	}
	var d Decision
	s.exileOrderOptions(k, &d)
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
		t.Fatalf("decision kind %d sub %d player %d n %d, want DkChoose/ChGenReplOrder for its owner over two sources", d.Kind, d.Sub, d.Player, d.N)
	}
	s.res.active = false // no live program left to continue in this unit test
	s.applyExileOrder(k, d.Options[0])
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("the answered bear is in zone %d, want exile", z)
	}
	// The token's turn came after the answer and it had nothing to order: it
	// moved with no second ask.
	if sc.onBF(tok) {
		t.Fatal("the non-competing token is still on the battlefield after the answer")
	}
	if s.parkedWaiting() || s.Stalled() {
		t.Fatalf("after the answer: parkedWaiting %v stalled %v", s.parkedWaiting(), s.Stalled())
	}
}
