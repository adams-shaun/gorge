//go:build dirac_mp

package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// oppScen is a four-seat kernel state over the BLB table (Plains and a
// creature), with the seat gone (if any) out of the game, beside a gorge
// game of the same seating for its player orders.
func oppScen(t *testing.T, gone int) (*rules.Engine, *scen, map[string]CardID) {
	t.Helper()
	const n = 4
	land := cmdTestCard(t, "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n")
	names := make([]string, n)
	gd := make([][]*cards.Card, n)
	for p := 0; p < n; p++ {
		names[p] = fmt.Sprintf("p%d", p)
		for i := 0; i < 40; i++ {
			gd[p] = append(gd[p], land)
		}
	}
	e := rules.New(rules.Config{Seed: 1, Names: names, Decks: gd})
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}, {Name: "Treetop Sentries", Count: 1}})
	s := sc.s
	s.g.nseat = n
	for p := 0; p < n; p++ {
		s.pl[p].life = 20
	}
	if gone >= 0 {
		e.G.Players[gone].Lost = true
		s.pl[gone].lost = true
	}
	if len(s.cold.discHeld) == 0 {
		s.cold.discHeld = make([]uint8, discHeldLen)
	}
	return e, sc, id
}

func aliveFromGorge(e *rules.Engine, p, skip int) []uint8 {
	var o []uint8
	for _, q := range e.G.AliveFrom(state.PlayerID(p)) {
		if int(q) != skip {
			o = append(o, uint8(q))
		}
	}
	return o
}

// TestEachDiscardHoldsEverySeat: "each player discards N" where every hand
// goes whole holds each earlier player's hand until the last player's turn,
// then moves them all; at four seats every seat discards, not just the
// controller and the next.
func TestEachDiscardHoldsEverySeat(t *testing.T) {
	_, sc, id := oppScen(t, -1)
	s, k := sc.s, sc.k
	for p := uint8(0); p < 4; p++ {
		sc.hand(id["Plains"], p)
		sc.hand(id["Plains"], p)
	}
	s.res.e.ctrl = 2
	in := &kop.Instr{Op: kop.OpDiscard, Obj: kop.RefEachPlayer, A: kop.Const(2), C: kop.ChooseSelf}
	if !s.startEachDiscard(k, in, 2) {
		t.Fatal("a whole-hand discard posed an ask")
	}
	for p := 0; p < 4; p++ {
		if s.pl[p].handN != 0 {
			t.Fatalf("seat %d kept %d cards", p, s.pl[p].handN)
		}
	}
	for i := 0; i < len(s.cold.discHeld); i += discHeldBlock {
		if s.cold.discHeld[i] != 0 {
			t.Fatalf("held block %d not flushed", i/discHeldBlock)
		}
	}
}

// TestHeldDiscardBlocksFlushInOrder: three earlier players' choices (picks,
// a whole hand, picks) are held in separate blocks and flush in answer
// order, leaving every block empty.
func TestHeldDiscardBlocksFlushInOrder(t *testing.T) {
	_, sc, id := oppScen(t, -1)
	s, k := sc.s, sc.k
	a := sc.hand(id["Plains"], 2)
	keep := sc.hand(id["Plains"], 2)
	sc.hand(id["Plains"], 3)
	sc.hand(id["Plains"], 3)
	b := sc.hand(id["Plains"], 0)
	s.holdDiscard(2, []Obj{a})
	s.holdWhole(3)
	s.holdDiscard(0, []Obj{b})
	if s.cold.discHeld[2*discHeldBlock] != 1 {
		t.Fatal("the third player's picks did not get their own block")
	}
	s.flushHeldDiscard(k)
	if s.objs[a].zone != kop.ZoneGraveyard || s.objs[b].zone != kop.ZoneGraveyard || s.pl[3].handN != 0 {
		t.Fatal("a held discard did not move")
	}
	if s.objs[keep].zone != kop.ZoneHand {
		t.Fatal("an unpicked card moved")
	}
	for i := 0; i < len(s.cold.discHeld); i += discHeldBlock {
		if s.cold.discHeld[i] != 0 {
			t.Fatalf("held block %d not cleared", i/discHeldBlock)
		}
	}
}

// TestMayMillAsksEachSeat: an optional mill over each player asks every
// seat still in the game, in gorge's order (AliveFrom the controller).
func TestMayMillAsksEachSeat(t *testing.T) {
	for gone := -1; gone < 4; gone++ {
		for ctrl := 0; ctrl < 4; ctrl++ {
			if ctrl == gone {
				continue
			}
			e, sc, _ := oppScen(t, gone)
			s, k := sc.s, sc.k
			s.res.e.ctrl = uint8(ctrl)
			want := aliveFromGorge(e, ctrl, -1)
			if s.exec(k, &kop.Instr{Op: kop.OpMill, Obj: kop.RefEachPlayer, A: kop.Const(1), C: kop.MvMayMill}) {
				t.Fatal("no may-mill ask")
			}
			var got []uint8
			for i := range want {
				if s.res.ask != raGenMayMill {
					t.Fatalf("gone%d ctrl%d: ask %d after %v", gone, ctrl, s.res.ask, got)
				}
				got = append(got, s.res.askP)
				if i == len(want)-1 {
					break // the last "no" continues the resolution
				}
				s.applyAskCzParams(k, MakeAction(AkChoose, 0, 0, 0))
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("gone%d ctrl%d: asked %v, gorge %v", gone, ctrl, got, want)
			}
		}
	}
}

