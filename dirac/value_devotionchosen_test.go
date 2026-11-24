package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExDevotionChosenEval: ExDevotionChosen is the controller's devotion to
// the colour recorded on the source, 0 with none recorded; the record and
// clear primitives write the slot an as-enters ChooseColor answer lands in.
func TestExDevotionChosenEval(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 4}, {Name: "Plains", Count: 8}})
	fc, pl := id["Valley Floodcaller"], id["Plains"]
	sc.perm(fc, 0)
	sc.perm(fc, 0)
	sc.perm(fc, 1)
	src := sc.perm(pl, 0) // stands in for the recording source
	sl := sc.slot(src)
	cost := sc.k.t.Cards[fc].Cost
	if cost.Pips[1] == 0 {
		t.Fatal("Valley Floodcaller has no blue pip")
	}
	blue, white := 2*int(cost.Pips[1]), 2*int(cost.Pips[0])
	if blue == white {
		t.Fatalf("blue and white devotion both %d: the test cannot tell the colours apart", blue)
	}
	sc.k.t.Exprs = append(sc.k.t.Exprs, kop.Expr{Kind: kop.ExDevotionChosen})
	i := int16(len(sc.k.t.Exprs) - 1)
	ctx := evalCtx{src: src, srcSl: sl, ctrl: 0}
	eval := func() int { return sc.s.expr(sc.k, i, ctx) }

	if sc.s.perms[sl].chosen != 255 {
		t.Fatalf("a fresh permanent already records %d", sc.s.perms[sl].chosen)
	}
	if got := eval(); got != 0 {
		t.Fatalf("no colour recorded: %d, want 0", got)
	}
	sc.s.recordChosenColor(sl, 1) // blue
	if got := eval(); got != blue {
		t.Fatalf("blue recorded: %d, want %d", got, blue)
	}
	if got, want := sc.s.devotion(sc.k, kop.ColU, 0), eval(); got != want {
		t.Fatalf("expr %d disagrees with devotion to blue %d", want, got)
	}
	sc.s.recordChosenColor(sl, 0) // white
	if got := eval(); got != white {
		t.Fatalf("white recorded: %d, want %d", got, white)
	}
	if sc.s.objs[src].chosen != 0 {
		t.Fatalf("the card object keeps %d, want the recorded colour 0", sc.s.objs[src].chosen)
	}
	sc.s.recordChosenColor(sl, 7) // not a colour: ignored
	if got := eval(); got != white {
		t.Fatalf("an out-of-range colour changed the record: %d, want %d", got, white)
	}
	sc.s.clearChosenColor(sl)
	if got := eval(); got != 0 {
		t.Fatalf("after clear: %d, want 0", got)
	}
	if sc.s.perms[sl].chosen != 255 || sc.s.objs[src].chosen != 255 {
		t.Fatalf("clear left perm %d obj %d", sc.s.perms[sl].chosen, sc.s.objs[src].chosen)
	}
}
