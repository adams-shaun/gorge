package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestHiddenLandCountsOnTheElectionsAnswer: a land played off a Hideaway land with
// a shock land's election takes its land play when the answer completes the play,
// as a land played from hand does.
func TestHiddenLandCountsOnTheElectionsAnswer(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // a table with Exprs prepares the gen tables
	land := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Shock", Types: kop.TLand, Abilities: []kop.Ability{{Kind: kop.AbReplacement,
		Repl: kop.Repl{Kind: kop.ReplEntersProgram},
		Prog: []kop.Instr{{Op: kop.OpUnlessPaid, Obj: kop.RefController, A: kop.Const(2), C: int16(kop.PayLife) | 1<<4}, {Op: kop.OpEnterTapped}}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	o := sc.obj(land, 0)
	sc.s.place(o, kop.ZoneExile, false)
	sc.s.res.e.ctrl = 0
	sc.s.hideLand(sc.k, o)
	if !sc.onBF(o) || sc.s.g.nask != 1 {
		t.Fatalf("on the battlefield %v, %d asks queued; want the land entered and its election queued", sc.onBF(o), sc.s.g.nask)
	}
	if n := sc.s.pl[0].landsPlayed; n != 0 {
		t.Fatalf("%d land plays with the election open, want 0", n)
	}
	sc.s.answerQueuedAsk(sc.k, MakeAction(AkChoose, 0, 0, 0))
	if n := sc.s.pl[0].landsPlayed; n != 1 {
		t.Fatalf("%d land plays after the answer, want 1", n)
	}
}
