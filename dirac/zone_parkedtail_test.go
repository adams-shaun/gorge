package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// midResFix is a Banner of Kinship (an artifact whose as-enters replacement
// queues a type election) in the top of player 0's library, and a spell whose
// program is prog (filter f matches an artifact in the library).
type midResFix struct {
	sc     *scen
	banner Obj
}

func newMidResFix(t *testing.T, mk func(art, land kop.FilterID) []kop.Instr) *midResFix {
	t.Helper()
	deck := []cardset.Entry{{Name: "Banner of Kinship", Count: 4}, {Name: "Plains", Count: 16}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var ban, pl CardID = 255, 255
	for i := range tbl.Cards {
		switch tbl.Cards[i].Name {
		case "Banner of Kinship":
			ban = CardID(i)
		case "Plains":
			pl = CardID(i)
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TArtifact, Zone: kop.ZoneLibrary}, kop.Filter{Types: kop.TLand, Zone: kop.ZoneLibrary})
	art, land := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
	spell := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Fetcher", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: mk(art, land)}}})
	sc := newScenTableLib(t, tbl, pl, Config{NoElide: true})
	if sc.k.gen == nil {
		t.Fatal("the fixture table has no generated state")
	}
	s := sc.s
	b := sc.obj(ban, 0)
	s.objs[b].zone = kop.ZoneLibrary
	s.pl[0].libTop(b) // the dig looks at the top; a search finds it anywhere
	s.res = resCtx{e: stackEntry{kind: seSpell, obj: NoObj, card: spell, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.e.tgt = noTargets
	s.ctl = ctlResolve
	return &midResFix{sc: sc, banner: b}
}

func (m *midResFix) libOrder() []Obj {
	pl := &m.sc.s.pl[0]
	return append([]Obj(nil), pl.lib[pl.libLo:pl.libHi]...)
}

// TestMidResSearchEnterAskOrder: a library search whose found card enters
// with a queued as-enters election suspends at the election; the mandatory
// shuffle (CR 701.24b) runs once it is answered, as gorge does (the search
// effect's chain stops at the ask).
func TestMidResSearchEnterAskOrder(t *testing.T) {
	m := newMidResFix(t, func(art, land kop.FilterID) []kop.Instr {
		return []kop.Instr{
			{Op: kop.OpSearchEx, Obj: kop.RefController, A: kop.Const(1), F: art, C: int16(kop.ZoneBattlefield)},
			{Op: kop.OpSearchEx, Obj: kop.RefController, A: kop.Const(1), F: land, C: int16(kop.ZoneHand)},
		}
	})
	s, k := m.sc.s, m.sc.k
	in := &s.prog(k)[0]
	if s.startSearch(k, in) || s.res.ask != raGenSearch {
		t.Fatalf("ask %d, want the search pick", s.res.ask)
	}
	before, draws := m.libOrder(), s.rng.Draws
	s.applyResolveGen(k, MakeAction(AkChoose, TCode(m.banner), 0, 0))
	if s.objs[m.banner].zone != kop.ZoneBattlefield {
		t.Fatalf("the found Banner is in zone %d, want the battlefield", s.objs[m.banner].zone)
	}
	if s.g.nask != 1 || s.res.ask != raGenEnterAsk || s.res.left != tailSearch {
		t.Fatalf("nask %d ask %d left %d, want the Banner's election posed with the shuffle parked", s.g.nask, s.res.ask, s.res.left)
	}
	if s.rng.Draws != draws {
		t.Fatalf("the shuffle ran before the election was answered: draws %d -> %d", draws, s.rng.Draws)
	}
	want := before[:0:0]
	for _, o := range before {
		if o != m.banner {
			want = append(want, o)
		}
	}
	for i, o := range m.libOrder() {
		if o != want[i] {
			t.Fatalf("the library order changed before the election was answered (slot %d)", i)
		}
	}
	// answer the election: the shuffle runs, then the program resumes (the
	// second search poses its pick at pc 1)
	s.applyResolveGen(k, MakeAction(AkChoose, 0, 0, 0))
	if s.rng.Draws == draws {
		t.Fatal("the shuffle did not run after the election was answered")
	}
	if s.g.nask != 0 || s.res.left != 0 {
		t.Fatalf("after the answer: nask %d left %d, want the queue drained and the tail consumed", s.g.nask, s.res.left)
	}
	if s.res.pc != 1 || s.res.ask != raGenSearch {
		t.Fatalf("pc %d ask %d after the answer, want the program resumed at the next search's pick", s.res.pc, s.res.ask)
	}
}

// TestMidResDigEnterAskOrder: a dig whose taken card enters with a queued
// election suspends before the rest is ordered and placed.
func TestMidResDigEnterAskOrder(t *testing.T) {
	m := newMidResFix(t, func(art, land kop.FilterID) []kop.Instr {
		return []kop.Instr{
			{Op: kop.OpDigEx, Obj: kop.RefController, A: kop.Const(4), B: kop.Const(1), F: art,
				C: int16(kop.ZoneBattlefield) | int16(kop.ZoneGraveyard|kop.DigRandomOrder)<<3},
			{Op: kop.OpSearchEx, Obj: kop.RefController, A: kop.Const(1), F: land, C: int16(kop.ZoneHand)},
		}
	})
	s, k := m.sc.s, m.sc.k
	in := &s.prog(k)[0]
	draws := s.rng.Draws
	if s.startDig(k, in) { // the one Banner is forced: taken unasked, then its election
		t.Fatal("startDig finished, want the election to suspend the program")
	}
	if s.objs[m.banner].zone != kop.ZoneBattlefield {
		t.Fatalf("the taken Banner is in zone %d, want the battlefield", s.objs[m.banner].zone)
	}
	// The dig opened the enters hold, so its parked tail keeps the Banner's
	// enters checks held until the rest is placed (tailDigH).
	if s.g.nask != 1 || s.res.ask != raGenEnterAsk || s.res.left != tailDigH {
		t.Fatalf("nask %d ask %d left %d, want the election posed with the dig's tail parked, checks held", s.g.nask, s.res.ask, s.res.left)
	}
	if s.rng.Draws != draws || s.pl[0].graveN != 0 {
		t.Fatalf("the rest was ordered/placed before the election: draws %d -> %d, graveyard %d", draws, s.rng.Draws, s.pl[0].graveN)
	}
	s.applyResolveGen(k, MakeAction(AkChoose, 0, 0, 0))
	if s.rng.Draws == draws || s.pl[0].graveN != 3 {
		t.Fatalf("after the answer: draws %d -> %d, graveyard %d, want the rest (3) ordered and placed", draws, s.rng.Draws, s.pl[0].graveN)
	}
	if s.g.nask != 0 || s.res.left != 0 || s.res.pc != 1 || s.res.ask != raGenSearch {
		t.Fatalf("after the answer: nask %d left %d pc %d ask %d, want drained, consumed, resumed", s.g.nask, s.res.left, s.res.pc, s.res.ask)
	}
}
