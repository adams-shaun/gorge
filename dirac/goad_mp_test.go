//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// goadScen is a four-seat kernel state over the goad pool's table: a Goblin
// Racketeer in the matchup so the table carries an OpGoad program and the
// per-seat goad mechs take their arena indices (stallclass.go prepGoad no
// longer refuses a program-goad table at more than two seats), Grizzly Bears
// as the creatures, Plains for the scenario deck. The seat gone (if any) is
// out of the game.
func goadScen(t *testing.T, gone int) (*scen, map[string]CardID) {
	t.Helper()
	sc, id := newSetScenTweak(t, "goad", []cardset.Entry{
		{Name: "Goblin Racketeer", Count: 1}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8},
	}, nil)
	sc.s.g.nseat = 4
	for p := uint8(0); p < 4; p++ {
		sc.s.pl[p].life = 20
	}
	if gone >= 0 {
		sc.s.pl[gone].lost = true
	}
	if len(sc.s.cold.discHeld) == 0 {
		sc.s.cold.discHeld = make([]Obj, discHeldLen)
	}
	return sc, id
}

// goadOffers runs the active player's attackers decision and returns, in
// offer order, the seats the creature o is offered to attack (its player
// offers only; the goad drops planeswalkers and battles, CR 508.1d).
func (sc *scen) goadOffers(o Obj) []uint8 {
	d := new(Decision)
	sc.s.attackerOptions(sc.k, d)
	var out []uint8
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkAttack && a.Source() == TCode(uint8(o)) && isPlayerTarget(a.Target()) {
			out = append(out, CodeSeat(a.Target()))
		}
	}
	return out
}

// sameSeats compares two seat lists as sets.
func sameSeats(got, want []uint8) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestGoadFourSeatDefenders: a goaded creature at four seats attacks players
// only, and a player who goaded it is offered as defender only when every
// other live player goaded it too (CR 701.15b, gorge combat.GoadMayAttack):
// goaded by seat 1, the offers are the two live non-goaders; with seats 2
// and 3 out of the game the goader is the only defender and is offered;
// goaded by seats 1, 2 and 3, every live player is offered, because no
// defender choice can satisfy every goad and each stays legal.
func TestGoadFourSeatDefenders(t *testing.T) {
	sc, id := goadScen(t, -1)
	s, k := sc.s, sc.k
	bear := sc.perm(id["Grizzly Bears"], 0)
	sl := sc.slot(bear)
	if !goadOn(k) {
		t.Fatal("the goad table carries no goad bookkeeping: the goad set changed")
	}
	offers := sc.goadOffers(bear)
	if !sameSeats(offers, []uint8{1, 2, 3}) {
		t.Fatalf("an ungoaded creature is offered every live opponent, got %v", offers)
	}
	s.goadSet(k, sl, 1)
	if !s.goadedAt(k, sl) || !s.mSl(k, goadMech(1), sl) || s.mSl(k, goadMech(2), sl) {
		t.Fatalf("seat 1's mark is not the only one on the creature: goaded %v m1 %v m2 %v",
			s.goadedAt(k, sl), s.mSl(k, goadMech(1), sl), s.mSl(k, goadMech(2), sl))
	}
	offers = sc.goadOffers(bear)
	if !sameSeats(offers, []uint8{2, 3}) {
		t.Fatalf("a creature goaded by seat 1 must attack the other live players, got %v", offers)
	}
	s.pl[2].lost, s.pl[3].lost = true, true
	offers = sc.goadOffers(bear)
	if !sameSeats(offers, []uint8{1}) {
		t.Fatalf("a creature goaded by the only other live player may attack its goader, got %v", offers)
	}
	s.pl[2].lost, s.pl[3].lost = false, false
	s.goadSet(k, sl, 2)
	s.goadSet(k, sl, 3)
	offers = sc.goadOffers(bear)
	if !sameSeats(offers, []uint8{1, 2, 3}) {
		t.Fatalf("a creature goaded by every other live player may attack any of them, got %v", offers)
	}
}

// TestGoadMarksEverySeat: each seat's goad is its own mark, ending as that
// seat's turn begins (gorge expireTurnGoads at TurnChange), not with
// another seat's, and it stays while the goader is out of the game (gorge
// pruneGoads ignores the loss; expireTurnGoads only fires on the goader's
// own turn).
func TestGoadMarksEverySeat(t *testing.T) {
	sc, id := goadScen(t, -1)
	s, k := sc.s, sc.k
	a := sc.perm(id["Grizzly Bears"], 0)
	b := sc.perm(id["Grizzly Bears"], 0)
	sa, sb := sc.slot(a), sc.slot(b)
	if goadMech(3) == goadMech(1) {
		t.Fatal("seats 1 and 3 share a goad mech: the marks would alias")
	}
	s.goadSet(k, sa, 3)
	s.goadSet(k, sb, 1)
	if !s.goadedAt(k, sa) || !s.goadedAt(k, sb) {
		t.Fatal("a seat's goad mark did not land")
	}
	s.goadTurn(k, 1) // seat 1's turn begins
	if s.goadedAt(k, sb) {
		t.Fatal("seat 1's goad outlived seat 1's turn")
	}
	if !s.goadedAt(k, sa) {
		t.Fatal("seat 3's goad ended at seat 1's turn start")
	}
	s.pl[3].lost = true
	s.goadTurn(k, 2)
	if !s.goadedAt(k, sa) {
		t.Fatal("a departed goader's goad was cleared on another seat's turn")
	}
	if offers := sc.goadOffers(a); !sameSeats(offers, []uint8{1, 2}) {
		t.Fatalf("a creature goaded by a lost seat 3 still attacks players other than the live goaders, got %v", offers)
	}
	s.goadTurn(k, 3)
	if s.goadedAt(k, sa) {
		t.Fatal("seat 3's goad did not end at seat 3's turn start")
	}
}