// TestDamageSeqEverySeat: the replacement path's each-player damage sequence
// deals every seat still in the game (a seat that has left is dealt none),
// and the each-opponent form every opponent of the controller.
func TestDamageSeqEverySeat(t *testing.T) {
	for gone := -1; gone < 4; gone++ {
		for ctrl := 0; ctrl < 4; ctrl++ {
			if ctrl == gone {
				continue
			}
			for _, opp := range []bool{false, true} {
				_, sc, _ := oppScen(t, gone)
				s, k := sc.s, sc.k
				s.active = uint8((ctrl + 1) % 4)
				s.res.e.ctrl = uint8(ctrl)
				s.damageSeqStart(k, slotMask{}, !opp)
				if opp {
					s.g.dmgSeqPl = seatBits(s.oppsFrom(uint8(ctrl)))
				}
				s.damageSeqNext(k, 3)
				if s.g.dmgSeqOn {
					t.Fatal("the sequence did not finish")
				}
				for p := 0; p < 4; p++ {
					hit := p != gone && !(opp && p == ctrl)
					if (s.pl[p].life == 17) != hit {
						t.Fatalf("gone%d ctrl%d opp%v: seat %d at %d life", gone, ctrl, opp, p, s.pl[p].life)
					}
				}
			}
		}
	}
}

// TestDamageAllEachOpponent: OpDamageAll's each-opponent bit deals every
// opponent of the controller, not one seat.
func TestDamageAllEachOpponent(t *testing.T) {
	for ctrl := 0; ctrl < 4; ctrl++ {
		_, sc, _ := oppScen(t, -1)
		s, k := sc.s, sc.k
		s.res.e.ctrl = uint8(ctrl)
		s.damageAllOpp(k, &kop.Instr{Op: kop.OpDamageAll, A: kop.Const(2), B: kop.Const(2)}, uint8(ctrl), 2)
		for p := 0; p < 4; p++ {
			if (s.pl[p].life == 18) != (p != ctrl) {
				t.Fatalf("ctrl%d: seat %d at %d life", ctrl, p, s.pl[p].life)
			}
		}
	}
}

// TestOneOpponentRefs: a single-player read of an opponent ref is the first
// of gorge's Defined$ Opponent list (AliveFrom the controller without it),
// as is the pile chooser; the not-remembered-controller set drops every
// remembered controller from that list.
func TestOneOpponentRefs(t *testing.T) {
	for gone := -1; gone < 4; gone++ {
		for ctrl := 0; ctrl < 4; ctrl++ {
			if ctrl == gone {
				continue
			}
			e, sc, id := oppScen(t, gone)
			s, k := sc.s, sc.k
			s.res.e.ctrl = uint8(ctrl)
			s.res.e.obj = NoObj
			opps := aliveFromGorge(e, ctrl, ctrl)
			for _, r := range []kop.Ref{kop.RefOpponent, kop.RefDefendingPlayer} {
				if got := s.playerRef(k, r); got != opps[0] {
					t.Fatalf("gone%d ctrl%d ref%d: %d, gorge %d", gone, ctrl, r, got, opps[0])
				}
			}
			if got := seatsOf(s.oppsNonRemCtrl()); got != fmt.Sprint(opps) {
				t.Fatalf("gone%d ctrl%d: nothing remembered %s, gorge %v", gone, ctrl, got, opps)
			}
			x := sc.perm(id["Plains"], opps[len(opps)-1])
			s.remember(x)
			if got := seatsOf(s.oppsNonRemCtrl()); got != fmt.Sprint(opps[:len(opps)-1]) {
				t.Fatalf("gone%d ctrl%d: remembered seat %d: %s", gone, ctrl, opps[len(opps)-1], got)
			}
		}
	}
}

// TestTargetedExilerAndMostCreatures: DefinedExiler$ Targeted names the
// targeted player, and "most creatures" compares against every other seat.
func TestTargetedExilerAndMostCreatures(t *testing.T) {
	_, sc, id := oppScen(t, -1)
	s, k := sc.s, sc.k
	s.res.e.ctrl = 0
	s.res.e.tgt[0] = TCode(PlayerCode(2))
	if got := s.targetedExiler(0); got != 2 {
		t.Fatalf("exiler %d, want the targeted seat 2", got)
	}
	sc.perm(id["Treetop Sentries"], 3)
	if s.mostCreatures(k, 0) || s.mostCreatures(k, 1) || !s.mostCreatures(k, 3) {
		t.Fatal("most creatures misread a seat beyond the first opponent")
	}
}
