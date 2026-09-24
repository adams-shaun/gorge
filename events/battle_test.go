package events

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// siegeCard is a real-shaped Battle Siege front face (Invasion of Tolvada's
// non-oracle body): the types carry Battle Siege and Defense prints 5.
func siegeCard() *cards.Card {
	c, d := cards.ParseBytes("siege.txt", []byte("Name:Invasion of Tolvada\nManaCost:3 W B\nTypes:Battle Siege\nDefense:5\nOracle:x\n"))
	if len(d) != 0 {
		panic(d)
	}
	c.Link()
	return c
}

// gameWithSiege builds a two-seat game with one Siege in seat 0's hand.
func gameWithSiege(t *testing.T) (*state.Game, state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"Ann", "Bob"})
	o := g.AddObject(siegeCard(), 0)
	o.Zone = state.ZHand
	g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
	return g, o.ID
}

// TestEntryCounterGrantsBattleDefense is CR 310.6/310.8's entry grant:
// a Battle enters with defense counters equal to its printed Defense. The
// arithmetic now lives in EntryCounterGrants; events.Move no longer folds it
// itself (rules places it through a real CounterChange so the CR 614
// replacement class and the CantPutCounter prohibition see it, task
// addcounter1/2), so this test also pins that a bare MoveZone fold grants
// nothing -- a re-added fold would double every entry counter.
func TestEntryCounterGrantsBattleDefense(t *testing.T) {
	g, id := gameWithSiege(t)
	if grants := EntryCounterGrants(g.Obj(id), false); len(grants) != 1 ||
		grants[0].Kind != "DEFENSE" || grants[0].Amount != 5 {
		t.Fatalf("EntryCounterGrants = %+v, want one DEFENSE 5", grants)
	}
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	if got := g.Obj(id).Counter("DEFENSE"); got != 0 {
		t.Fatalf("bare MoveZone fold granted %d defense counters, want 0 (rules is the sole placement path)", got)
	}
}

// TestEntryCounterGrantsFaceDownGrantsNothing is the manifest/cloak
// boundary: a face-down entry (CR 708.5) is a 2/2 creature, not a Battle,
// and grants no defense counters -- nor any other entry counter. Both
// battlefield face-down entry markers reach the same gate, via the shared
// IsFaceDownEntry predicate the rules snapshot uses.
func TestEntryCounterGrantsFaceDownGrantsNothing(t *testing.T) {
	for _, tc := range []struct {
		name, counter string
		cloaked       bool
	}{
		{"manifest", FaceDownEntryCounter, false},
		{"cloak", CloakEntryCounter, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, id := gameWithSiege(t)
			Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield,
				Counter: tc.counter})
			o := g.Obj(id)
			if !o.FaceDown {
				t.Fatal("face-down entry did not fold FaceDown")
			}
			if o.Cloaked != tc.cloaked {
				t.Fatalf("Cloaked=%v, want %v", o.Cloaked, tc.cloaked)
			}
			if !IsFaceDownEntry(tc.counter) {
				t.Fatalf("IsFaceDownEntry(%q) = false, want true", tc.counter)
			}
			if got := EntryCounterGrants(o, IsFaceDownEntry(tc.counter)); len(got) != 0 {
				t.Fatalf("face-down EntryCounterGrants = %+v, want none", got)
			}
		})
	}
}

// TestIsFaceDownEntryCoversBothMarkers pins the shared predicate the rules
// side reads when it must skip a face-down battlefield entry: BOTH markers
// count (the payload-bearing manifest spellings too), and no other MoveZone
// Counter value does. Without this, a consumer reaching only for
// FaceDownEntryFields silently misses every cloak entry.
func TestIsFaceDownEntryCoversBothMarkers(t *testing.T) {
	for _, c := range []string{
		FaceDownEntryCounter,
		CloakEntryCounter,
		FaceDownEntryCounterFor("Land & Forest", 0, 0, false),
		FaceDownEntryCounterFor("", 3, 3, true),
	} {
		if !IsFaceDownEntry(c) {
			t.Errorf("IsFaceDownEntry(%q) = false, want true", c)
		}
	}
	for _, c := range []string{"", "exiled_with", "face_down", "exiled_with_face_down", "entered_cloaked_x"} {
		if IsFaceDownEntry(c) {
			t.Errorf("IsFaceDownEntry(%q) = true, want false", c)
		}
	}
}

// TestBattleSameZoneMoveDoesNotRegrant pins the wasBattlefield boundary at
// the point it now lives: EntryCounterGrants is zone-agnostic (it reads only
// the object's own characteristics and the logged elections), so the RULES
// snapshot -- not the fold -- is what skips a battlefield-internal move by
// checking the object's live zone. rules/entry_counters_test.go's
// TestEntryCounterStayDoesNotRegrant pins that engine gate on a real battle.
func TestBattleSameZoneMoveDoesNotRegrant(t *testing.T) {
	g, id := gameWithSiege(t)
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZBattlefield})
	if got := EntryCounterGrants(g.Obj(id), false); len(got) != 1 || got[0].Amount != 5 {
		t.Fatalf("EntryCounterGrants on a battlefield Siege = %+v, want one DEFENSE 5 (the zone skip is the rules snapshot's)", got)
	}
}

// TestSiegeProtectorChoiceFoldsAndResets pins the CR 310.10 record: a Choose
// "protector" event folds the chosen opponent onto the battle object, and
// leaving the battlefield clears it (a re-entering battle is protected
// afresh -- Move's leaving-the-battlefield reset list).
func TestSiegeProtectorChoiceFoldsAndResets(t *testing.T) {
	g, id := gameWithSiege(t)
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	Apply(g, Event{Kind: Choose, Obj: id, Counter: "protector", Player: 1})
	o := g.Obj(id)
	if !o.ProtectorValid || o.Protector != 1 {
		t.Fatalf("protector fold: valid=%v protector=%d, want true/1", o.ProtectorValid, o.Protector)
	}
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	o = g.Obj(id)
	if o.ProtectorValid || o.Protector != 0 {
		t.Fatalf("protector survived leaving the battlefield: valid=%v protector=%d", o.ProtectorValid, o.Protector)
	}
}
