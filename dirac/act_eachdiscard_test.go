package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// eachDiscardScen builds a scenario whose stack holds a test sorcery with
// one OpDiscard obj=EachPlayer A=2 prog, resolving for controller ctrl.
func eachDiscardScen(t *testing.T, ctrl uint8) *scen {
	t.Helper()
	return eachDiscardScenN(t, ctrl, 2)
}

// eachDiscardScenN is eachDiscardScen with an explicit NumCards$ (an X=0
// each-player discard exercises gorge's discardBounds floor).
func eachDiscardScenN(t *testing.T, ctrl uint8, a int) *scen {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Each Discard", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpDiscard, Obj: kop.RefEachPlayer, A: kop.Const(a), C: kop.ChooseSelf}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o := sc.obj(src, ctrl)
	sc.s.objs[o].zone = kop.ZoneStack
	sc.s.push(stackEntry{kind: seSpell, obj: o, ctrl: ctrl, card: src, trigObj: NoObj})
	sc.s.res.e = stackEntry{kind: seSpell, obj: o, ctrl: ctrl, card: src, trigObj: NoObj}
	sc.s.res.pc = 0
	return sc
}

// TestEachDiscardAsksBothPlayers: both seats hold three cards, so each is
// asked for two picks in turn (controller first), and each answers before
// the other is asked; both hands end with one card.
func TestEachDiscardAsksBothPlayers(t *testing.T) {
	sc := eachDiscardScen(t, 0)
	h := [2][3]Obj{}
	for p := uint8(0); p < 2; p++ {
		for i := range h[p] {
			h[p][i] = sc.hand(v.Plains, p)
		}
	}
	sc.s.runProg(sc.k)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChDiscard || d.Player != 0 {
		t.Fatalf("first ask kind %d sub %d player %d, want DkChoose ChDiscard 0", d.Kind, d.Sub, d.Player)
	}
	sc.do(t, AkChoose, TCode(h[0][0]), scAny)
	sc.do(t, AkChoose, TCode(h[0][1]), scAny)
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChDiscard || d.Player != 1 {
		t.Fatalf("second ask kind %d sub %d player %d, want DkChoose ChDiscard 1", d.Kind, d.Sub, d.Player)
	}
	sc.do(t, AkChoose, TCode(h[1][0]), scAny)
	sc.do(t, AkChoose, TCode(h[1][1]), scAny)
	if sc.s.pl[0].handN != 1 || sc.s.pl[1].handN != 1 {
		t.Fatalf("hands %d/%d, want 1/1", sc.s.pl[0].handN, sc.s.pl[1].handN)
	}
	if sc.s.objs[h[0][0]].zone != kop.ZoneGraveyard || sc.s.objs[h[1][0]].zone != kop.ZoneGraveyard {
		t.Fatalf("picked cards not in the graveyard")
	}
	if sc.s.objs[h[0][2]].zone != kop.ZoneHand || sc.s.objs[h[1][2]].zone != kop.ZoneHand {
		t.Fatalf("unpicked cards not in hand")
	}
}

// TestEachDiscardWholeHandNoAsk: a player whose hand is exactly N poses no ask
// (gorge's askMin == askMax == len(eligible)), and the chain moves to the next
// seat, who chooses with those cards still in hand (CR 101.4).
func TestEachDiscardWholeHandNoAsk(t *testing.T) {
	sc := eachDiscardScen(t, 0)
	first := sc.hand(v.Plains, 0)
	second := sc.hand(v.Plains, 0)
	var other [3]Obj
	for i := range other {
		other[i] = sc.hand(v.Plains, 1)
	}
	sc.s.runProg(sc.k)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChDiscard || d.Player != 1 {
		t.Fatalf("first ask kind %d sub %d player %d, want the other seat", d.Kind, d.Sub, d.Player)
	}
	if sc.s.pl[0].handN != 2 {
		t.Fatalf("player 0 hand %d during the other seat's ask, want its 2 cards held", sc.s.pl[0].handN)
	}
	sc.do(t, AkChoose, TCode(other[0]), scAny)
	sc.do(t, AkChoose, TCode(other[1]), scAny)
	if sc.s.objs[first].zone != kop.ZoneGraveyard || sc.s.objs[second].zone != kop.ZoneGraveyard {
		t.Fatalf("player 0's two cards not discarded after the last answer")
	}
}

// TestEachDiscardXZeroOneCardHand: gorge's discardBounds keeps a resolved
// zero at zero (agent-20261027T055350Z-7faa6715), so an X=0 each-player
// discard (Death Cloud) discards nothing and poses no ask, whatever the hand.
func TestEachDiscardXZeroOneCardHand(t *testing.T) {
	sc := eachDiscardScenN(t, 0, 0)
	h := [2]Obj{}
	for p := uint8(0); p < 2; p++ {
		h[p] = sc.hand(v.Plains, p)
	}
	sc.s.runProg(sc.k)
	if d := sc.s.Step(); d.Sub == ChDiscard {
		t.Fatalf("discard ask kind %d player %d, want none (gorge asks nothing for a zero)", d.Kind, d.Player)
	}
	if sc.s.pl[0].handN != 1 || sc.s.pl[1].handN != 1 {
		t.Fatalf("hands %d/%d, want 1/1", sc.s.pl[0].handN, sc.s.pl[1].handN)
	}
	if sc.s.objs[h[0]].zone != kop.ZoneHand || sc.s.objs[h[1]].zone != kop.ZoneHand {
		t.Fatalf("an X=0 discard moved a card")
	}
}

// TestEachDiscardEmptyHandSkipped: an empty hand is skipped, and the seat
// with cards is asked.
func TestEachDiscardEmptyHandSkipped(t *testing.T) {
	sc := eachDiscardScen(t, 0)
	for i := 0; i < 3; i++ {
		sc.hand(v.Plains, 1)
	}
	sc.s.runProg(sc.k)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChDiscard || d.Player != 1 {
		t.Fatalf("first ask kind %d sub %d player %d, want the non-empty seat", d.Kind, d.Sub, d.Player)
	}
}

// TestEachDiscardControllerFirst: with the controller as seat 1, the order
// follows the controller (1 then 0), not seat order.
func TestEachDiscardControllerFirst(t *testing.T) {
	sc := eachDiscardScen(t, 1)
	h := [2][3]Obj{}
	for p := uint8(0); p < 2; p++ {
		for i := range h[p] {
			h[p][i] = sc.hand(v.Plains, p)
		}
	}
	sc.s.runProg(sc.k)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChDiscard || d.Player != 1 {
		t.Fatalf("first ask kind %d sub %d player %d, want the controller (1)", d.Kind, d.Sub, d.Player)
	}
	sc.do(t, AkChoose, TCode(h[1][0]), scAny)
	sc.do(t, AkChoose, TCode(h[1][1]), scAny)
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChDiscard || d.Player != 0 {
		t.Fatalf("second ask kind %d sub %d player %d, want the other seat (0)", d.Kind, d.Sub, d.Player)
	}
	sc.do(t, AkChoose, TCode(h[0][0]), scAny)
	sc.do(t, AkChoose, TCode(h[0][1]), scAny)
	if sc.s.pl[0].handN != 1 || sc.s.pl[1].handN != 1 {
		t.Fatalf("hands %d/%d, want 1/1", sc.s.pl[0].handN, sc.s.pl[1].handN)
	}
}
