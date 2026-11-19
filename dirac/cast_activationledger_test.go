package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

func ledgerScen(t *testing.T) *scen {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // the generated path
	return newScenTable(t, &tbl, Config{NoElide: true})
}

// TestActRingCountsPushesPerSource: one source activated past the old
// eight-push ring plays on, and ExThisTurnActGrave still counts every push
// (a repeatable pump in a four-seat Commander pod stalled 11 of 73 edh games
// before 2026-11-19).
func TestActRingCountsPushesPerSource(t *testing.T) {
	sc := ledgerScen(t)
	s := sc.s
	o := sc.hand(v.GrizzlyBears, 0)
	other := sc.hand(v.GrizzlyBears, 0)
	for i := 0; i < 20; i++ {
		s.noteAct(o, 0)
	}
	s.noteAct(other, 0)
	s.noteAct(o, 1) // another activator's push is its own
	if s.Stalled() {
		t.Fatalf("twenty pushes of one source stalled %q", StallName(s.StallReason()))
	}
	if n := s.thisTurnActGrave(evalCtx{ctrl: 0}); n != 0 {
		t.Fatalf("no source in a graveyard: %d, want 0", n)
	}
	s.objs[o].zone = kop.ZoneGraveyard
	if n := s.thisTurnActGrave(evalCtx{ctrl: 0}); n != 20 {
		t.Fatalf("player 0 with its source in the graveyard: %d, want 20", n)
	}
	if n := s.thisTurnActGrave(evalCtx{ctrl: 1}); n != 1 {
		t.Fatalf("player 1: %d, want 1", n)
	}
}

// TestActRingFullStalls: a distinct source past actRingN in one turn, or a
// push past one entry's count, is still a counted capacity stall.
func TestActRingFullStalls(t *testing.T) {
	sc := ledgerScen(t)
	s := sc.s
	for i := 0; i < actRingN; i++ {
		s.noteAct(sc.hand(v.GrizzlyBears, 0), 0)
	}
	if s.Stalled() {
		t.Fatalf("%d distinct sources stalled %q", actRingN, StallName(s.StallReason()))
	}
	s.noteAct(sc.hand(v.GrizzlyBears, 0), 0)
	if s.StallReason() != StallActRing {
		t.Fatalf("source %d: stall %q, want actring", actRingN+1, StallName(s.StallReason()))
	}

	sc = ledgerScen(t)
	s = sc.s
	o := sc.hand(v.GrizzlyBears, 0)
	for i := 0; i < actCntMax; i++ {
		s.noteAct(o, 0)
	}
	if s.Stalled() {
		t.Fatalf("%d pushes of one source stalled", actCntMax)
	}
	s.noteAct(o, 0)
	if s.StallReason() != StallActRing {
		t.Fatalf("push %d of one source: stall %q, want actring", actCntMax+1, StallName(s.StallReason()))
	}
}
