package effects

// Task diffcount2: the two remaining distinct-set aggregate spellings on the
// Count$Valid<zone> head, siblings of the Different* family (diffcount1) and
// the CardTypes/Colors reads:
//
//   - $CreatureType -- distinct creature subtypes among the matches (Valiant
//     Changeling's per-type reduction, /LimitMax.5; Hoshi Sato Exolinguist's
//     Federation dig; Saavik's /LimitMax.10).
//   - $CardTypesPermanent -- distinct CR 205.2 PERMANENT types among the
//     matches (Korvold, Gleeful Glutton's combat-damage trigger;
//     Matzalantli, the Great Door's transform gate -- whose oracle text
//     names the six).
//
// Both were whole-token fail-closed to zero before this.

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

const (
	aggTypeBear   = "Name:Grizzly\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	aggTypeBear2  = "Name:Kodiak\nManaCost:2 G\nTypes:Creature Bear\nPT:3/3\nOracle:x\n"
	aggTypeElf    = "Name:Llanowar\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"
	aggTypeGoblin = "Name:Mogg\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"
	aggTypeBolt   = "Name:Bolt\nManaCost:R\nTypes:Instant\nOracle:x\n"
	aggTypeMox    = "Name:Mox\nManaCost:0\nTypes:Artifact\nOracle:x\n"
	aggTypeLand   = "Name:Hill\nTypes:Land Mountain\nOracle:x\n"
)

// TestEvalCountCreatureTypeDedupsSubtypes pins the creature-type spelling:
// three creatures of types Bear, Elf and Goblin count THREE; a second Bear
// does not add a fourth.
func TestEvalCountCreatureTypeDedupsSubtypes(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	diffObject(t, g, 0, state.ZBattlefield, aggTypeBear)
	diffObject(t, g, 0, state.ZBattlefield, aggTypeElf)
	diffObject(t, g, 0, state.ZBattlefield, aggTypeGoblin)
	// PRECONDITION: the three creatures really are on the battlefield.
	if got := len(g.Zone(state.ZBattlefield, 0)); got != 3 {
		t.Fatalf("battlefield holds %d permanents, want 3", got)
	}
	const body = "Count$Valid Creature.YouCtrl$CreatureType"
	if got := EvalCount(h, c, body); got != 3 {
		t.Fatalf("%s = %d, want 3 distinct creature types", body, got)
	}
	diffObject(t, g, 0, state.ZBattlefield, aggTypeBear2)
	if got := EvalCount(h, c, body); got != 3 {
		t.Fatalf("%s after a duplicate Bear = %d, want still 3", body, got)
	}
}

// TestEvalCountCreatureTypeLimitMaxClamps pins the clamp: Valiant Changeling
// caps its reduction at {5}, Saavik at {10}, so a /LimitMax.<n> suffix on a
// CreatureType body must clamp the distinct count. (Before diffcount2 the
// suffix was only recognised for Colors, so an unclamped CreatureType read
// would have widened the discount past the card's own cap.)
func TestEvalCountCreatureTypeLimitMaxClamps(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	diffObject(t, g, 0, state.ZBattlefield, aggTypeBear)
	diffObject(t, g, 0, state.ZBattlefield, aggTypeElf)
	diffObject(t, g, 0, state.ZBattlefield, aggTypeGoblin)
	const body = "Count$Valid Creature.YouCtrl$CreatureType/LimitMax.2"
	if got := EvalCount(h, c, body); got != 2 {
		t.Fatalf("%s = %d, want 2 (3 distinct types clamped to the cap)", body, got)
	}
}

// TestEvalCountCardTypesPermanentExcludesNonpermanentTypes pins the
// permanent-type spelling against the plain CardTypes read on the same
// board: a graveyard holding a creature, an artifact, a land and an INSTANT
// reads 4 distinct card types but 3 distinct permanent types -- the instant
// contributes to neither Korvold's trigger nor Matzalantli's gate.
func TestEvalCountCardTypesPermanentExcludesNonpermanentTypes(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	diffObject(t, g, 0, state.ZGraveyard, aggTypeBear)
	diffObject(t, g, 0, state.ZGraveyard, aggTypeMox)
	diffObject(t, g, 0, state.ZGraveyard, aggTypeLand)
	diffObject(t, g, 0, state.ZGraveyard, aggTypeBolt)
	// PRECONDITION: the four cards really are in the graveyard.
	if got := len(g.Zone(state.ZGraveyard, 0)); got != 4 {
		t.Fatalf("graveyard holds %d cards, want 4", got)
	}
	if got := EvalCount(h, c, "Count$ValidGraveyard Card.YouOwn$CardTypes"); got != 4 {
		t.Fatalf("plain CardTypes = %d, want 4 (creature, artifact, land, instant)", got)
	}
	if got := EvalCount(h, c, "Count$ValidGraveyard Card.YouOwn$CardTypesPermanent"); got != 3 {
		t.Fatalf("CardTypesPermanent = %d, want 3 (the instant is not a permanent type)", got)
	}
}
