package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// lostSeatWightScen is a four-seat state in which seat 0 controls a Wight of
// Precinct Six (1/1 plus the creature cards in its opponents' graveyards)
// and a Souldrinker, in that battlefield order, both with damage marked
// (the Souldrinker's is lethal, the Wight's falls between its toughness with
// and without seat 1's two graveyard creatures). Seat 1's graveyard holds
// two Wights.
func lostSeatWightScen(t *testing.T) (sc *scen, wight, other Obj) {
	t.Helper()
	sc, id := newSetScenTweak(t, "mbc", []cardset.Entry{
		{Name: "Wight of Precinct Six", Count: 6}, {Name: "Souldrinker", Count: 1}, {Name: "Plains", Count: 8},
	}, nil)
	s := sc.s
	s.g.nseat = 4
	for p := uint8(0); p < 4; p++ {
		s.pl[p].life = 20
	}
	if len(s.cold.discHeld) == 0 {
		s.cold.discHeld = make([]uint8, discHeldLen)
	}
	wight = sc.perm(id["Wight of Precinct Six"], 0)
	other = sc.perm(id["Souldrinker"], 0)
	sc.grave(id["Wight of Precinct Six"], 1, 2)
	s.perms[sc.slot(wight)].damage = 2
	s.perms[sc.slot(other)].damage = 200
	return sc, wight, other
}

// lostSeatGraveOrder is seat 0's graveyard after the pass: the Wight's place
// among the casualties says which pass killed it.
func lostSeatGraveOrder(sc *scen, wight, other Obj) (wi, oi int) {
	wi, oi = -1, -1
	pl := &sc.s.pl[0]
	for i := 0; i < int(pl.graveN); i++ {
		switch pl.grave[i] {
		case wight:
			wi = i
		case other:
			oi = i
		}
	}
	return wi, oi
}

// TestLostSeatGraveyardCountsOnlyForItsOwnPass: a seat out of the game counts
// nothing for a graveyard count (gorge's AliveFrom walk), except a seat the
// running state-based pass has just lost: gorge queues the pass's losses in
// its batch and marks them when it applies, after the scan, so that scan still
// counts the seat's graveyard (CR 704.3, 800.4a).
//
//   - Lost to an effect before the pass: the Wight is 1/1 in the pass's scan
//     and dies in its batch, ahead of the Souldrinker behind it.
//   - Lost by life in the pass: the Wight is 3/3 in the scan and survives
//     the pass's batch; only the next pass, after the sweep, kills it.
func TestLostSeatGraveyardCountsOnlyForItsOwnPass(t *testing.T) {
	t.Run("effect", func(t *testing.T) {
		sc, wight, other := lostSeatWightScen(t)
		sc.s.pl[1].lost = true
		if !sc.s.out(1) {
			t.Fatal("precondition: seat 1 is not out of the game")
		}
		sc.s.sba(sc.k)
		wi, oi := lostSeatGraveOrder(sc, wight, other)
		if wi < 0 || oi < 0 {
			t.Fatalf("the Wight (%d) and the Souldrinker (%d) must both be in seat 0's graveyard", wi, oi)
		}
		if wi > oi {
			t.Fatalf("the Wight went to the graveyard after the Souldrinker (positions %d, %d): the effect-lost seat's graveyard counted in the pass's scan", wi, oi)
		}
	})
	t.Run("pass", func(t *testing.T) {
		sc, wight, other := lostSeatWightScen(t)
		sc.s.pl[1].life = 0
		sc.s.sba(sc.k)
		wi, oi := lostSeatGraveOrder(sc, wight, other)
		if wi < 0 || oi < 0 {
			t.Fatalf("the Wight (%d) and the Souldrinker (%d) must both be in seat 0's graveyard", wi, oi)
		}
		if wi < oi {
			t.Fatalf("the Wight went to the graveyard with the Souldrinker (positions %d, %d): the pass's own loss did not count in its scan", wi, oi)
		}
	})
}
