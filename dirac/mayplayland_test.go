package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMayPlayLandSpend: a land played from exile through a permission
// record spends the record, as a cast through one does (CR 701.18b: to play
// a card is to play it as a land or cast it). Before the fix the land's
// record outlived the play (seed 81 of the choosecardparams activate-first
// lockstep: four records beside gorge's two MayPlay effects).
func TestMayPlayLandSpend(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	o := sc.obj(v.Island, 0)
	s.place(o, kop.ZoneExile, false)
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurYourNextTurn), turn: mayWord(s.turn)}
	s.g.nmay = 1
	sc.do(t, AkPlayLand, TCode(o), scAny)
	if !sc.onBF(o) {
		t.Fatal("the exiled Island was not played")
	}
	if n := s.FeatMayCount(); n != 0 {
		t.Fatalf("the played land's permission was not spent: %d records left (%v)", n, s.g.may[:s.g.nmay])
	}

	// the cast path still spends its record (the shared spendMayRecord)
	c := sc.obj(v.ThinkTwice, 0)
	s.place(c, kop.ZoneExile, false)
	s.g.may[0] = mayPlay{obj: c, ver: s.objs[c].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
	s.g.nmay = 1
	*s.bld() = build{e: stackEntry{kind: seSpell, obj: c, card: v.ThinkTwice, ctrl: 0}}
	s.castFlagsGen(sc.k, c, castMayPlay)
	if n := s.FeatMayCount(); n != 0 {
		t.Fatalf("the cast's permission was not spent: %d records left", n)
	}
	if s.bld().e.flags&sfMayPlay == 0 {
		t.Fatal("the may-play cast did not carry sfMayPlay")
	}
}
