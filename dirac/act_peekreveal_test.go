package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPeekPublic: OpPeek with PeekPublic (gorge effReveal's public
// PeekAndReveal) poses no ask and, with PeekRemember, appends the top A
// library cards to the remembered set.
func TestPeekPublic(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	sc.k.t.Exprs = append(sc.k.t.Exprs, kop.Expr{Kind: kop.ExConst, N: 2})
	a := kop.ValueRef{K: kop.VExpr, N: int16(len(sc.k.t.Exprs) - 1)}
	pl := &sc.s.pl[0]
	in := kop.Instr{Op: kop.OpPeek, A: a, C: kop.PeekPublic}
	sc.s.g.nrem = 0
	sc.s.peekPublic(sc.k, &in, 0)
	if sc.s.g.nrem != 0 {
		t.Fatal("a public reveal without RememberRevealed remembered cards")
	}
	sc.s.remember(pl.lib[int(pl.libLo)+5]) // an earlier remembered card stays
	in.C |= kop.PeekRemember
	sc.s.peekPublic(sc.k, &in, 0)
	if sc.s.g.nrem != 3 || sc.s.g.rem[1] != pl.lib[int(pl.libLo)] || sc.s.g.rem[2] != pl.lib[int(pl.libLo)+1] {
		t.Fatalf("remembered %d %v, want the earlier card then the top two", sc.s.g.nrem, sc.s.g.rem[:sc.s.g.nrem])
	}
}

// gatheringStoneScen is a Gathering Stone on seat 0's battlefield over the
// real peekreveal pool, with the card ids by name and the Bear / Giant
// subtype indexes.
func gatheringStoneScen(t *testing.T) (sc *scen, id map[string]CardID, stone Obj, bear, giant uint8) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Forest", Count: 4}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Gathering Stone", Count: 1}, {Name: "Hill Giant", Count: 1},
	}
	set := cardsettest.Set(t, "peekreveal")
	tbl, a, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for _, c := range a {
		id[tbl.Cards[c].Name] = c
	}
	bear, giant = 255, 255
	for i, n := range tbl.Subtypes {
		switch n {
		case "Bear":
			bear = uint8(i)
		case "Giant":
			giant = uint8(i)
		}
	}
	if bear == 255 || giant == 255 {
		t.Fatalf("subtypes %v: no Bear or Giant", tbl.Subtypes)
	}
	sc = newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	stone = sc.perm(id["Gathering Stone"], 0)
	return sc, id, stone, bear, giant
}

// TestPeekRevealChosenType: the may-reveal of Gathering Stone's
// RevealValid$ Card.ChosenType reads the type the stone's source permanent
// chose (CR 701.20a), not the resolution's own ChooseType answer.
func TestPeekRevealChosenType(t *testing.T) {
	sc, id, stone, bear, giant := gatheringStoneScen(t)
	pl := &sc.s.pl[0]
	sc.s.objs[pl.lib[int(pl.libLo)]].card = id["Grizzly Bears"]
	sc.s.res.e = stackEntry{kind: seTrigger, obj: stone, srcVer: sc.s.objs[stone].ver, card: id["Gathering Stone"], ctrl: 0}
	sc.s.res.active = true
	in := &kop.Instr{Op: kop.OpPeek, Obj: kop.RefController, A: kop.Const(1), C: kop.PeekMayReveal, F: kop.FilterID(4)}
	for _, c := range [...]struct {
		ch   uint8
		skip bool
	}{{bear, false}, {255, true}, {giant, true}} {
		sc.s.perms[sc.slot(stone)].chosen = objWord(c.ch)
		sc.s.res.ask = 0
		got := sc.s.peekMayReveal(sc.k, in, 0)
		switch {
		case got && !c.skip:
			t.Errorf("peekMayReveal posed no ask with the source having chosen Bear and a Bear on top (slot %d, bear sub %d)", sc.slot(stone), bear)
		case !got && c.skip:
			t.Errorf("peekMayReveal posed an ask with the source having chosen subtype %d and a Bear on top (bear sub %d)", c.ch, bear)
		}
	}
}

// TestCostChosenType: Gathering Stone's ReduceCost Card.ChosenType takes one
// off a spell of the type its permanent chose (CR 601.2f).
func TestCostChosenType(t *testing.T) {
	sc, id, stone, bear, giant := gatheringStoneScen(t)
	for _, c := range [...]struct {
		ch   uint8
		want uint8
	}{{bear, 1}, {255, 2}, {giant, 2}} {
		sc.s.perms[sc.slot(stone)].chosen = objWord(c.ch)
		if g := sc.s.adjustCostGen(sc.k, 0, id["Grizzly Bears"], NoObj, kop.ManaCost{Generic: 2}).Generic; g != c.want {
			t.Errorf("Grizzly Bears costs generic %d with a Gathering Stone that chose subtype %d (bear %d), want %d", g, c.ch, bear, c.want)
		}
	}
}

// TestPeekPublicEachPlayer: a public reveal over Defined$ Player (Ambassador
// K'Ehleyr's parley) reveals the top card of every player's library, the
// resolving controller first, and remembers both.
func TestPeekPublicEachPlayer(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	sc.k.t.Exprs = append(sc.k.t.Exprs, kop.Expr{Kind: kop.ExConst, N: 1})
	a := kop.ValueRef{K: kop.VExpr, N: int16(len(sc.k.t.Exprs) - 1)}
	sc.s.res.e.ctrl = 1
	in := kop.Instr{Op: kop.OpPeek, Obj: kop.RefEachPlayer, A: a, C: kop.PeekPublic | kop.PeekRemember}
	sc.s.g.nrem = 0
	sc.s.execGen(sc.k, &in)
	p0, p1 := &sc.s.pl[0], &sc.s.pl[1]
	if sc.s.g.nrem != 2 || sc.s.g.rem[0] != p1.lib[int(p1.libLo)] || sc.s.g.rem[1] != p0.lib[int(p0.libLo)] {
		t.Fatalf("remembered %d %v, want seat 1's top card then seat 0's", sc.s.g.nrem, sc.s.g.rem[:sc.s.g.nrem])
	}
}
