package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The livelock watcher's object cap (LoopGuard.MaxObjs) bounds the LIVE
// board -- state.Game.LiveObjs, every card-shaped object that has not ceased
// to exist -- not the arena's length. The arena is monotone: it also holds
// every ceased object, every resolved trigger's exile parking, and every
// pending trigger on the stack. The hunt's cmdr4 leg caught the arena-length
// basis aborting the Scute Swarm storm games on a stack of pending landfall
// triggers (seed 78391's abort: arena 10001 with 5654 pending triggers and
// 2009 parked ones, so the persistent board was only 2134). The cap
// therefore excludes a CR 113.7a ability object entirely -- pending or
// parked -- because a storm queues thousands of them without growing the
// persistent board.

// stageObjs mints n fixture objects on the engine's arena and parks nCeased
// of them in ZCeased, the direct fixture staging the state-based-action and
// restart tests also use (no event, pre-play): the count under test is the
// mint/cease bookkeeping itself, not an effect.
func stageObjs(t *testing.T, e *Engine, n, nCeased int) {
	t.Helper()
	c := card(t, "Name:Filler\nTypes:Creature Construct\nOracle:x\n")
	for i := 0; i < n; i++ {
		o := e.G.AddObject(c, 0)
		if i < nCeased {
			o.Zone = state.ZCeased
			// The staging bypasses the event folds; the tally follows by hand.
			e.G.ClearLiveObject()
		}
	}
}

// countedLive is the reference the tally is checked against: a card-shaped
// object that has not ceased to exist. A CR 113.7a ability object (nil
// Card) is not board state, pending on the stack or parked in exile.
func countedLive(g *state.Game) int {
	n := 0
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZCeased && o.Card != nil {
			n++
		}
	}
	return n
}

// TestObjectCapCountsLiveNotTheArena pins the cap's basis: an arena past the
// cap whose live population is under it is NOT aborted (a storm game's
// resolved triggers and expired tokens stay in the arena forever), while the
// live population crossing it is, with Count naming the live count.
func TestObjectCapCountsLiveNotTheArena(t *testing.T) {
	e := livelockTestEngine(t, &LoopGuard{MaxObjs: 100})
	// 80 objects on the arena, 50 of them live: the arena is under the cap
	// and nothing aborts.
	stageObjs(t, e, 80, 30)
	if got := e.G.LiveObjs(); got != 50 {
		t.Fatalf("LiveObjs = %d, want 50 (80 staged, 30 ceased)", got)
	}
	if got := countedLive(e.G); got != 50 {
		t.Fatalf("the reference count = %d, want 50: the tally and the reference disagree before any event", got)
	}
	if lle := drivePattern(t, e, func(int) events.Event {
		return events.Event{Kind: events.Note, Obj: 1, Text: "ordinary play"}
	}, 5); lle != nil {
		t.Fatalf("watcher aborted an arena past 50 live objects under cap 100: %v", lle)
	}
	// Exactly at the cap is still no abort (the watcher aborts on crossing).
	stageObjs(t, e, 50, 0)
	if got := e.G.LiveObjs(); got != 100 || len(e.G.Objs) != 130 {
		t.Fatalf("live %d arena %d, want 100/130", got, len(e.G.Objs))
	}
	if lle := drivePattern(t, e, func(int) events.Event {
		return events.Event{Kind: events.Note, Obj: 1, Text: "at the cap"}
	}, 5); lle != nil {
		t.Fatalf("watcher aborted AT the cap (the abort is on crossing): %v", lle)
	}
	// One more live object crosses it: the abort fires with the LIVE count
	// and the cap, on an arena (131) that was already past the cap an event
	// ago.
	stageObjs(t, e, 1, 0)
	var lle *LivelockError
	func() {
		defer func() {
			if r := recover(); r != nil {
				l, ok := r.(*LivelockError)
				if !ok {
					panic(r)
				}
				lle = l
			}
		}()
		e.emit(events.Event{Kind: events.Note, Obj: 1, Text: "crossing"})
	}()
	if lle == nil {
		t.Fatal("watcher never fired with the live population past the cap")
	}
	if lle.Reason != ObjectCapReason || lle.Count != 101 || lle.Cap != 100 {
		t.Errorf("abort = reason %q count %d cap %d, want object cap / 101 / 100", lle.Reason, lle.Count, lle.Cap)
	}
	if !strings.Contains(lle.Error(), "the game holds 101 live card object(s), cap 100") {
		t.Errorf("diagnostic %q does not name the live count", lle.Error())
	}
	// A clone carries the same tally as its own field (host paths clone at
	// every turn start and keep watching).
	if got := e.Clone().G.LiveObjs(); got != e.G.LiveObjs() {
		t.Errorf("clone LiveObjs = %d, want %d", got, e.G.LiveObjs())
	}
}

