package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The face-down ENTRY of a NONCREATURE card must still match a Moved
// replacement's `Creature.faceDown+...` ValidCard$ (CR 614.12, 708.2, 701.58a):
// a manifested or cloaked card enters as a 2/2 creature with no other types,
// and the replacement applies to the object AS IT WOULD ENTER. Veiled
// Ascension's "Face-down creatures you control enter with a flying counter on
// them" cloaks noncreature cards every game (its upkeep trigger cloaks the
// library's top card whatever it is), and gorge's type predicate used to read
// the origin-zone card's printed types at match time -- a cloaked Plains or
// instant answered Creature=no and got no counter. TestETBReplacementFilter
// VeiledAscension already pins the creature case; these two pin the
// noncreature one. The helpers come from etbrepl_filter_test.go and
// cloak_test.go; the decks are compiled corpus cards only.

// cloakByName moves one of seat p's NAMED cards onto p's battlefield with the
// exact logged MoveZone the effCloak primitive emits, returning the id. It is
// cloakBear generalized over the card name: the corpus cloak carriers keep
// their own tests.
func cloakByName(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, p) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, Player: p,
					From: z, To: state.ZBattlefield,
					Counter: "entered_cloaked", Secret: true})
				return id
			}
		}
	}
	t.Fatalf("seat %d has no %s in hand/library to cloak", p, name)
	return 0
}

// assertCloakedEntry pins the test's precondition -- the object is a face-down
// cloaked battlefield permanent that the log's cloak entry produced -- and the
// rule under test: exactly one Flying counter from Veiled Ascension's
// replacement. The precondition matters because a counter on a face-UP
// permanent would pass the count while testing nothing.
func assertCloakedEntry(t *testing.T, e *Engine, id state.ObjID, name string) {
	t.Helper()
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || !o.FaceDown || !o.Cloaked {
		t.Fatalf("cloaked %s: zone %s FaceDown %v Cloaked %v", name, o.Zone, o.FaceDown, o.Cloaked)
	}
	if n := counterCount(o, "Flying"); n != 1 {
		t.Fatalf("cloaked %s flying counters = %d, want 1 (the face-down entry replacement; CR 614.12, 708.2)", name, n)
	}
}

// TestETBReplacementFilterVeiledAscensionCloakedLand: a cloaked Plains is a
// 2/2 creature as it enters (CR 708.2), so Veiled Ascension's
// `Creature.faceDown+YouCtrl` replacement puts the flying counter on it.
func TestETBReplacementFilterVeiledAscensionCloakedLand(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t,
		"v/veiled_ascension.txt", "g/grizzly_bears.txt", "f/forest.txt", "m/mountain.txt", "p/plains.txt")
	e, cfg := etbreplEngine(t, reg, "Veiled Ascension", "Plains")
	castVanilla(t, e, "Veiled Ascension", "WWWW")
	plains := cloakByName(t, e, 0, "Plains")
	assertCloakedEntry(t, e, plains, "Plains")
	replayCheck(t, e, cfg)
}

// TestETBReplacementFilterVeiledAscensionCloakedInstant: a cloaked instant is
// also a creature as it enters. The card keeps its printed types under the
// face-down marker only in gorge's origin-zone read, which is the defect.
func TestETBReplacementFilterVeiledAscensionCloakedInstant(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t,
		"v/veiled_ascension.txt", "g/grizzly_bears.txt", "f/forest.txt", "m/mountain.txt", "g/giant_growth.txt")
	e, cfg := etbreplEngine(t, reg, "Veiled Ascension", "Giant Growth")
	castVanilla(t, e, "Veiled Ascension", "WWWW")
	bolt := cloakByName(t, e, 0, "Giant Growth")
	assertCloakedEntry(t, e, bolt, "Giant Growth")
	replayCheck(t, e, cfg)
}
