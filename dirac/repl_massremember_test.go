package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDestroyAllRemembersParkedVictims: a destroy-all with RememberDestroyed$
// whose later victim is parked for a CR 616.1 order ask is remembered when its
// parked move finally reaches the graveyard, not by the immediate scan that
// runs while it is still on the battlefield. The earlier victim that moved
// before the ask is remembered at once, so the battlefield walk order holds.
func TestDestroyAllRemembersParkedVictims(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Day of Judgment", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 4},
	}, nil)
	s, k := sc.s, sc.k
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	own := sc.perm(id["Grizzly Bears"], 0) // p0's own creature: the Leylines do not replace it
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
	if s.graveMoveCompetes(k, own) {
		t.Fatal("precondition: p0's own creature's move competes although the Leylines only replace p1's")
	}
	s.ctl = ctlResolve
	s.res = resCtx{e: stackEntry{card: id["Day of Judgment"], ctrl: 0}, active: true}
	all := firstInstr(t, sc, id["Day of Judgment"], kop.OpDestroyAll)
	all.C |= 1 // RememberDestroyed$ (Starfall Invocation's form)
	s.exec(k, &all)
	s.ctl = ctlExileOrder
	if s.Stalled() {
		t.Fatalf("the destroy-all stalled: %s", StallName(s.StallReason()))
	}
	if !s.parkedWaiting() || s.cold.parked[0].kind != parkMass || s.cold.parked[0].obj != bear {
		t.Fatalf("parkedWaiting %v head kind %d obj %d, want the bear's parkMass ask", s.parkedWaiting(), s.cold.parked[0].kind, s.cold.parked[0].obj)
	}
	if !sc.onBF(bear) || !sc.onBF(tok) {
		t.Fatalf("at the ask: bear on battlefield %v, token %v; want both", sc.onBF(bear), sc.onBF(tok))
	}
	// p0's own creature was before the first competing victim, so it already
	// moved and was remembered before the ask.
	if z := s.objs[own].zone; z != kop.ZoneGraveyard {
		t.Fatalf("p0's own creature is in zone %d, want the graveyard before the ask", z)
	}
	if s.g.nrem != 1 || s.g.rem[0] != own {
		t.Fatalf("at the ask remembered %v, want only p0's own creature %d", s.g.rem[:s.g.nrem], own)
	}
	s.res.active = false // no live program left to continue in this unit test
	var d Decision
	s.exileOrderOptions(k, &d)
	s.applyExileOrder(k, d.Options[0])
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("the answered bear is in zone %d, want exile", z)
	}
	if z := s.objs[tok].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the zero-source token is in zone %d, want the graveyard", z)
	}
	if s.g.nrem != 2 || s.g.rem[0] != own || s.g.rem[1] != tok {
		t.Fatalf("after the answer remembered %v, want [%d %d]", s.g.rem[:s.g.nrem], own, tok)
	}
	if s.parkedWaiting() || s.Stalled() {
		t.Fatalf("after the answer: parkedWaiting %v stalled %v", s.parkedWaiting(), s.Stalled())
	}
}
