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
	if len(s.cold.parked) == 0 || s.parkedAny() {
		t.Fatalf("precondition: parked queue len %d live %v, want allocated and empty", len(s.cold.parked), s.parkedAny())
	}
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

// TestDestroyAllBatchedParksFromEmptyQueue: the same destroy-all on a table with
// a "one or more" leave line (the leaveAllBatched path) still parks at the first
// competing victim from an allocated-but-empty park queue. The report that it
// skipped the park misread len(cold.parked), a fixed allocation, as occupancy
// (see 742c2c8a4, c87bb9ee0).
func TestDestroyAllBatchedParksFromEmptyQueue(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Day of Judgment", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Midnight Reaper", Count: 1},
		{Name: "Plains", Count: 4},
	}, func(tbl *kop.Table, id map[string]CardID) {
		cd := &tbl.Cards[id["Midnight Reaper"]]
		cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
		for j := range cd.Abilities {
			if a := &cd.Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigDies {
				a.Trig.Event = kop.TrigLeaves
				a.Trig.Flags2 |= kop.Tf2Batch
				return
			}
		}
		t.Fatal("Midnight Reaper has no dies trigger to make a batch line")
	})
	s, k := sc.s, sc.k
	if !k.gen.anyBatch {
		t.Fatal("the table has no batch line")
	}
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	reaper := sc.perm(id["Midnight Reaper"], 0)
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
	if s.graveMoveCompetes(k, reaper) {
		t.Fatal("precondition: the reaper's move competes although its owner controls the Leylines")
	}
	if len(s.cold.parked) == 0 || s.parkedAny() {
		t.Fatalf("precondition: parked queue len %d live %v, want allocated and empty", len(s.cold.parked), s.parkedAny())
	}
	s.ctl = ctlResolve
	s.res = resCtx{e: stackEntry{card: id["Day of Judgment"], ctrl: 0}, active: true}
	all := firstInstr(t, sc, id["Day of Judgment"], kop.OpDestroyAll)
	s.exec(k, &all)
	s.ctl = ctlExileOrder
	if s.Stalled() {
		t.Fatalf("the destroy-all stalled: %s", StallName(s.StallReason()))
	}
	if !s.parkedWaiting() || s.cold.parked[0].kind != parkMass {
		t.Fatalf("parkedWaiting %v, want the bear's parkMass ask", s.parkedWaiting())
	}
	if sc.onBF(reaper) {
		t.Fatal("the non-competing reaper prefix is still on the battlefield")
	}
	if !sc.onBF(bear) || !sc.onBF(tok) {
		t.Fatalf("at the ask: bear on battlefield %v, token %v; want both", sc.onBF(bear), sc.onBF(tok))
	}
	var d Decision
	s.exileOrderOptions(k, &d)
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
		t.Fatalf("decision kind %d sub %d player %d n %d, want DkChoose/ChGenReplOrder for player 1 over two sources", d.Kind, d.Sub, d.Player, d.N)
	}
	s.res.active = false
	s.applyExileOrder(k, d.Options[0])
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("the answered bear is in zone %d, want exile", z)
	}
	if sc.onBF(tok) {
		t.Fatal("the token is still on the battlefield after the answer")
	}
	if s.parkedWaiting() || s.Stalled() {
		t.Fatalf("after the answer: parkedWaiting %v stalled %v", s.parkedWaiting(), s.Stalled())
	}
}