// TestObjectCapIgnoresThePendingTriggerStack replays seed 78391's shape at
// fixture scale: the arena is far past the cap, but 500 pending CR 113.7a
// trigger objects are on the stack while the card-shaped board is under it.
// The watcher must not abort (the triggers are not board state); only the
// board crossing the cap does.
func TestObjectCapIgnoresThePendingTriggerStack(t *testing.T) {
	e := livelockTestEngine(t, &LoopGuard{MaxObjs: 100})
	stageObjs(t, e, 90, 0)
	for i := 0; i < 500; i++ {
		a := e.G.AddObject(nil, 0)
		e.emit(events.Event{Kind: events.MoveZone, Obj: a.ID, From: state.ZLibrary, To: state.ZStack})
		a.Ability = &cards.SA{}
	}
	// Precondition: the arena is past the cap, the board is not.
	if got := len(e.G.Objs); got != 590 {
		t.Fatalf("arena = %d, want 590 (90 staged cards + 500 pending triggers)", got)
	}
	if got, ref := e.G.LiveObjs(), countedLive(e.G); got != 90 || ref != 90 {
		t.Fatalf("live board = %d (reference %d), want 90 under cap 100", got, ref)
	}
	if lle := drivePattern(t, e, func(i int) events.Event {
		return events.Event{Kind: events.Note, Obj: state.ObjID(1 + i%3), Text: "storm resolving"}
	}, 5); lle != nil {
		t.Fatalf("watcher aborted on an arena past the cap whose board is 90: %v", lle)
	}
	// The board itself crossing the cap still aborts.
	stageObjs(t, e, 11, 0)
	var lle *LivelockError
	func() {
		defer func() {
			if r := recover(); r != nil {
				l, ok := r.(*LivelockError)
				if !ok {
					panic(r)
				}
				lle = l
			}
		}()
		e.emit(events.Event{Kind: events.Note, Obj: 1, Text: "board crossing"})
	}()
	if lle == nil || lle.Count != 101 || lle.Cap != 100 {
		t.Fatalf("board crossing: got %+v, want an object-cap abort at 101/100", lle)
	}
}

// TestObjectCapLiveFoldIsTheMove pins the tally's maintenance through the
// real event folds: a cease and the under-card revival each move it by
// exactly one, while a CR 113.7a ability object never joins it -- pending on
// the stack or parked in exile -- and a CR 727.1 restart sweep does not
// clear a parked ability a second time.
func TestObjectCapLiveFoldIsTheMove(t *testing.T) {
	e := livelockTestEngine(t, &LoopGuard{MaxObjs: 3})
	c := card(t, "Name:Filler\nTypes:Creature Construct\nOracle:x\n")
	o := e.G.AddObject(c, 0)
	if got := e.G.LiveObjs(); got != 1 {
		t.Fatalf("LiveObjs after the mint = %d, want 1", got)
	}
	// Library -> battlefield -> ceased: one object's ordinary life and death.
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZBattlefield, To: state.ZCeased, Text: "ceased to exist"})
	if got := e.G.LiveObjs(); got != 0 {
		t.Fatalf("LiveObjs after the cease = %d, want 0", got)
	}
	// A ZCeased park coming back (the under-card restore) is live again.
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZCeased, To: state.ZBattlefield})
	if got := e.G.LiveObjs(); got != 1 {
		t.Fatalf("LiveObjs after the revival = %d, want 1", got)
	}
	// A CR 113.7a face-less ability is not a card object, so it never joins
	// the tally: neither the pending trigger (the move to the stack) nor its
	// logged stack -> exile parking. The ability field is set after the move,
	// as Apply's own mint (foldTriggerPush) does.
	a := e.G.AddObject(nil, 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: a.ID, From: state.ZLibrary, To: state.ZStack})
	a.Ability = &cards.SA{}
	if got := e.G.LiveObjs(); got != 1 {
		t.Fatalf("LiveObjs with the ability pending on the stack = %d, want 1 (a trigger is not board state)", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: a.ID, From: state.ZStack, To: state.ZExile})
	if got := e.G.LiveObjs(); got != 1 {
		t.Fatalf("LiveObjs after the ability parked = %d, want 1 (the ability was never live)", got)
	}
	if got := countedLive(e.G); got != 1 {
		t.Fatalf("the reference count = %d, want 1: the tally and the reference disagree", got)
	}
	// A CR 727.1 game restart ceases every remaining object in one sweep.
	// The parked ability above is already off the tally, so the sweep must
	// not clear it again; the card object o is re-dealt as a fresh library
	// object (CR 727.2), which is the one live object left afterwards.
	e.emit(events.Event{Kind: events.GameRestart, Amount: 40})
	if a := e.G.Obj(a.ID); a == nil || a.Zone != state.ZCeased {
		t.Fatalf("the restart left the parked ability in %v, want ZCeased (the sweep ran)", a)
	}
	if got, want := e.G.LiveObjs(), countedLive(e.G); got != want || want != 1 {
		t.Fatalf("after GameRestart: LiveObjs = %d, the arena's live count = %d, want both 1 (a parked ability was cleared twice)", got, want)
	}
}

// TestObjectCapLiveTracksTheAcceptanceGame is the tally's whole-game
// invariant over a real corpus game: after every checked decision,
// state.Game.LiveObjs equals the count of card-shaped objects that have not
// ceased to exist. It is what catches a mint or a cease fold the tally's
// maintenance misses, live and on a replay alike.
func TestObjectCapLiveTracksTheAcceptanceGame(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names, decks, err := testutil.AcceptanceDecks(reg, 2)
	if err != nil {
		t.Fatal(err)
	}
	cfg := AcceptanceConfig(reg, names, decks)
	_, _, err = PlayAcceptance(cfg, acceptanceTestBot, func(e *Engine, n int) {
		if got, want := e.G.LiveObjs(), countedLive(e.G); got != want {
			t.Fatalf("at decision %d: LiveObjs = %d, the arena's live count = %d", n, got, want)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}
