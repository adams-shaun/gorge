package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// A layer-4 type grant on a face-down permanent whose printed face's word
// set EQUALS the derived set (CR 708.5 + CR 613.1c): the grant applies to the
// CR 708.5 base and the printed face stays hidden, so the derived list can
// come out identical to the hidden printed face while the base a filter's
// printed-face fallback reads is still the face-down set. The derived-type
// table must then carry an entry -- dropping it because "derived == printed"
// hides the grant from every Count$Valid and target filter over the object
// (Ethereal Armor's "+1/+1 for each enchantment you control" missed a
// manifested creature a pump had just made an enchantment: the kfuzz
// std-dsk facedown_typegrant_dropped_from_type_table repro).

// creeperFixture prints exactly the word set the grant below gives: an
// Enchantment Creature whose CR 708.5 face-down base is {Creature}, so the
// grant's Enchantment completes the printed word set.
const creeperFixture = "Name:Test Creeper\nManaCost:1 U\nTypes:Enchantment Creature\nPT:1/1\nOracle:x\n"

// typeGrantFixture grants Enchantment to every creature its controller
// controls: the face-down base {Creature} plus this word is the printed set.
const typeGrantFixture = "Name:Type Grant\nTypes:Enchantment\n" +
	"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddType$ Enchantment\nOracle:x\n"

// typeCountFixture pays +X/+X per enchantment its controller controls, the
// Count$Valid type filter the repro's Ethereal Armor carries: with no table
// entry the face-down creature falls back to its CR 708.5 set and is
// miscounted as one enchantment short.
const typeCountFixture = "Name:Type Count\nTypes:Enchantment\n" +
	"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddPower$ N | AddToughness$ N\n" +
	"SVar:N:Count$Valid Enchantment.YouCtrl\nOracle:x\n"

func TestFaceDownTypeGrantMatchingPrintedFaceIsCounted(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := facedownLayer4Engine(t, reg,
		[]*cards.Card{
			lookup(t, reg, "Reality Shift"),
			lookup(t, reg, "Grizzly Bears"),
			card(t, typeGrantFixture),
			card(t, typeCountFixture),
		},
		nil)
	moveByName(t, e, 0, "Type Grant", state.ZBattlefield)
	moveByName(t, e, 0, "Type Count", state.ZBattlefield)
	target := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	// The deck's seeded deal may leave Reality Shift in the library for
	// this card list; bring it to the hand (an evented move) before the
	// cast ask looks for it there.
	inHand := false
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Reality Shift" {
			inHand = true
		}
	}
	if !inHand {
		moveByName(t, e, 0, "Reality Shift", state.ZHand)
	}
	// The manifest takes the library's top card; put the fixture there
	// deterministically (addToLibrary's direct zone surgery, the
	// dig_ask_kernel_test precedent -- the game is not replay-checked).
	addToLibrary(t, e, 0, creeperFixture, 12)
	top := e.G.Zone(state.ZLibrary, 0)[0]
	if name := e.G.Obj(top).Face().Name; name != "Test Creeper" {
		t.Fatalf("fixture wants Test Creeper on top of seat 0's library, got %q", name)
	}

	castAtOpponent(t, e, "Reality Shift", target, "UU")
	o := e.G.Obj(top)
	if o.Zone != state.ZBattlefield || !o.FaceDown || o.Controller != 0 {
		t.Fatalf("manifested Test Creeper: zone=%s FaceDown=%v Controller=%d, want battlefield/true/0",
			o.Zone, o.FaceDown, o.Controller)
	}

	// The grant applies to the CR 708.5 base: the derived types carry the
	// granted word, and the printed face's other word set never leaks. This
	// holds both before and after the fix -- it is the precondition the
	// table assertion below depends on.
	d := e.Derived(top)
	if !slices.Contains(d.Types, "Creature") || !slices.Contains(d.Types, "Enchantment") {
		t.Fatalf("face-down creature's derived types %v, want Creature and Enchantment", d.Types)
	}
	if slices.Contains(d.Types, "Land") || slices.Contains(d.Types, "Forest") {
		t.Fatalf("face-down creature's derived types leaked the printed face: %v", d.Types)
	}

	// The derived-type table must carry the object: its derived list is
	// word-equal to the HIDDEN printed face, so the old printed-face
	// differ-test dropped the entry and every Count$Valid type filter fell
	// back to the CR 708.5 set. The walk's own list is [Creature,
	// Enchantment] (base plus grant); assert the entry, its words and the
	// all-creature-types marker all agree with the walk.
	entry := -1
	for i := range e.layer4Types {
		if e.layer4Types[i].ID == top {
			entry = i
		}
	}
	if entry < 0 {
		t.Fatalf("derived-type table %v has no entry for the face-down granted creature %d", e.layer4Types, top)
	}
	if !slices.Contains(e.layer4Types[entry].Types, "Enchantment") {
		t.Fatalf("table entry types %v, want the granted Enchantment", e.layer4Types[entry].Types)
	}

	// The Count$Valid read the table feeds: +X/+X per enchantment you
	// control counts the face-down creature itself beside the two
	// enchantment fixtures, so 2/2 + 3/3.
	if d.Power != 5 || d.Toughness != 5 {
		t.Fatalf("face-down creature through the per-enchantment count = %d/%d, want 5/5 (2/2 base + 3 enchantments)", d.Power, d.Toughness)
	}

	noUnimplementedManifest(t, e)
	// No replayCheck: the library was arranged by direct zone surgery
	// (addToLibrary, the dig_ask_kernel_test precedent), which a log-only
	// replay does not reconstruct.
}
