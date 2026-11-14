package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A manifest dread window's other card moving to the graveyard under two
// any-zone exile-instead replacements is parked for its CR 616.1 order ask
// (CR 701.62a): it stays on the library list until the answer, and the answer
// exiles it. Before the park the move stalled at StallReplLeave.
func TestManifestDreadOtherCardParksExileOrder(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Plains", Count: 4},
	}, nil)
	s, k := sc.s, sc.k
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	pl := &s.pl[1]
	keep, other := pl.lib[pl.libLo], pl.lib[pl.libLo+1]
	if !s.graveMoveCompetes(k, other) {
		t.Fatal("precondition: the other card's move is not contested by two exile-instead sources")
	}
	prog := []kop.Instr{{Op: kop.OpManifestDread, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(0)}}
	src := CardID(len(k.t.Cards))
	k.t.Cards = append(k.t.Cards, kop.CardDef{Name: "Test Dread", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	s.res = resCtx{e: stackEntry{card: src, ab: 0, ctrl: 1}, active: true}
	s.ctl = ctlResolve
	s.sp = 1
	if s.exec(k, &prog[0]) || s.res.ask != raGenManifestDread {
		t.Fatalf("ask %d, want the manifest dread window", s.res.ask)
	}
	var d Decision
	s.resolveOptionsGen(k, &d)
	var pick Action
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Source() == TCode(uint8(keep)) {
			pick = d.Options[i]
		}
	}
	if pick.Kind() != AkChoose {
		t.Fatal("the top card to manifest is not offered")
	}
	s.applyAskOblige(k, pick)

	if s.Stalled() {
		t.Fatalf("the window's other-card move stalled: %s", StallName(s.StallReason()))
	}
	if s.ctl != ctlExileOrder || !s.parkedWaiting() {
		t.Fatalf("ctl %d parkedWaiting %v, want the parked dread order ask", s.ctl, s.parkedWaiting())
	}
	if z := s.objs[other].zone; z != kop.ZoneLibrary {
		t.Fatalf("the asked card is in zone %d, want the library", z)
	}
	if pl.lib[pl.libLo] != other {
		t.Fatal("the asked card left the library list before its answer")
	}
	if z := s.objs[keep].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the chosen card is in zone %d, want the battlefield", z)
	}
	var od Decision
	s.exileOrderOptions(k, &od)
	if od.Kind != DkChoose || od.Sub != ChGenReplOrder || od.Player != 1 || od.N != 2 {
		t.Fatalf("decision kind %d sub %d player %d n %d, want DkChoose/ChGenReplOrder for its owner over two sources",
			od.Kind, od.Sub, od.Player, od.N)
	}
	s.applyExileOrder(k, od.Options[0])
	if z := s.objs[other].zone; z != kop.ZoneExile {
		t.Fatalf("the answered card is in zone %d, want exile", z)
	}
	if s.parkedWaiting() || s.Stalled() {
		t.Fatalf("after the answer: parkedWaiting %v stalled %v", s.parkedWaiting(), s.Stalled())
	}
	for i := int(pl.libLo); i < int(pl.libHi); i++ {
		if pl.lib[i] == other {
			t.Fatal("the answered card is still on the library list")
		}
	}
}
