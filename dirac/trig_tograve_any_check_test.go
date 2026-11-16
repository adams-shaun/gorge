package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckAnyToGraveyard is the kernel-side half of TestAnyToGraveyardFiresOncePerCard
// (dirac_test). Vulturous Zombie's "whenever a card is put into an opponent's
// graveyard from anywhere" lowers to two kernel triggers (kgen
// lower_anytograve.go): a ToGraveyard line for every origin but the battlefield
// and a Dies line, and a move reaches exactly one of them. Each of the
// opponent's discard, mill and creature death queues the Zombie's trigger once,
// and none of the Zombie controller's own cards queue it.
func CheckAnyToGraveyard(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	zombie, bears, swamp := id["Vulturous Zombie"], id["Grizzly Bears"], id["Swamp"]
	for _, cfg := range []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}} {
		sc := newScenTableLib(t, tbl, swamp, cfg)
		z := sc.perm(zombie, 0)
		if !sc.onBF(z) {
			t.Fatal("Vulturous Zombie is not on the battlefield")
		}
		// the table carries both halves of the one printed line
		var toGrave, dies int
		for _, a := range tbl.Cards[zombie].Abilities {
			if a.Kind != kop.AbTriggered {
				continue
			}
			switch a.Trig.Event {
			case kop.TrigToGraveyard:
				toGrave++
			case kop.TrigDies:
				dies++
			}
		}
		if toGrave != 1 || dies != 1 {
			t.Fatalf("Vulturous Zombie has %d ToGraveyard and %d Dies lines, want 1 and 1", toGrave, dies)
		}
		pend := func(what string, want int, move func()) {
			t.Helper()
			sc.s.npend = 0
			move()
			if got := int(sc.s.npend); got != want {
				t.Errorf("gorgeLowering=%v: %s queued %d triggers, want %d", cfg.GorgeLowering, what, got, want)
			}
			sc.s.npend = 0
		}
		// the opponent's card from the hand (discard)
		h := sc.hand(swamp, 1)
		pend("an opponent's discard", 1, func() { sc.s.discardObj(sc.k, h) })
		// the opponent's card from the library (mill)
		pend("an opponent's mill", 1, func() { sc.s.millTop(1, 1, 0) })
		// the opponent's creature dying from the battlefield
		b := sc.perm(bears, 1)
		pend("an opponent's creature dying", 1, func() { sc.s.destroy(sc.k, sc.slot(b)) })
		// a noncreature permanent of the opponent's dying (a land): the Dies
		// line's ValidCard$ is a bare Card, so it hears more than creatures
		land := sc.perm(swamp, 1)
		pend("an opponent's land dying", 1, func() { sc.s.destroy(sc.k, sc.slot(land)) })
		// two of the opponent's creatures dying together fire once per card
		b1, b2 := sc.perm(bears, 1), sc.perm(bears, 1)
		pend("two opposing creatures dying one after the other", 2, func() {
			sc.s.destroy(sc.k, sc.slot(b1))
			sc.s.destroy(sc.k, sc.slot(b2))
		})
		// the Zombie controller's own cards hear nothing
		own := sc.hand(swamp, 0)
		pend("the controller's discard", 0, func() { sc.s.discardObj(sc.k, own) })
		pend("the controller's mill", 0, func() { sc.s.millTop(0, 1, 0) })
		ob := sc.perm(bears, 0)
		pend("the controller's creature dying", 0, func() { sc.s.destroy(sc.k, sc.slot(ob)) })
	}
}
