package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// placementTapeHost serves a KArrange from the tape: reverse answers the
// offered options in reverse order, otherwise the ask is unserved. Every
// posed decision is kept, with its options copied.
type placementTapeHost struct {
	*fakeHost
	reverse bool
	posed   []*decision.Decision
}

func (h *placementTapeHost) AskCount() uint64 { return uint64(len(h.posed)) }

func (h *placementTapeHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.posed = append(h.posed, &cp)
	if !h.reverse {
		return decision.Intent{}, false
	}
	var ch []int
	for i := len(d.Options) - 1; i >= 0; i-- {
		ch = append(ch, i)
	}
	return decision.Intent{Choices: ch}, true
}

// placementFixture seats a library of two filler cards for seat 0 and puts n
// creature cards in seat 0's graveyard; it returns the graveyard ids in scan
// order.
func placementFixture(t *testing.T, n int, reverse bool) (*placementTapeHost, *Ctx, []state.ObjID, []state.ObjID) {
	t.Helper()
	fh := newHost(t, 2)
	filler := mkCard(t, "Name:Filler\nTypes:Creature\nPT:1/1\nOracle:x\n")
	lib := fillLibrary(fh.g, 0, filler, 2)
	var yard []state.ObjID
	for i := 0; i < n; i++ {
		o := fh.g.AddObject(mkCard(t, "Name:Yard"+string(rune('A'+i))+"\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
		yard = append(yard, o.ID)
	}
	for _, id := range yard {
		fh.g.Obj(id).Zone = state.ZGraveyard
	}
	fh.g.SetZone(state.ZGraveyard, 0, yard)
	h := &placementTapeHost{fakeHost: fh, reverse: reverse}
	return h, &Ctx{Source: lib[0], Controller: 0}, yard, lib
}

func resolveSweep(h *placementTapeHost, c *Ctx, t *testing.T, extra string) {
	t.Helper()
	Resolve(h, c, sa(t, "DB$ ChangeZoneAll | ChangeType$ Creature | Origin$ Graveyard | Destination$ Library"+extra))
}

// TestChangeZoneAllPlacementAskShape: CR 401.4 -- a sweep that puts two or
// more of one owner's cards at the same library position poses ONE
// owner-seated KArrange (Min == Max == the pile, "library_placement"
// options in the pile's current order); the answer is the placed order; and
// one card, RandomOrder$, an owed shuffle and the Hidden$ pick pose nothing.
func TestChangeZoneAllPlacementAskShape(t *testing.T) {
	for _, pos := range []struct {
		name, param string
		bottom      bool
	}{{"Bottom", " | LibraryPosition$ -1", true}, {"Top", " | LibraryPosition$ 0", false}} {
		t.Run(pos.name+"PosesOneArrange", func(t *testing.T) {
			h, c, yard, lib := placementFixture(t, 3, true)
			resolveSweep(h, c, t, pos.param)
			if len(h.posed) != 1 {
				t.Fatalf("asks = %d, want 1", len(h.posed))
			}
			d := h.posed[0]
			if d.Kind != decision.KArrange || d.Player != 0 || d.Min != 3 || d.Max != 3 || len(d.Options) != 3 {
				t.Fatalf("decision = %+v, want a seat-0 KArrange Min==Max==3 over 3 options", d)
			}
			for i, o := range d.Options {
				if o.Kind != "library_placement" || o.Obj != yard[i] {
					t.Fatalf("option %d = %+v, want library_placement over %v in the current order", i, o, yard[i])
				}
			}
			// The answer was the reverse of the offered order; it must differ
			// from the scan order or the assertion below proves nothing.
			want := []state.ObjID{yard[2], yard[1], yard[0]}
			if slices.Equal(want, yard) {
				t.Fatal("precondition: answered order equals the scan order")
			}
			got := h.g.Zone(state.ZLibrary, 0)
			if pos.bottom {
				want = append(append([]state.ObjID(nil), lib...), want...)
			} else {
				want = append(want, lib...)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("library = %v, want %v", got, want)
			}
		})
	}
	t.Run("BottomUnservedKeepsMoveOrderWithoutAnEvent", func(t *testing.T) {
		h, c, yard, lib := placementFixture(t, 3, false)
		resolveSweep(h, c, t, " | LibraryPosition$ -1")
		if len(h.posed) != 1 {
			t.Fatalf("asks = %d, want 1 (precondition: the ask is posed)", len(h.posed))
		}
		if want := append(append([]state.ObjID(nil), lib...), yard...); !slices.Equal(h.g.Zone(state.ZLibrary, 0), want) {
			t.Fatalf("library = %v, want %v", h.g.Zone(state.ZLibrary, 0), want)
		}
		for _, ev := range h.log {
			if ev.Kind == events.LibraryOrder {
				t.Fatalf("an unchanged bottom order emitted a LibraryOrder: %+v", ev)
			}
		}
	})
	for _, tc := range []struct {
		name  string
		n     int
		extra string
	}{
		{"OneCard", 1, " | LibraryPosition$ -1"},
		{"RandomOrderBottom", 3, " | LibraryPosition$ -1 | RandomOrder$ True"},
		{"RandomOrderTop", 3, " | LibraryPosition$ 0 | RandomOrder$ True"},
		{"OwedShuffleBottom", 3, " | LibraryPosition$ -1 | Shuffle$ True"},
		{"OwedShuffleTop", 3, " | LibraryPosition$ 0 | Shuffle$ True"},
	} {
		t.Run(tc.name+"PosesNothing", func(t *testing.T) {
			h, c, yard, _ := placementFixture(t, tc.n, true)
			resolveSweep(h, c, t, tc.extra)
			if len(h.posed) != 0 {
				t.Fatalf("asks = %d, want 0", len(h.posed))
			}
			for _, id := range yard { // the sweep moved the cards: not vacuous
				if o := h.g.Obj(id); o == nil || o.Zone != state.ZLibrary {
					t.Fatalf("precondition: %v did not reach the library: %+v", id, o)
				}
			}
		})
	}
	t.Run("HiddenPickPosesNothing", func(t *testing.T) {
		h, c, yard, _ := placementFixture(t, 3, true)
		for _, id := range yard {
			events.Apply(h.g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZLibrary, Player: 0})
		}
		cz := ChangeZoneOf(sa(t, "DB$ ChangeZone | Hidden$ True | Origin$ Graveyard | Destination$ Library | LibraryPosition$ -1"))
		if !cz.Hidden {
			t.Fatal("precondition: Hidden$ not parsed")
		}
		placeTargetedLibraryObjects(h, c, cz, yard)
		if len(h.posed) != 0 {
			t.Fatalf("asks = %d, want 0", len(h.posed))
		}
	})
	t.Run("TargetedGroupPosesOne", func(t *testing.T) {
		h, c, yard, _ := placementFixture(t, 3, true)
		for _, id := range yard {
			events.Apply(h.g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZLibrary, Player: 0})
		}
		cz := ChangeZoneOf(sa(t, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Library | LibraryPosition$ 0"))
		placeTargetedLibraryObjects(h, c, cz, yard)
		if len(h.posed) != 1 {
			t.Fatalf("asks = %d, want 1", len(h.posed))
		}
		if got := h.g.Zone(state.ZLibrary, 0)[:3]; !slices.Equal(got, []state.ObjID{yard[2], yard[1], yard[0]}) {
			t.Fatalf("top = %v, want the reversed answer", got)
		}
	})
}
