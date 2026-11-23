package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExileCountOwnedBySeatFourSeats: ExCountExile with N 2 counts only the
// exiled cards player x.Ref owns, in a four-seat game where "the other
// seat's exile" is not "the opponent's" (Ravenloft Adventurer's defending
// player, Etrata, the Silencer's damaged player). Cards sit in every seat's
// exile; the count follows the named seat, whichever it is, for the trigger
// player (Etrata) and for an attack trigger's bound defending seat
// (Ravenloft). Without N 2 the count spans every living seat's exile.
func TestExileCountOwnedBySeatFourSeats(t *testing.T) {
	sc, id := fourSeatScen(t, "preventkw", "Plains", []cardset.Entry{{Name: "Plains", Count: 8}, {Name: "Shock", Count: 2}})
	if sc.s.duel() {
		t.Fatal("not a four-seat game: the seat reads below would fall back to the duel's one opponent")
	}
	want := [MaxSeats]int{0, 1, 2, 3} // seat p's exile holds p cards
	for p := uint8(0); p < MaxSeats; p++ {
		for i := 0; i < int(p); i++ {
			sc.s.place(sc.obj(id["Shock"], p), kop.ZoneExile, false)
		}
	}
	for p := range want {
		if got := int(sc.s.pl[p].exileN); got != want[p] {
			t.Fatalf("seat %d exile holds %d cards, want %d: the scenario is not set", p, got, want[p])
		}
	}
	read := func(n int16, ref kop.Ref, e *stackEntry, ctrl uint8) int {
		x := kop.Expr{Kind: kop.ExCountExile, N: n, Ref: ref}
		return sc.s.exprCold(sc.k, &x, evalCtx{e: e, ctrl: ctrl, src: NoObj})
	}
	all := read(0, 0, nil, 0)
	if all != 6 {
		t.Fatalf("N 0 counts %d exiled cards, want all 6: the N 2 reads below would not differ from it", all)
	}
	for q := uint8(0); q < MaxSeats; q++ {
		// Etrata: the trigger player (the damaged seat q), whoever controls the trigger
		tr := &stackEntry{kind: seTrigger, trigPl: q}
		if got := read(2, kop.RefTriggerPlayer, tr, 0); got != want[q] {
			t.Errorf("trigger player seat %d: counted %d, want the seat's own %d", q, got, want[q])
		}
		// Ravenloft Adventurer: the defending seat bound to the attack trigger
		at := &stackEntry{kind: seTrigger, defSeat: q + 1}
		if got := read(2, kop.RefDefendingPlayer, at, 0); got != want[q] {
			t.Errorf("defending seat %d: counted %d, want the seat's own %d", q, got, want[q])
		}
	}
	// a seat that left the game has no exile to count; no seat at all counts nothing
	sc.s.pl[3].lost = true
	if got := read(2, kop.RefTriggerPlayer, &stackEntry{kind: seTrigger, trigPl: 3}, 0); got != 0 {
		t.Errorf("a seat that left the game: counted %d, want 0", got)
	}
	if got := read(2, kop.RefTriggerPlayer, nil, 0); got != 0 {
		t.Errorf("no trigger entry: counted %d, want 0", got)
	}
}
