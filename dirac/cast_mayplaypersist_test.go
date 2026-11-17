package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMayPersistMoved: an Effect-delivered graveyard grant (DurWhileGrave)
// survives the card's stay and is re-armed with the card's new version when
// it returns; a ForgetOnMoved$ Exile grant is dropped on the move to exile.
func TestMayPersistMoved(t *testing.T) {
	sc := newScenTable(t, v.NewTable(), Config{})
	s := sc.s
	o := sc.obj(v.ThinkTwice, 0)
	s.objs[o].zone = kop.ZoneGraveyard

	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurWhileGraveForget), turn: mayWord(s.turn)}
	s.g.nmay = 1
	s.objs[o].ver++ // a zone change gives the card a new version
	s.mayPersistMoved(o, kop.ZoneExile)
	if s.g.nmay != 0 {
		t.Fatalf("ForgetOnMoved$ Exile kept the record after a move to exile")
	}

	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurWhileGrave), turn: mayWord(s.turn)}
	s.g.nmay = 1
	s.objs[o].ver++
	s.mayPersistMoved(o, kop.ZoneExile)
	if s.g.nmay != 1 {
		t.Fatalf("a no-ForgetOnMoved grant was dropped on a move to exile")
	}
	s.objs[o].ver++
	s.mayPersistMoved(o, kop.ZoneGraveyard)
	if s.g.nmay != 1 || s.g.may[0].ver != s.objs[o].ver {
		t.Fatalf("record not re-armed: nmay=%d ver=%d, want 1/%d", s.g.nmay, s.g.may[0].ver, s.objs[o].ver)
	}
}
