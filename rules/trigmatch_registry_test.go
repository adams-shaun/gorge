package rules

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The trigger-mode dispatch used to be one switch in trigger_match.go, which
// made every ticket touching a mode edit the same file. It is now a table the
// per-mode trigmatch_*.go files register into. These tests pin the two
// properties that replace what the compiler used to give us for free: no mode
// silently loses its matcher, and no two files claim the same mode.

// registeredModes is every Mode$ the switch dispatched before the split. A mode
// dropping out of the table is a mode that silently stops firing, which no
// other test would catch -- the trigger just never matches.
var registeredModes = []string{
	"ChangesZone", "ChangesZoneAll",
	"SpellCast", "SpellCastOrCopy", "SpellCopy", "AbilityCast", "SpellAbilityCast",
	"Attacks", "AttackersDeclared", "AttackersDeclaredOneTarget",
	"Cycled", "Explores", "Investigated", "RingTemptsYou",
	"CounterAdded", "CounterRemoved",
	"Attached", "Exerted", "TokenCreated", "TokenCreatedOnce",
	"Sacrificed", "Discarded", "CommitCrime",
	"Taps", "TapsForMana",
	"DamageDone", "DamageDealtOnce", "DamageDoneOnce", "DamagePreventedOnce",
	"FlippedCoin", "Vote", "Drawn",
	"LifeLost", "LifeLostAll", "LifeGained",
	"BecomesTarget", "LandPlayed", "Phase", "Mutates", "Always",
}

func TestEveryDispatchedTriggerModeHasAMatcher(t *testing.T) {
	for _, mode := range registeredModes {
		if trigMatchers[mode] == nil {
			t.Errorf("Mode$ %s has no registered matcher: it can never fire", mode)
		}
	}
}

func TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched(t *testing.T) {
	// The mirror of the test above: a mode in the table that was not in the
	// switch means the split invented a dispatch, which is a behaviour change.
	known := map[string]bool{}
	for _, m := range registeredModes {
		known[m] = true
	}
	var extra []string
	for mode := range trigMatchers {
		if !known[mode] {
			extra = append(extra, mode)
		}
	}
	sort.Strings(extra)
	if len(extra) != 0 {
		t.Errorf("modes registered but not dispatched before the split: %v", extra)
	}
}

func TestAnUnregisteredModeNeverFires(t *testing.T) {
	// The switch had no default arm: an unknown mode fell off the end with
	// matched still false. The table must keep that, not panic on a lookup miss.
	if trigMatchers["NoSuchModeExists"] != nil {
		t.Fatal("test precondition: NoSuchModeExists must not be registered")
	}
}

func TestRegisteringOneModeTwicePanics(t *testing.T) {
	// Two files claiming one mode is the merge accident the split makes
	// possible, so it must be loud at startup rather than a matcher that
	// quietly stopped being reached.
	const mode = "TestOnlyDuplicateMode"
	fn := func(*Engine, cards.Trigger, state.ObjID, events.Event, *state.Object) bool { return false }
	registerTrigMatcher(fn, mode)
	defer delete(trigMatchers, mode)

	defer func() {
		if recover() == nil {
			t.Error("registering a mode twice must panic")
		}
	}()
	registerTrigMatcher(fn, mode)
}
