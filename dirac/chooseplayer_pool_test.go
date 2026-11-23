package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// choosePodScen is a four-seat game over the cursecurses table (Archnemesis,
// an Enchant:Opponent aura, is the source the random pick reads its
// attachment from), seat ctrl resolving an ability of that aura.
func choosePodScen(t *testing.T, ctrl uint8) (*scen, Slot) {
	t.Helper()
	set := cardsettest.Set(t, "cursecurses")
	deck := []cardset.Entry{{Name: "Archnemesis", Count: 4}, {Name: "Island", Count: 36}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cd := tbl.Cards[id["Archnemesis"]]
	cd.Abilities = nil
	tbl.Cards[id["Archnemesis"]] = cd
	var decks [MaxSeats][]CardID
	for p := range decks {
		decks[p] = da
	}
	s := new(State)
	s.InitN(tbl, decks, 4, 1, Config{NoElide: true})
	sc := &scen{s: s, k: s.k()}
	o := sc.perm(id["Archnemesis"], ctrl)
	sl := s.objs[o].slot
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: o, srcVer: s.objs[o].ver,
		card: s.objs[o].card, ab: 0, ctrl: ctrl, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	return sc, sl
}

// poolSeats is the seats of a pool in its order.
func poolSeats(q seatSeq) []uint8 {
	var seats []uint8
	for i := uint8(0); i < q.n; i++ {
		seats = append(seats, q.s[i])
	}
	return seats
}

func poolSeatsEqual(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestChooseFromSetPod: the controller's player asks at four seats list the
// pool from the controller in seat order (gorge effChoosePlayer walks
// AliveFrom(chooser)): the opponents-only ask is the three opponents, and the
// any-player ask (Valleymaker, Choices$ absent) is the controller and then
// the three opponents. A player the pool leaves out of the ask is a lost one.
func TestChooseFromSetPod(t *testing.T) {
	sc, _ := choosePodScen(t, 1)
	s := sc.s
	mid := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtChoosePlayerMid))}
	if s.exec(sc.k, &mid) {
		t.Fatal("the opponents ask was answered unasked at four seats")
	}
	if s.res.ask != raChoosePlayerMid {
		t.Fatalf("the program stopped at ask %d, want the opponents ask", s.res.ask)
	}
	if got, want := poolSeats(s.choosePlayerMidPool(1, 0)), []uint8{2, 3, 0}; !poolSeatsEqual(got, want) {
		t.Errorf("the opponents ask lists %v, want %v", got, want)
	}

	// Choices$ Player.Opponent+lifeEQX (the highest life among the opponents):
	// seats 3 and 0 tie at 25, seat 2 has 20, and the controller's own 40 is
	// not an opponent's. The pool keeps the tied ones in seat order from the
	// controller; a lone leader is recorded unasked and a dead one is gone.
	s.pl[1].life, s.pl[2].life, s.pl[3].life, s.pl[0].life = 40, 20, 25, 25
	if got, want := poolSeats(s.choosePlayerMidPool(1, kop.ChoosePlayerMidHigh)), []uint8{3, 0}; !poolSeatsEqual(got, want) {
		t.Errorf("the highest-life ask lists %v, want %v", got, want)
	}
	s.pl[0].life = 24
	if got, want := poolSeats(s.choosePlayerMidPool(1, kop.ChoosePlayerMidHigh)), []uint8{3}; !poolSeatsEqual(got, want) {
		t.Errorf("a lone leader's pool is %v, want %v", got, want)
	}
	s.res.ask, s.res.chPl = raNone, 0
	high := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtChoosePlayerMid)), C: kop.ChoosePlayerMidHigh}
	if !s.exec(sc.k, &high) || s.res.chPl != 4 {
		t.Errorf("a lone leader: asked or chPl %d, want it recorded unasked as seat 3", s.res.chPl)
	}
	s.pl[3].lost = true
	s.res.chPl = 0
	if !s.exec(sc.k, &high) || s.res.chPl != 1 {
		t.Errorf("the leader lost: chPl %d, want the next highest (seat 0, 24) unasked", s.res.chPl)
	}

	sc, _ = choosePodScen(t, 1)
	s = sc.s
	res := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtChoosePlayerRes)), C: kop.ChoosePlayerForget}
	if s.exec(sc.k, &res) {
		t.Fatal("the any-player ask was answered unasked at four seats")
	}
	if got, want := poolSeats(s.choosePlayerResPool(&res)), []uint8{1, 2, 3, 0}; !poolSeatsEqual(got, want) {
		t.Errorf("the any-player ask lists %v, want %v", got, want)
	}
	// the answer is the chosen player but, with no RememberChosen$, not a
	// remembered one (a later Player.!IsRemembered pool does not leave it out)
	s.choosePlayerResRecord(3, res.C)
	if got := s.chosenSeatRes(); got != 3 {
		t.Errorf("the chosen seat is %d, want 3", got)
	}
	if s.res.cpl&cplRemMask != 0 {
		t.Errorf("the choice joined the remembered players (%#x) without RememberChosen$", s.res.cpl&cplRemMask)
	}

	// control: with the remember bit clear the answer is remembered
	s.res.cpl = 0
	s.choosePlayerResRecord(3, 0)
	if s.res.cpl&cplRemMask != 1<<3 {
		t.Errorf("the remembered mask is %#x, want seat 3's bit", s.res.cpl&cplRemMask)
	}
}

// TestChoosePlayerRandomPod: Random$ True over plain Choices$ Player.Opponent
// at four seats picks one of the three opponents in seat order from the
// controller with one rng draw (gorge randomChoices), and an Aura the source
// is attached to does not leave its player out of the pool (only Choices$
// Player.Opponent+!EnchantedBy does: Maddening Hex).
func TestChoosePlayerRandomPod(t *testing.T) {
	picks := func(c int16) (map[uint8]int, uint64) {
		sc, sl := choosePodScen(t, 1)
		s := sc.s
		s.attachPlayer(sc.k, sl, 2) // seat 2 is the aura's bearer
		in := kop.Instr{Op: kop.OpChoosePlayer, Obj: kop.RefController, C: c}
		seen := map[uint8]int{}
		start := s.rng.Draws
		for i := 0; i < 60; i++ {
			s.res.chPl = 0
			r := s.rng // a copy: the draw the pick must make
			if !s.exec(sc.k, &in) {
				t.Fatal("the random pick posed an ask")
			}
			if s.res.chPl == 0 {
				t.Fatal("no pick recorded from a non-empty pool")
			}
			pool := []uint8{2, 3, 0}
			if c&kop.ChPlPlain == 0 {
				pool = []uint8{3, 0}
			}
			if want := pool[r.IntN(len(pool))] + 1; s.res.chPl != want {
				t.Fatalf("draw %d picked chPl %d, want %d from pool %v", i, s.res.chPl, want, pool)
			}
			seen[s.res.chPl-1]++
		}
		return seen, s.rng.Draws - start
	}
	hex, n := picks(kop.ChPlRandom)
	if n != 60 || hex[2] != 0 {
		t.Errorf("Maddening Hex's pool: %d draws, seat 2 picked %d times; want 60 draws and the bearer left out", n, hex[2])
	}
	plain, n := picks(kop.ChPlRandom | kop.ChPlPlain)
	if n != 60 {
		t.Errorf("%d draws, want one each", n)
	}
	if plain[2] == 0 || plain[3] == 0 || plain[0] == 0 {
		t.Errorf("picks %v: want all three opponents reached (the bearer is a plain opponent)", plain)
	}
}
