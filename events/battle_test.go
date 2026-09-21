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

// TestBattleMoveGrantsDefenseCounters is CR 310.6/310.8's entry grant: a
// Battle enters with defense counters equal to its printed Defense, folded
// inside Move so every entry path (cast, blink, search, reanimate) is
// covered by construction.
func TestBattleMoveGrantsDefenseCounters(t *testing.T) {
	g, id := gameWithSiege(t)
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	if got := g.Obj(id).Counter("DEFENSE"); got != 5 {
		t.Fatalf("battle entered with %d defense counters, want 5", got)
	}
}

// TestBattleFaceDownEntryGrantsNothing is the manifest/cloak boundary: a
// face-down entry (CR 708.5) is a 2/2 creature, not a Battle, and gains no
// defense counters -- the same boundary the loyalty and lore grants document.
// Both battlefield face-down entry markers are pinned.
func TestBattleFaceDownEntryGrantsNothing(t *testing.T) {
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
			if got := o.Counter("DEFENSE"); got != 0 {
				t.Fatalf("face-down battle entered with %d defense counters, want 0", got)
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

// TestBattleSameZoneMoveDoesNotRegrant is the wasBattlefield boundary: a
// battlefield-internal move (the fold removes and re-appends within the
// zone) must not run the entry arm a second time and double the grant.
func TestBattleSameZoneMoveDoesNotRegrant(t *testing.T) {
	g, id := gameWithSiege(t)
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZBattlefield})
	if got := g.Obj(id).Counter("DEFENSE"); got != 5 {
		t.Fatalf("battlefield-internal move re-granted: %d defense counters, want 5", got)
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
