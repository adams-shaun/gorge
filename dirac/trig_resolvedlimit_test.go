package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestResolvedLimit: a resolution of a ResolvedLimit$ line records its
// source (the object as it is), after which the source's limited lines do
// not queue this turn; a new object (a zone change) and the next turn start
// fresh.
func TestResolvedLimit(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2, Abilities: []kop.Ability{
		{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigETB, Flags2: kop.Tf2ResolvedLimit}, Prog: gain}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	src := sc.perm(bear, 0)
	if s.rlSpent(src) {
		t.Fatal("nothing resolved yet")
	}
	e := stackEntry{obj: src, srcVer: s.objs[src].ver}
	s.countRL(&e)
	if !s.rlSpent(src) {
		t.Fatal("the source resolved its limited line")
	}
	l := trigListener{obj: src, ab: 0, card: bear, ctrl: 0}
	n := s.npend
	s.queueTrig(s.k(), l, &s.k().t.Cards[bear].Abilities[0], NoObj, 0, 0)
	if s.npend != n {
		t.Fatal("a spent source's limited line queued")
	}
	s.objs[src].ver++ // a new object (CR 400.7)
	if s.rlSpent(src) {
		t.Fatal("a new object has resolved nothing")
	}
	s.objs[src].ver--
	s.g.nused = 0 // the next turn
	if s.rlSpent(src) {
		t.Fatal("the count is per turn")
	}
}
