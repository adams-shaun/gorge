package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sacParkLinkScen: player 0 controls two Leylines of the Void (so player 1's
// creatures' moves to the graveyard are CR 616.1 order asks) and the table
// holds an until-leaves program, so holdOpen opens the departure frame.
func sacParkLinkScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Grizzly Bears", Count: 3},
		{Name: "Plains", Count: 8},
	}, func(tbl *kop.Table, id map[string]CardID) {
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Link", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell,
				Prog: []kop.Instr{{Op: kop.OpExileUntilLeaves, Obj: kop.RefTarget0}}}}})
	})
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	if !sc.k.gen.anyLinkRet {
		t.Fatal("the table has no until-leaves program")
	}
	return sc, id
}

// TestSacParkLinkMovedFirstStalls: an each-player edict sacrifices player 0's
// Bears (its move does not compete: it goes now) and player 1's Bears (parked
// for its order ask). Player 0's Bears exiled a card until it leaves; gorge
// returns that card after the whole flush (CR 610.3d), the parked move
// included, which the kernel cannot place once the frame closes, so the
// flush stalls by name rather than return the card between the moves.
func TestSacParkLinkMovedFirstStalls(t *testing.T) {
	sc, id := sacParkLinkScen(t)
	s, k := sc.s, sc.k
	mine := sc.perm(id["Grizzly Bears"], 0)
	theirs := sc.perm(id["Grizzly Bears"], 1)
	held := exiledUntilLeaves(sc, mine, id["Plains"], 1)
	if s.graveMoveCompetes(k, mine) || !s.graveMoveCompetes(k, theirs) {
		t.Fatal("want player 0's move plain and player 1's competing")
	}
	s.npend = 0
	s.addSac(sc.slot(mine))
	s.addSac(sc.slot(theirs))
	s.flushSac(k)
	if !s.Stalled() || s.StallReason() != StallReplLeave {
		t.Fatalf("stalled %v reason %s, held card zone %d, parked %v; want StallReplLeave",
			s.Stalled(), StallName(s.StallReason()), s.objs[held].zone, s.parkedWaiting())
	}
}

// TestSacParkLinkParkedFirstStalls: two of player 1's Bears are sacrificed
// together, both parked for their order asks, and the first exiled a card
// until it leaves. Gorge returns it after both moves; the kernel would return
// it between them, so the flush stalls by name.
func TestSacParkLinkParkedFirstStalls(t *testing.T) {
	sc, id := sacParkLinkScen(t)
	s, k := sc.s, sc.k
	a := sc.perm(id["Grizzly Bears"], 1)
	b := sc.perm(id["Grizzly Bears"], 1)
	held := exiledUntilLeaves(sc, a, id["Plains"], 0)
	if !s.graveMoveCompetes(k, a) || !s.graveMoveCompetes(k, b) {
		t.Fatal("want both moves competing")
	}
	s.npend = 0
	s.addSac(sc.slot(a))
	s.addSac(sc.slot(b))
	s.flushSac(k)
	if !s.Stalled() || s.StallReason() != StallReplLeave {
		t.Fatalf("stalled %v reason %s, held card zone %d, parked %v; want StallReplLeave",
			s.Stalled(), StallName(s.StallReason()), s.objs[held].zone, s.parkedWaiting())
	}
}
