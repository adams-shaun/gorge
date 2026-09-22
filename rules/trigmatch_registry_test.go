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
	"CounterAdded", "CounterAddedOnce", "CounterRemoved",
	"Attached", "Exerted", "TokenCreated", "TokenCreatedOnce",
	"Sacrificed", "Discarded", "CommitCrime",
	"Taps", "TapsForMana",
	"DamageDone", "DamageDealtOnce", "DamageDoneOnce", "DamagePreventedOnce",
	"FlippedCoin", "Vote", "Drawn",
	"LifeLost", "LifeLostAll", "LifeGained",
	"BecomesTarget", "LandPlayed", "Phase", "Mutates", "Always",
}

// addedAfterTheSplit names every mode registered into the table since the
// split, each with the ticket that added it. The split did not invent these
// dispatches -- they are deliberate new trigger modes whose events did not
// exist before the split -- but a new mode must land here to be legal, which
// keeps the addition explicit and reviewable instead of silent. A mode NOT on
// either list is the merge accident the mirror test exists to catch.
var addedAfterTheSplit = []string{
	// trigdisc1: "Whenever you discover ..." (CR 701.57; Val, Marooned
	// Surveyor; Curator of Sun's Creation) and "Whenever you seek one or more
	// cards ..." (Vexyr, Ich-Tekik's Heir; Val; Lurker in the Deep). They
	// match the events.Discover / events.Seek marker Kinds, which were
	// appended for them, so neither mode could have been in the pre-split
	// switch.
	"Discover", "SeekAll",
	// connive1: "Whenever a creature you control connives ..." (CR 702.59;
	// Iron Monger Sadistic Tycoon, Glorious Purpose, Ultron Unlimited). It
	// matches the events.Connive marker Kind, which was appended for it, so
	// it could not have been in the pre-split switch.
	"Connives",
}

func allRegisteredModeNames() []string {
	all := make([]string, 0, len(registeredModes)+len(addedAfterTheSplit))
	all = append(all, registeredModes...)
	all = append(all, addedAfterTheSplit...)
	return all
}

func TestEveryDispatchedTriggerModeHasAMatcher(t *testing.T) {
	for _, mode := range allRegisteredModeNames() {
		if trigMatchers[mode] == nil {
			t.Errorf("Mode$ %s has no registered matcher: it can never fire", mode)
		}
	}
}

func TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched(t *testing.T) {
	// The mirror of the test above: a mode in the table that is on neither
	// the pre-split switch's list nor the documented post-split additions
	// means a dispatch appeared without review -- which is a behaviour
	// change. A legitimate new mode joins addedAfterTheSplit with its ticket
	// rather than broadening this test.
	known := map[string]bool{}
	for _, m := range allRegisteredModeNames() {
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
		t.Errorf("modes registered that the split never dispatched and no ticket added: %v", extra)
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
