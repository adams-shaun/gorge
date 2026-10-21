// clone_exceptions_test.go pins CR 707.9a/b for api:Clone: a Clone effect's
// exceptions and granted abilities are part of the copy's copiable values,
// so a copy of that copy (CR 707.3) has them too.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// imageCopyingBears casts Phantasmal Image as a copy of seat 0's Grizzly
// Bears and returns (image, bears).
func imageCopyingBears(t *testing.T, e *Engine) (state.ObjID, state.ObjID) {
	t.Helper()
	bears := findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	img := findCardObj(t, e, 0, "Phantasmal Image", state.ZHand)
	addMana(t, e, 0, "CU")
	castCopyCard(t, e, img, bears, 0)
	if o := e.G.Obj(img); o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" || !e.hasType(o, "Illusion") {
		t.Fatal("setup: Phantasmal Image is not an Illusion Grizzly Bears")
	}
	return img, bears
}

// TestCloneOfAPhantasmalImageKeepsItsExceptions is CR 707.9a/b: Phantasmal
// Image's exceptions ("it's an Illusion in addition to its other types and it
// has 'When this creature becomes the target of a spell or ability, sacrifice
// it.'") are part of the Image's copiable values, so a Clone that copies the
// Image (which is copying Grizzly Bears) is an Illusion Bear with the
// sacrifice trigger.
func TestCloneOfAPhantasmalImageKeepsItsExceptions(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1404, []string{"Clone", "Phantasmal Image", "Grizzly Bears"}, nil, nil)
	img, _ := imageCopyingBears(t, e)
	clone := findCardObj(t, e, 0, "Clone", state.ZHand)
	addMana(t, e, 0, "CCCU")
	castCopyCard(t, e, clone, img, 0)

	o := e.G.Obj(clone)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("Clone is not a Grizzly Bears copy: zone %v", o.Zone)
	}
	if !e.hasType(o, "Illusion") {
		t.Fatal("Clone of a Phantasmal Image is not an Illusion: the AddTypes$ exception is not a copiable value (CR 707.9b)")
	}
	trig := false
	for _, tr := range o.Face().Triggers {
		if tr.Mode == "BecomesTarget" && tr.Effect != nil {
			trig = true
		}
	}
	if !trig {
		t.Fatal("Clone of a Phantasmal Image lacks the becomes-target sacrifice trigger (CR 707.9a)")
	}
	replayCheck(t, e, cfg)
}

// TestPhantasmalImageCopyIsSacrificedWhenTargeted is the guard that the
// granted trigger still WORKS once it lives on the copy's face: targeting
// the Image with Giant Growth sacrifices it.
func TestPhantasmalImageCopyIsSacrificedWhenTargeted(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1410, []string{"Phantasmal Image", "Grizzly Bears", "Giant Growth"}, nil, nil)
	img, _ := imageCopyingBears(t, e)
	growth := findCardObj(t, e, 0, "Giant Growth", state.ZHand)
	addMana(t, e, 0, "G")
	castCopyCard(t, e, growth, 0, img)
	if o := e.G.Obj(img); o.Zone != state.ZGraveyard {
		t.Fatalf("targeted Phantasmal Image copy is in %s, want the graveyard (its granted trigger sacrificed it)", o.Zone)
	}
	replayCheck(t, e, cfg)
}
