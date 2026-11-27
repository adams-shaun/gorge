package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTgfStackOppCtrl: a TgtPermOrSpell slot flagged TgfStackOppCtrl (Sink
// into Stupor, gorge Card.inZoneStack+OppCtrl) accepts only an opponent's
// spell from the stack; unflagged it accepts either player's.
func TestTgfStackOppCtrl(t *testing.T) {
	tbl := &kop.Table{
		Filters: []kop.Filter{{Types: kop.TCreature}},
		Exprs:   []kop.Expr{{Kind: kop.ExConst, N: 1}},
		Cards: []kop.CardDef{
			{Name: "Test Plains", Types: kop.TLand},
			{Name: "Test Instant", Types: kop.TInstant},
		},
	}
	sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
	s, k := sc.s, sc.k
	mine, theirs, src := sc.obj(1, 0), sc.obj(1, 1), sc.obj(1, 0)
	for i, o := range []Obj{mine, theirs} {
		s.objs[o].zone = kop.ZoneStack
		s.stack[i] = stackEntry{kind: seSpell, obj: o, ctrl: uint8(i)}
	}
	s.sp = 2
	flagged := &kop.TargetSpec{Kind: kop.TgtPermOrSpell, Filter: 0, TFlags: kop.TgfStackOppCtrl}
	plain := &kop.TargetSpec{Kind: kop.TgtPermOrSpell, Filter: 0}
	ok := func(ts *kop.TargetSpec, o Obj) bool { return s.targetOKPermOrSpell(k, ts, 0, NoSlot, src, o) }
	if !ok(plain, mine) || !ok(plain, theirs) {
		t.Fatal("precondition: an unflagged slot must accept either player's spell")
	}
	if ok(flagged, mine) {
		t.Fatal("TgfStackOppCtrl accepted the chooser's own spell")
	}
	if !ok(flagged, theirs) {
		t.Fatal("TgfStackOppCtrl refused an opponent's spell")
	}
}
