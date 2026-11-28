package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 614.12, 708.2a: an entry replacement judges a permanent entering face
// down by the characteristics it will have. One returned face down as a Forest
// land (Yedora, Grave Gardener's shape) is no creature, so "face-down
// creatures you control enter with a counter" (Veiled Ascension's shape) does
// not apply to it. Both fixtures are written by hand (never a corpus script).
const fdCounterStaticSrc = "Name:Fixture Ascension\nManaCost:W\nTypes:Enchantment\n" +
	"K:ETBReplacement:Other:Mark:Mandatory:Battlefield:Creature.faceDown+YouCtrl\n" +
	"SVar:Mark:DB$ PutCounter | ETB$ True | Defined$ ReplacedCard | CounterType$ Flying | CounterNum$ 1\n" +
	"Oracle:x\n"

func fdReturnSpell(setType string) string {
	return "Name:Fixture Return\nManaCost:G\nTypes:Instant\n" +
		"A:SP$ ChangeZone | ValidTgts$ Creature.YouOwn | Origin$ Graveyard | Destination$ Battlefield | FaceDown$ True" + setType + "\n" +
		"Oracle:x\n"
}

// returnFaceDownUnderAscension puts the counter enchantment on the battlefield
// and resolves spell on a creature card in its controller's graveyard, and
// returns that card.
func returnFaceDownUnderAscension(t *testing.T, seed uint64, spell string) *state.Object {
	t.Helper()
	e, _, _ := newFixtureDeckWithTokens(t, seed, spell)
	onBoard(t, e, 0, fdCounterStaticSrc)
	id := onBoard(t, e, 0, "Name:Fixture Goblin\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: the creature card is in %s, want the graveyard", z)
	}
	addMana(t, e, 0, "G")
	castFirst(t, e, "cast")
	flickerTarget(t, e, id)
	passUntilStackEmpty(t, e, 20)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || !o.FaceDown {
		t.Fatalf("precondition: the card is in %s (face down %v), want it returned face down", o.Zone, o.FaceDown)
	}
	return o
}

func TestAFaceDownLandEntryTakesNoFaceDownCreatureCounter(t *testing.T) {
	t.Parallel()
	o := returnFaceDownUnderAscension(t, 115, fdReturnSpell(" | FaceDownSetType$ Land & Forest"))
	if o.EffectiveIsCreature() {
		t.Fatalf("precondition: the returned permanent is a creature, want a Forest land (set type %q)", o.FaceDownSetType)
	}
	if n := o.Counter("Flying"); n != 0 {
		t.Fatalf("the face-down Forest land entered with %d flying counters, want 0 (CR 614.12)", n)
	}
}

// The control: the same return without a set type enters a face-down creature,
// which does take the counter.
func TestAFaceDownCreatureEntryTakesTheFaceDownCreatureCounter(t *testing.T) {
	t.Parallel()
	o := returnFaceDownUnderAscension(t, 116, fdReturnSpell(""))
	if n := o.Counter("Flying"); n != 1 {
		t.Fatalf("the face-down creature entered with %d flying counters, want 1", n)
	}
}
