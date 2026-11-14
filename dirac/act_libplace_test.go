package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// elixirScen is Elixir of Immortality's bracketed graveyard sweep (ShuffleOwners'
// pool) on a permanent of seat 0 with n Islands in seat 0's graveyard; run plays its activated
// ability's program from the start until an ask is posed or it ends.
func elixirScen(t *testing.T, n int) (sc *scen, src Obj, grave []Obj, run func() bool) {
	t.Helper()
	set := cardsettest.Set(t, "shuffleowners")
	deck := []cardset.Entry{{Name: "Elixir of Immortality", Count: 4}, {Name: "Island", Count: 36}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	// the ability's own program ends in a shuffle; keep the bracketed sweep only
	cd := tbl.Cards[id["Elixir of Immortality"]]
	ab := cd.Abilities[0]
	var prog []kop.Instr
	for _, in := range ab.Prog {
		if in.Op == kop.OpChangeZoneAll || in.Op == kop.OpChangeZoneAllEx || in.Op == kop.OpExt && kop.ExtOp(in.A.N) == kop.ExtLibPlace {
			prog = append(prog, in)
		}
	}
	if len(prog) != 3 {
		t.Fatalf("Elixir's sweep is bracketed by %d instructions, want arm, sweep, ask", len(prog))
	}
	ab.Prog = prog
	cd.Abilities = []kop.Ability{ab}
	sweeper := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, cd)
	sc = newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	s := sc.s
	src = sc.perm(sweeper, 0)
	grave = sc.grave(id["Island"], 0, n)
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: src, srcVer: s.objs[src].ver, card: sweeper, ab: 0, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	s.scrResEnter()
	run = func() bool {
		for ; int(s.res.pc) < len(prog); s.res.pc++ {
			in := prog[s.res.pc]
			if !s.exec(sc.k, &in) {
				return false
			}
		}
		return true
	}
	return sc, src, grave, run
}

// TestLibPlaceAskOwnerOrders: Elixir's graveyard sweep puts three cards on top
// of their owner's library (gorge effChangeZoneAll's top placement), so the
// owner is asked to order them (CR 401.4, gorge libraryPlacementOrder): the
// ask is the owner's, over the three cards in move order -- which gorge still
// has at the library's bottom -- and the picked order, the first nearest the
// top, ends on top.
func TestLibPlaceAskOwnerOrders(t *testing.T) {
	sc, src, grave, run := elixirScen(t, 3)
	s := sc.s
	pl := &s.pl[0]
	before, top := pl.libLen(), pl.lib[pl.libLo]
	if run() || s.res.ask != raGenLibPlace {
		t.Fatalf("ask %d, want the library placement ask", s.res.ask)
	}
	if s.res.askP != 0 {
		t.Fatalf("asked seat %d, want the cards' owner 0", s.res.askP)
	}
	// mid-ask, as gorge has it: the moved cards at the bottom in move order
	if got := pl.lib[pl.libLo]; got != top {
		t.Fatalf("top of the library is %d, want the untouched %d", got, top)
	}
	if got := slices.Clone(pl.lib[int(pl.libHi)-3 : pl.libHi]); !slices.Equal(got, grave) {
		t.Fatalf("bottom three %v, want the swept cards in move order %v", got, grave)
	}
	want := []Obj{grave[2], grave[0], grave[1]}
	for i, w := range want {
		var d Decision
		s.resolveOptionsGen(sc.k, &d)
		if d.Sub != ChArrange || int(d.N) != 3-i {
			t.Fatalf("pick %d: sub %d with %d options, want ChArrange over %d", i, d.Sub, d.N, 3-i)
		}
		s.applyAskCold2(sc.k, MakeAction(AkChoose, TCode(w), 0, 0))
	}
	if s.res.ask == raGenLibPlace {
		t.Fatal("the ask is still posed after every card was ordered")
	}
	if pl.libLen() != before+3 {
		t.Fatalf("library holds %d, want %d", pl.libLen(), before+3)
	}
	if got := libTopOrder(s, 0, 4); !slices.Equal(got, []Obj{grave[2], grave[0], grave[1], top}) {
		t.Fatalf("library top %v, want the picked order then the old top %v", got, []Obj{grave[2], grave[0], grave[1], top})
	}
	_ = src
}

// TestLibPlaceOneCardAsksNothing: one swept card is no arrangement (CR 401.4
// needs two or more); it goes on top and the program ends unasked.
func TestLibPlaceOneCardAsksNothing(t *testing.T) {
	sc, _, grave, run := elixirScen(t, 1)
	s := sc.s
	top := s.pl[0].lib[s.pl[0].libLo]
	if !run() {
		t.Fatalf("ask %d posed for a single swept card", s.res.ask)
	}
	if got := libTopOrder(s, 0, 2); !slices.Equal(got, []Obj{grave[0], top}) {
		t.Fatalf("library top %v, want the swept card above the old top %v", got, []Obj{grave[0], top})
	}
}
