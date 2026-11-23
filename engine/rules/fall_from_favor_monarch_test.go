package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Fall from Favor's lock is a printed Untap replacement over
// "Creature.EnchantedBy+ControlledBy EnchantedController.!isMonarch": the
// enchanted creature stays tapped through its controller's untap step unless
// that player is the monarch. Inline copy of the printed line.
const fallFromFavorSrc = "Name:Fall from Favor\nManaCost:2 U\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
	"R:Event$ Untap | ValidCard$ Creature.EnchantedBy+ControlledBy EnchantedController.!isMonarch | ValidStepTurnToController$ You | Layer$ CantHappen | Description$ x\n" +
	"Oracle:x\n"

// fallFromFavorBoard puts seat 0's Fall from Favor on seat 1's tapped Bear.
func fallFromFavorBoard(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	aura := onBoard(t, e, 0, fallFromFavorSrc)
	bear := onBoard(t, e, 1, "Name:Lock Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{bear}})
	e.emit(events.Event{Kind: events.Tap, Obj: bear})
	if !e.G.Obj(bear).Tapped {
		t.Fatal("precondition: the bear is untapped")
	}
	e.pending = nil
	e.priorityRound()
	return e, bear
}

// TestFallFromFavorEnchantedCreatureUntapsOnlyForTheMonarch: the enchanted
// creature's controller is not the monarch (or nobody is), so it stays tapped
// through its untap step; when that controller is the monarch it untaps.
func TestFallFromFavorEnchantedCreatureUntapsOnlyForTheMonarch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		monarch  int // -1: nobody
		wantTapd bool
	}{
		{"nobody is the monarch", -1, true},
		{"the aura's controller is the monarch", 0, true},
		{"the creature's controller is the monarch", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, bear := fallFromFavorBoard(t)
			if tc.monarch >= 0 {
				e.emit(events.Event{Kind: events.MonarchChange, Player: state.PlayerID(tc.monarch)})
			}
			driveToStep(t, e, 2, 1, state.StepMain1)
			if got := e.G.Obj(bear).Tapped; got != tc.wantTapd {
				t.Fatalf("enchanted creature tapped after its untap step = %v, want %v", got, tc.wantTapd)
			}
		})
	}
}

// TestEnchantedControllerMonarchSpellingsAreKnown: the two EnchantedController
// spellings are known predicates, not ones that fail closed.
func TestEnchantedControllerMonarchSpellingsAreKnown(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		"Creature.EnchantedBy+ControlledBy EnchantedController.!isMonarch",
		"Creature.EnchantedBy+ControlledBy EnchantedController.isMonarch",
	} {
		if u := effects.UnknownPredicates(s); len(u) != 0 {
			t.Fatalf("%s: unknown predicates %v", s, u)
		}
	}
}
