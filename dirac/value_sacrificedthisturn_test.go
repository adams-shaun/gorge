package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSacrificedThisTurnTally: the per-player tallies
// behind ExSacrificedThisTurn are written at the one sacrifice choke point.
func TestSacrificedThisTurnTally(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature},
		kop.CardDef{Name: "Test Bauble", Types: kop.TArtifact})
	bauble := bear + 1
	sc := newScenTable(t, tbl, Config{NoElide: true})
	read := func(p uint8, n int) int {
		x := &kop.Expr{Kind: kop.ExSacrificedThisTurn, N: int16(n)}
		return sc.s.exprCold(sc.k, x, evalCtx{ctrl: p})
	}
	want := func(what string, p0all, p0art, p1all, p1art int) {
		t.Helper()
		got := [4]int{read(0, 0), read(0, 1), read(1, 0), read(1, 1)}
		if got != [4]int{p0all, p0art, p1all, p1art} {
			t.Fatalf("%s: tallies %v, want %v", what, got, [4]int{p0all, p0art, p1all, p1art})
		}
	}
	want("start", 0, 0, 0, 0)

	// one fireSac: the controller's all-tally; an artifact adds the artifact tally
	b0 := sc.perm(bear, 0)
	sc.s.fireSac(sc.k, sc.slot(b0))
	want("a creature", 1, 0, 0, 0)
	a0 := sc.perm(bauble, 0)
	sc.s.fireSac(sc.k, sc.slot(a0))
	want("an artifact", 2, 1, 0, 0)

	// fireSacAll over two permanents with no TrigSacrificed listener (the
	// OpSacrificeAll bypass): one count per object, each to its controller
	if sc.s.subs[kop.TrigSacrificed].any() {
		t.Fatal("a TrigSacrificed listener exists; the scenario needs none")
	}
	x0, x1 := sc.perm(bauble, 0), sc.perm(bear, 1)
	sc.s.fireSacAll(sc.k, slotMask{}.with(sc.slot(x0)).with(sc.slot(x1)))
	want("fireSacAll", 3, 2, 1, 0)

	// a stolen permanent counts for the player who controls it, not its owner
	stolen := sc.perm(bauble, 0)
	sc.s.ctrlMove(sc.slot(stolen), 1)
	sc.s.fireSac(sc.k, sc.slot(stolen))
	want("a stolen artifact", 3, 2, 2, 1)

	// a token copy is skipped
	cp := sc.perm(bauble, 0)
	sc.s.objs[cp].of |= ofToken | ofCopy
	sc.s.fireSac(sc.k, sc.slot(cp))
	want("a token copy", 3, 2, 2, 1)
	// a plain token (a Clue) is a permanent and counts
	tok := sc.perm(bauble, 0)
	sc.s.objs[tok].of |= ofToken
	sc.s.fireSac(sc.k, sc.slot(tok))
	want("a token", 4, 3, 2, 1)

	// saturation
	sc.s.pl[0].sac = [3]uint8{255, 255, 0}
	a1 := sc.perm(bauble, 0)
	sc.s.fireSac(sc.k, sc.slot(a1))
	if sc.s.pl[0].sac != [3]uint8{255, 255, 0} {
		t.Fatalf("sac %v after a 256th sacrifice, want 255 each", sc.s.pl[0].sac)
	}

	// the turn start zeroes both players
	sc.s.beginTurn(1)
	want("next turn", 0, 0, 0, 0)
}
