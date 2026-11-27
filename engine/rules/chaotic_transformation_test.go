package rules

// Chaotic Transformation (DMU): "Exile up to one target artifact, ... For
// each permanent exiled this way, its controller reveals cards from the top
// of their library until they reveal a card that shares a card type with it,
// puts that card onto the battlefield, then shuffles."
//
// The chain is RepeatEach | RepeatPlayers$ ImprintedController -> RepeatEach |
// DefinedCards$ ImprintedLKI.RememberedPlayerCtrl -> DigUntil. Before the
// selector fix both RepeatEach selectors fell through: repeatPlayers knew no
// ImprintedController (ok=false, "RepeatEach selector unimplemented" Note,
// nobody digs) and repeatedCards folded ImprintedLKI onto the host-remembered
// list, dropping the .RememberedPlayerCtrl qualifier so the per-seat inner
// loop ran zero times.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const ctArtifactSrc = "Name:CT Test Artifact\nManaCost:1\nTypes:Artifact\nOracle:x\n"
const ctLibraryArtifactSrc = "Name:CT Library Artifact\nManaCost:1\nTypes:Artifact\nOracle:x\n"
const ctCreatureSrc = "Name:CT Test Creature\nManaCost:1\nTypes:Creature Test-Beast\nPT:2/2\nOracle:x\n"
const ctSpareArtifactSrc = "Name:CT Spare Artifact\nManaCost:1\nTypes:Artifact\nOracle:x\n"
const ctSpareCreatureSrc = "Name:CT Spare Creature\nManaCost:1\nTypes:Creature Test-Beast\nPT:2/2\nOracle:x\n"
const ctLibraryCreatureSrc = "Name:CT Library Creature\nManaCost:1\nTypes:Creature Test-Beast\nPT:2/2\nOracle:x\n"

// ctFind is seat p's object named name in zone z, or the test fails.
func ctFind(t *testing.T, e *Engine, p state.PlayerID, name string, z state.Zone) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(z, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("seat %d has no %q in %s", p, name, z)
	return 0
}

// ctInLibrary returns seat p's card named name, first moving it from the
// opening hand into the library if the shuffle dealt it there.
func ctInLibrary(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
		}
	}
	return ctFind(t, e, p, name, state.ZLibrary)
}

func TestChaoticTransformationEachControllerDigsToASharedType(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ct := lookup(t, reg, "Chaotic Transformation")
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{ct, card(t, ctArtifactSrc), card(t, ctLibraryArtifactSrc), card(t, ctSpareArtifactSrc)},
		[]*cards.Card{card(t, ctCreatureSrc), card(t, ctLibraryCreatureSrc), card(t, ctSpareCreatureSrc)})
	artID := moveByName(t, e, 0, "CT Test Artifact", state.ZBattlefield)
	creatureID := moveByName(t, e, 1, "CT Test Creature", state.ZBattlefield)
	moveByName(t, e, 0, "Chaotic Transformation", state.ZHand)
	addMana(t, e, 0, "RRRRRR")

	// Preconditions: each shared-type card starts in the library the DigUntil
	// reads (not the battlefield), and each permanent to be exiled starts on
	// the battlefield -- so the zones asserted below actually moved.
	libArtID := ctInLibrary(t, e, 0, "CT Library Artifact")
	libCreatureID := ctInLibrary(t, e, 1, "CT Library Creature")
	if o := e.G.Obj(libArtID); o.Zone != state.ZLibrary || o.Owner != 0 {
		t.Fatalf("precondition: seat 0's library artifact is %+v", o)
	}
	if o := e.G.Obj(libCreatureID); o.Zone != state.ZLibrary || o.Owner != 1 {
		t.Fatalf("precondition: seat 1's library creature is %+v", o)
	}
	// Spares: a second card of the type each seat lost, still in its library.
	// The .RememberedPlayerCtrl qualifier makes each seat dig exactly once; an
	// unfiltered pile runs both seats' digs once per pile card (twice) and
	// would put the spare on the battlefield too.
	spare0 := ctInLibrary(t, e, 0, "CT Spare Artifact")
	spare1 := ctInLibrary(t, e, 1, "CT Spare Creature")
	if o := e.G.Obj(artID); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: seat 0's artifact is %+v", o)
	}
	if o := e.G.Obj(creatureID); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: seat 1's creature is %+v", o)
	}

	castNamed(t, e, "Chaotic Transformation")
	drainDecisions(t, e, 400, func(d *decision.Decision) []int {
		if d.Kind == decision.KTarget {
			// The artifact and creature asks each offer the one legal
			// permanent; every other ask falls to the first-answer stand-in.
			for _, o := range d.Options {
				if o.Obj == artID || o.Obj == creatureID {
					return []int{o.Index}
				}
			}
		}
		return firstAnswer(d)
	})

	// The named permanents are exiled (the imprint seeds).
	if o := e.G.Obj(artID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("seat 0's artifact was not exiled: %+v", o)
	}
	if o := e.G.Obj(creatureID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("seat 1's creature was not exiled: %+v", o)
	}
	// Each controller's own shared-type card got dug to the battlefield under
	// them: seat 0's artifact and seat 1's creature.
	if o := e.G.Obj(libArtID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("seat 0 did not dig its artifact to the battlefield: %+v", o)
	}
	if o := e.G.Obj(libCreatureID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("seat 1 did not dig its creature to the battlefield: %+v", o)
	}
	if o := e.G.Obj(spare0); o == nil || o.Zone != state.ZLibrary {
		t.Fatalf("seat 0 dug for its artifact more than once: %+v", o)
	}
	if o := e.G.Obj(spare1); o == nil || o.Zone != state.ZLibrary {
		t.Fatalf("seat 1 dug for its creature more than once: %+v", o)
	}
	// Neither RepeatEach selector fell through: no "unimplemented" Note.
	if n := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.Note && strings.Contains(ev.Text, "RepeatEach selector unimplemented")
	}); n != 0 {
		t.Fatalf("%d RepeatEach selector-unimplemented Notes emitted", n)
	}
	replayCheck(t, e, cfg)
}
