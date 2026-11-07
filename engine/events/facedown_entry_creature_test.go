package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestFaceDownExilePlayedFaceUpEntryCreature pins CR 708.2: the "is a 2/2
// creature" reading of FaceDown belongs to a face-down permanent ON the
// battlefield. A card exiled face down (Black Cat, Cunning Thief's
// exiled_with_face_down_maylook marker) and then played face up enters as
// what its face says, so its ThisTurnEntered_*_Creature record is a creature
// entry only when the face is a creature. The pre-fix fold read the stale
// exile FaceDown flag and counted a played land as a creature entry, which
// made Spider-UK's "if two or more creatures entered" end-step trigger fire.
func TestFaceDownExilePlayedFaceUpEntryCreature(t *testing.T) {
	for _, tc := range []struct {
		name      string
		src       string
		wantEntry bool // WasCreature on the battlefield entry
	}{
		{
			name:      "land face is not a creature",
			src:       "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n",
			wantEntry: false,
		},
		{
			name:      "creature face is a creature",
			src:       "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n",
			wantEntry: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := state.NewGame([]string{"Ann", "Bob"})
			c, diags := cards.ParseBytes("card.txt", []byte(tc.src))
			if len(diags) != 0 {
				t.Fatalf("diags: %v", diags)
			}
			c.Link()
			o := g.AddObject(c, 0)
			g.SetZone(state.ZLibrary, 0, []state.ObjID{o.ID})

			// The face-down may-look exile a Dig with ExileFaceDown$ True mints.
			Apply(g, Event{Kind: MoveZone, Obj: o.ID, From: state.ZLibrary,
				To: state.ZExile, Counter: "exiled_with_face_down_maylook", Amount: 0})
			if o.Zone != state.ZExile || !o.FaceDown {
				t.Fatalf("precondition: zone=%v faceDown=%v, want exile face down", o.Zone, o.FaceDown)
			}

			// Play it face up: no FaceDown$ entry marker.
			Apply(g, Event{Kind: MoveZone, Obj: o.ID, From: state.ZExile, To: state.ZBattlefield,
				Player: 0})

			if o.Zone != state.ZBattlefield || o.FaceDown {
				t.Fatalf("after the entry: zone=%v faceDown=%v, want battlefield face up", o.Zone, o.FaceDown)
			}
			found := false
			for _, e := range g.Entered {
				if e.Obj == o.ID && e.To == state.ZBattlefield {
					found = true
					if e.WasCreature != tc.wantEntry {
						t.Fatalf("battlefield entry WasCreature=%v, want %v", e.WasCreature, tc.wantEntry)
					}
				}
			}
			if !found {
				t.Fatal("no battlefield entry recorded")
			}
		})
	}
}
