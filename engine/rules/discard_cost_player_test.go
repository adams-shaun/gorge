package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A cost discard names no player on its event, and it is still its payer's
// discard (CR 701.9a): "whenever you discard" answers its own controller's and
// nobody else's, whichever seat that is.
func TestCostDiscardBelongsToThePayer(t *testing.T) {
	t.Parallel()
	const (
		watcher = "Name:Discard Watcher\nTypes:Creature Shark\nPT:1/1\n" +
			"T:Mode$ DiscardedAll | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ Grow | TriggerDescription$ x\n" +
			"SVar:Grow:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"
		filler = "Name:Spare Card\nTypes:Sorcery\nOracle:x\n"
	)
	for _, tc := range []struct {
		name         string
		watcher, pay int
		fires        bool
	}{
		{"own seat 0", 0, 0, true},
		{"own seat 1", 1, 1, true},
		{"opponent's as seat 0", 0, 1, false},
		{"opponent's as seat 1", 1, 0, false},
	} {
		e := handEngine(t)
		toMain1(t, e)
		src := onBoard(t, e, state.PlayerID(tc.watcher), watcher)
		id := handCard(e, card(t, filler), state.PlayerID(tc.pay))
		e.emit(events.DiscardCost(id))
		fired := false
		for _, pt := range e.pendingTriggers {
			fired = fired || pt.Source == src
		}
		if fired != tc.fires {
			t.Errorf("%s: triggered = %v, want %v", tc.name, fired, tc.fires)
		}
	}
}
