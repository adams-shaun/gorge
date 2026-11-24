package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMissHighwaterLKICopySelectorCopiesLoserBoard drives the REAL corpus
// card's LoseGameTrigger body (Defined$ TriggeredLastStateBattlefield
// LKICopy.Creature+ControlledBy TriggeredPlayer,Artifact+ControlledBy
// TriggeredPlayer) on a three-seat board. Seat 1 loses controlling two
// creatures, an artifact creature and a land; seat 0 controls Miss Highwater;
// seat 2 controls a creature that must not be copied. The loss's own
// state-based sweep (CR 800.4a) removes seat 1's permanents BEFORE the trigger
// resolves, so the selector has to read what seat 1 controlled when it lost.
//
// The Effect's delayed registration of the LoseGameTrigger (a comma-listed
// Triggers$) is the sibling arming change's, so the fire is driven by hand: the
// trigger's referents are captured from the real PlayerLost event exactly as a
// fire captures them (rules' triggerReferents), the loss's sweep runs, and then
// the real TrigCopy body resolves for seat 0.
func TestMissHighwaterLKICopySelectorCopiesLoserBoard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	mhwCard := mustCorpusCard(t, reg, "Miss Highwater")
	bears := mustCorpusCard(t, reg, "Grizzly Bears")
	ornithopter := mustCorpusCard(t, reg, "Ornithopter")
	deck := func(front ...*cards.Card) []*cards.Card {
		return append(append([]*cards.Card(nil), front...), mountainDeck(t, 40-len(front))...)
	}
	cfg := seatZeroStart(Config{Seed: 47, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{deck(mhwCard), deck(bears, bears, ornithopter), deck(bears)}})
	e := New(cfg)

	park := func(owner state.PlayerID, name string, n int) []state.ObjID {
		var out []state.ObjID
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if len(out) == n {
				break
			}
			if o.Owner == owner && o.Zone != state.ZBattlefield && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: state.ZBattlefield})
				out = append(out, o.ID)
			}
		}
		if len(out) != n {
			t.Fatalf("parked %d of %d %q for seat %d", len(out), n, name, owner)
		}
		return out
	}
	mhw := park(0, "Miss Highwater", 1)[0]
	loserBoard := append(append(park(1, "Grizzly Bears", 2), park(1, "Ornithopter", 1)...), park(1, "Mountain", 1)...)
	park(2, "Grizzly Bears", 1)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	e.pending = nil
	e.pendingTriggers = nil

	// Preconditions: the loser's permanents are on the battlefield before
	// the loss, at least three of them, and one is neither artifact nor
	// creature.
	onBattlefield := 0
	for _, id := range loserBoard {
		if o := e.G.Obj(id); o.Zone == state.ZBattlefield && e.controllerOf(id) == 1 {
			onBattlefield++
		}
	}
	if onBattlefield != len(loserBoard) || onBattlefield < 3 {
		t.Fatalf("precondition: %d of %d loser permanents on the battlefield, want all and at least 3", onBattlefield, len(loserBoard))
	}
	tokensOf := func(p state.PlayerID) (names map[string]int, total int) {
		names = map[string]int{}
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if o := e.G.Obj(id); o.IsToken && o.Face() != nil {
				names[o.Face().Name]++
				total++
			}
		}
		return names, total
	}
	if _, n := tokensOf(0); n != 0 {
		t.Fatalf("precondition: seat 0 already has %d tokens", n)
	}

	face := e.G.Obj(mhw).Face()
	trig, ok := cards.ParseTriggerLine(face.SVars["LoseGameTrigger"])
	if !ok {
		t.Fatal("precondition: LoseGameTrigger does not parse")
	}
	body := cards.ResolveSVar(face.SVars, "TrigCopy")
	if body == nil {
		t.Fatal("precondition: TrigCopy does not resolve")
	}
	n0 := len(e.L.Events)
	if !e.playerLoses(1, loseReasonEffect, "test loss") {
		t.Fatal("precondition: seat 1's loss did not happen")
	}
	lost := e.L.Events[n0]
	if lost.Kind != events.PlayerLost || lost.Player != 1 {
		t.Fatalf("precondition: first event after the loss is %v for seat %d", lost.Kind, lost.Player)
	}
	refs := e.triggerReferents(trig, mhw, lost, nil)
	e.Advance() // the CR 800.4a sweep: seat 1's permanents cease to exist
	for _, id := range loserBoard {
		if z := e.G.Obj(id).Zone; z != state.ZCeased {
			t.Fatalf("precondition: loser permanent %d is in zone %v after the sweep, want ceased", id, z)
		}
	}
	ctx := effects.NewCtxPtr(mhw, 0, effects.CtxInit{
		TriggerContext: refs,
		SVars:          face.SVars,
		Remembered:     []state.Target{{Player: 1, IsPlayer: true}},
	})
	effects.Resolve(e, ctx, body)
	drainStack(t, e, 40)

	names, total := tokensOf(0)
	if total != 3 || names["Grizzly Bears"] != 2 || names["Ornithopter"] != 1 {
		t.Fatalf("seat 0 tokens = %v (total %d), want 2 Grizzly Bears + 1 Ornithopter (the land is not copied)", names, total)
	}
	if _, n := tokensOf(1); n != 0 {
		t.Fatalf("seat 1 (the loser) controls %d tokens, want 0: the copies belong to the Effect's controller", n)
	}
	if _, n := tokensOf(2); n != 0 {
		t.Fatalf("seat 2 controls %d tokens, want 0", n)
	}
}
