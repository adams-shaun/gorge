package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// startCounterWatcher draws whenever a player is given counters: the trigger a
// scripted start must drop, and the control proving the seed event reached
// trigger discovery in the first place.
const startCounterWatcher = "Name:Start Counter Watcher\nManaCost:1\nTypes:Creature Elf\nPT:1/1\n" +
	"T:Mode$ CounterPlayerAddedAll | ValidObject$ Player | Execute$ WatcherDraw | TriggerDescription$ x\n" +
	"SVar:WatcherDraw:DB$ Draw | NumCards$ 1 | Defined$ You\nOracle:x\n"

// TestScriptedStartPlayerCounters seeds player counters through a scripted
// start: the counts are ordinary logged PlayerCounterChange events in the hash
// chain, a log-only replay restates them, and the trigger they queue is setup
// state that Settle drops.
func TestScriptedStartPlayerCounters(t *testing.T) {
	special := [2]map[int]string{{2: startCounterWatcher}, {}}
	start := &StartSpec{Seats: []StartSeat{{
		Battlefield: []int{2},
		PlayerCounters: []StartPlayerCounter{
			{Kind: "ENERGY", N: 2},
			{Kind: "POISON", N: 1},
		},
	}}}
	cfg := startConfig(t, 17, start, special)
	e := New(cfg)

	// Precondition: the watcher is in play, so the seed's events have a
	// trigger to queue and Settle has one to drop.
	watcher := e.G.Obj(state.ObjID(2 + 1))
	if watcher == nil || watcher.Zone != state.ZBattlefield {
		t.Fatalf("watcher is not on the battlefield: %+v", watcher)
	}
	if got := e.G.Players[0].Counter("ENERGY"); got != 2 {
		t.Errorf("ENERGY = %d, want 2", got)
	}
	if got := e.G.Players[0].Counter("POISON"); got != 1 {
		t.Errorf("POISON = %d, want 1", got)
	}

	// One logged event per entry: the counts are in the hash chain, and a
	// log-only replay (events.Apply alone, never the StartSpec) restates them.
	var seeded []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.PlayerCounterChange && ev.Player == 0 {
			seeded = append(seeded, ev)
		}
	}
	if len(seeded) != 2 {
		t.Fatalf("setup logged %d player-counter events, want 2: %+v", len(seeded), seeded)
	}
	if diff := diffGames(e.G, replayFromLog(t, cfg, e.L.Events)); diff != "" {
		t.Errorf("log replay differs from the live setup:\n%s", diff)
	}

	// The seed is setup state, so the two triggers the watcher queued are
	// dropped before the game starts.
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush {
			t.Fatalf("a setup seed's trigger reached the stack: %+v", ev)
		}
	}
	if n := len(e.pendingTriggers); n != 0 || e.orderedTriggers != 0 {
		t.Fatalf("pending triggers after setup: %d (ordered %d)", n, e.orderedTriggers)
	}

	// Control: the same watcher does queue on a player-counter event outside
	// setup, so the empty queue above is Settle's drop, not a card mismatch.
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "ENERGY", Amount: 1})
	if n := len(e.pendingTriggers); n == 0 {
		t.Fatal("control: the watcher did not queue on a player-counter event")
	}
}
