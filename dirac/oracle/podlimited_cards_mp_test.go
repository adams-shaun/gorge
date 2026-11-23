package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// Cards a kgen lowering once marked two-seat only (kop.CardDef.TwoSeat) for
// reading "the opponent" as one player, now lowered for every seat count: a
// four-seat pod holding one plays to its end in lockstep. Each test deals the
// evidence loop's focus pool (kfuzz -synth <pool> -seats 4 -focus <card>
// -seed 1) and plays the listed pods (a pod whose dealt decks hit an
// interaction guard is skipped, as kfuzz skips it), and requires the card's
// ability to have fired in some of them.

// firedTally counts the ability triggers a card put on gorge's stack and the
// counters of one kind its game placed.
func firedTally(card, counter string, fired, counters *int) func(g *GorgeSide) {
	return func(g *GorgeSide) {
		for j := range g.E.L.Events {
			e := &g.E.L.Events[j]
			switch {
			case e.Kind == events.TriggerPush:
				if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == card {
					*fired++
				}
			case e.Kind == events.CounterChange && counter != "" && e.Counter == counter:
				*counters++
			}
		}
	}
}

// TestFourSeatWeaverKingPodsLockstep: The Weaver King's per-opponent pick
// walks the three opponents (the each-opponent loop, the pick over the loop
// player's own graveyard asked of the controller, CR 101.4), not the one
// opponent two seats read.
func TestFourSeatWeaverKingPodsLockstep(t *testing.T) {
	var fired, n int
	y := playFocusPods(t, "tail-64", "The Weaver King", "The Weaver King\n", []int{2, 3, 4, 6, 7, 8, 10, 11}, firedTally("The Weaver King", "", &fired, &n))
	t.Logf("%+v triggers=%d", y, fired)
	if y.played == 0 || fired == 0 {
		t.Fatalf("The Weaver King never triggered: %+v triggers=%d", y, fired)
	}
}

// TestFourSeatEtrataPodsLockstep: Etrata, the Silencer's "owns three or more
// exiled cards with hit counters" reads the damaged player's own exile (the
// trigger player, any seat), and its target is that player's creature.
func TestFourSeatEtrataPodsLockstep(t *testing.T) {
	var fired, hits int
	y := playFocusPods(t, "tail-63", "Etrata, the Silencer", "Etrata, the Silencer\n", []int{0, 1, 2, 3, 4, 5, 6, 7}, firedTally("Etrata, the Silencer", "HIT", &fired, &hits))
	t.Logf("%+v triggers=%d hit counters=%d", y, fired, hits)
	if fired == 0 || hits == 0 {
		t.Fatalf("Etrata never exiled a creature with a hit counter: triggers=%d hits=%d", fired, hits)
	}
}

// TestFourSeatRavenloftPodsLockstep: Ravenloft Adventurer's exile-with-a-hit-
// counter replacement and its attack trigger's "each card they own in exile
// with a hit counter" read the defending player (the seat attacked at any seat
// count), not the controller's one opponent.
func TestFourSeatRavenloftPodsLockstep(t *testing.T) {
	var fired, hits int
	y := playFocusPods(t, "tail-32", "Ravenloft Adventurer", "Ravenloft Adventurer\n", []int{2, 4, 8, 15}, firedTally("Ravenloft Adventurer", "HIT", &fired, &hits))
	t.Logf("%+v triggers=%d hit counters=%d", y, fired, hits)
	if y.played == 0 || hits == 0 {
		t.Fatalf("no creature was exiled with a hit counter: %+v hits=%d", y, hits)
	}
}
