package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// gainWatcherSrc draws whenever any player gains life.
const gainWatcherSrc = "Name:Gain Watcher\nTypes:Creature Bird\nPT:1/1\n" +
	"T:Mode$ LifeGained | ValidPlayer$ Player | TriggerZones$ Battlefield | Execute$ WatcherDraws | TriggerDescription$ Whenever a player gains life, draw a card.\n" +
	"SVar:WatcherDraws:DB$ Draw\n" +
	"Oracle:x\n"

// Dr. Beverly Crusher: her controller gaining life makes each life-gain
// trigger of a permanent they control trigger an additional time, her own
// included; an opponent's gain triggers the same listener only once (CR
// 603.2d: the static scopes the triggering event's player).
func TestCorpusCrusherDoublesOnlyHerControllersLifeGainTriggers(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	doc := onBoardCard(t, e, 0, corpusCard(t, "Dr. Beverly Crusher"))
	watcher := onBoard(t, e, 0, gainWatcherSrc)
	queued := func(src state.ObjID) int {
		n := 0
		for _, pt := range e.pendingTriggers {
			if pt.Source == src {
				n++
			}
		}
		return n
	}

	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 2})
	if queued(doc) != 2 || queued(watcher) != 2 {
		t.Fatalf("her controller's gain queued %d Crusher and %d watcher triggers, want 2 and 2", queued(doc), queued(watcher))
	}
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: 2})
	if queued(doc) != 0 || queued(watcher) != 1 {
		t.Fatalf("an opponent's gain queued %d Crusher and %d watcher triggers, want 0 and 1", queued(doc), queued(watcher))
	}
}
