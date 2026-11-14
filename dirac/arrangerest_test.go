package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// arrangeRestRun drives a scry or surveil over the top four cards of seat
// 0's library through the resolve ask: picks answers each ChArrange decision
// (-1: AkDone, else the index into the looked-at cards), and it returns the
// four looked-at cards plus the number of decisions posed.
func arrangeRestRun(t *testing.T, op kop.Op, rest bool, picks []int) (*scen, [4]Obj, int) {
	t.Helper()
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Rabbit Response", Count: 1}, {Name: "Plains", Count: 8}})
	s := sc.s
	if rest {
		s.cfgx |= cfgxArrangeRest
	}
	rr := id["Rabbit Response"]
	in := firstInstr(t, sc, rr, kop.OpScry)
	in.Op, in.A = op, kop.Const(4)
	for ai, ab := range sc.k.t.Cards[rr].Abilities {
		for pc, x := range ab.Prog {
			if x.Op == kop.OpScry {
				s.res.e.card, s.res.e.ab, s.res.pc = rr, uint8(ai), uint8(pc)
			}
		}
	}
	pl := &s.pl[0]
	var look [4]Obj
	for i := range look {
		look[i] = pl.lib[int(pl.libLo)+i]
	}
	s.res.e.ctrl = 0
	if s.exec(sc.k, &in) {
		t.Fatal("the arrange did not suspend on a decision")
	}
	posed := 0
	for _, pick := range picks {
		var d Decision
		if !s.resolveOptions(sc.k, &d) || d.Sub != ChArrange {
			t.Fatalf("decision %d: sub %d, want ChArrange", posed, d.Sub)
		}
		posed++
		a := MakeAction(AkDone, 0, 0, 0)
		if pick >= 0 {
			a = MakeAction(AkChoose, TCode(look[pick]), 0, 0)
		}
		found := false
		for i := 0; i < int(d.N); i++ {
			found = found || d.Options[i] == a
		}
		if !found {
			t.Fatalf("decision %d: answer %d not offered (%v)", posed, pick, d.Options[:d.N])
		}
		s.applyResolve(sc.k, a)
	}
	return sc, look, posed
}

// TestArrangeRestOrder: without Config.ArrangeRestOrder the unpicked
// cards of a surveil go to the graveyard (a scry, the bottom) in library
// order; with it, AkDone opens a pile-B order phase (all but the last card
// picked) and the graveyard / bottom takes them in that order (CR 404.3).
func TestArrangeRestOrder(t *testing.T) {
	// the top phase: pick look[2] to keep on top, then Done. Cards 0, 1, 3
	// remain.
	t.Run("surveil default: library order", func(t *testing.T) {
		sc, look, posed := arrangeRestRun(t, kop.OpSurveil, false, []int{2, -1})
		pl := &sc.s.pl[0]
		if posed != 2 || pl.graveN != 3 || pl.grave[0] != look[0] || pl.grave[1] != look[1] || pl.grave[2] != look[3] {
			t.Fatalf("posed %d, graveyard %v (%d), want library order %v", posed, pl.grave[:pl.graveN], pl.graveN, look)
		}
		if pl.lib[pl.libLo] != look[2] {
			t.Fatal("the kept card is not on top")
		}
	})
	t.Run("surveil rest order", func(t *testing.T) {
		// rest cards 0, 1, 3: order 3, 0 then the implied 1
		sc, look, posed := arrangeRestRun(t, kop.OpSurveil, true, []int{2, -1, 3, 0})
		pl := &sc.s.pl[0]
		if posed != 4 || sc.s.res.ask != raNone {
			t.Fatalf("posed %d decisions, ask %d: the rest phase should close after n-1 picks", posed, sc.s.res.ask)
		}
		if pl.graveN != 3 || pl.grave[0] != look[3] || pl.grave[1] != look[0] || pl.grave[2] != look[1] {
			t.Fatalf("graveyard %v, want [%d %d %d]", pl.grave[:pl.graveN], look[3], look[0], look[1])
		}
		if pl.lib[pl.libLo] != look[2] {
			t.Fatal("the kept card is not on top")
		}
		if sc.s.objs[look[3]].zone != kop.ZoneGraveyard {
			t.Fatal("a rest card is not in the graveyard zone")
		}
	})
	t.Run("scry rest order", func(t *testing.T) {
		sc, look, _ := arrangeRestRun(t, kop.OpScry, true, []int{-1, 3, 1, 0})
		pl := &sc.s.pl[0]
		// no top picks: the whole look is pile B, ordered 3, 1, 0 (then 2)
		bot := pl.lib[int(pl.libHi)-4 : pl.libHi]
		want := [4]Obj{look[3], look[1], look[0], look[2]}
		if [4]Obj(bot) != want {
			t.Fatalf("bottom %v, want %v", bot, want)
		}
	})
	t.Run("scry default: library order", func(t *testing.T) {
		sc, look, _ := arrangeRestRun(t, kop.OpScry, false, []int{1, -1})
		pl := &sc.s.pl[0]
		bot := pl.lib[int(pl.libHi)-3 : pl.libHi]
		if [3]Obj(bot) != [3]Obj{look[0], look[2], look[3]} || pl.lib[pl.libLo] != look[1] {
			t.Fatalf("bottom %v top %d, want library order of the unpicked", bot, pl.lib[pl.libLo])
		}
	})
	t.Run("one card left is not asked", func(t *testing.T) {
		sc, look, posed := arrangeRestRun(t, kop.OpSurveil, true, []int{0, 1, 2, -1})
		pl := &sc.s.pl[0]
		if posed != 4 || sc.s.res.ask != raNone || pl.graveN != 1 || pl.grave[0] != look[3] {
			t.Fatalf("posed %d ask %d graveyard %v", posed, sc.s.res.ask, pl.grave[:pl.graveN])
		}
	})
	t.Run("every card kept closes the decision", func(t *testing.T) {
		sc, _, posed := arrangeRestRun(t, kop.OpSurveil, true, []int{3, 2, 1, 0})
		if posed != 4 || sc.s.res.ask != raNone || sc.s.pl[0].graveN != 0 {
			t.Fatalf("posed %d ask %d graveyard %d", posed, sc.s.res.ask, sc.s.pl[0].graveN)
		}
	})
	t.Run("config sets the bit", func(t *testing.T) {
		if sc := newScen(t, Config{NoElide: true, ArrangeRestOrder: true}); sc.s.cfgx&cfgxArrangeRest == 0 {
			t.Fatal("Config.ArrangeRestOrder did not set cfgxArrangeRest")
		}
		if sc := newScen(t, Config{NoElide: true}); sc.s.cfgx&cfgxArrangeRest != 0 {
			t.Fatal("cfgxArrangeRest set by default")
		}
	})
}
