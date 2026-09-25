package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Press the Enemy can target either a spell or a nonland permanent. Forge's
// SpellTargeted is specifically the former; its parallel Targeted ref includes
// either target kind.
func TestSpellTargetedExcludesNonSpellTarget(t *testing.T) {
	card, ok := testutil.CorpusRegistry(t).Lookup("Press the Enemy")
	if !ok {
		t.Fatal("corpus missing Press the Enemy")
	}
	body := card.Faces[0].SVars["X"]
	if body != "SpellTargeted$CardManaCostLKI" {
		t.Fatalf("Press the Enemy SVar X = %q, want SpellTargeted$CardManaCostLKI", body)
	}
	if got := card.Faces[0].SVars["Y"]; got != "Targeted$CardManaCostLKI" {
		t.Fatalf("Press the Enemy SVar Y = %q, want Targeted$CardManaCostLKI", got)
	}

	h, c := fixtureHost(t)
	permanent := h.g.AddObject(mkCard(t, "Name:Target Permanent\nManaCost:4\nTypes:Artifact\nOracle:x\n"), 1)
	h.g.Obj(permanent.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 1, []state.ObjID{permanent.ID})
	c.Targets = []state.Target{{Obj: permanent.ID}}

	// The binding is a live battlefield permanent, not a stack spell, and its
	// mana value differs from the resolving source's. Thus SpellTargeted must
	// produce the legitimate zero while Targeted reads four.
	target := h.g.Obj(permanent.ID)
	if target == nil || target.Zone != state.ZBattlefield || target.Face() == nil || target.Face().Cmc() != 4 {
		t.Fatalf("precondition: target is not a battlefield permanent with mana value 4: %+v", target)
	}
	if src := h.g.Obj(c.Source); src != nil && src.Face() != nil && src.Face().Cmc() == 4 {
		t.Fatal("precondition: source mana value equals target's; comparison would not distinguish bindings")
	}
	if got := EvalCount(h, c, body); got != 0 {
		t.Errorf("%s on permanent target = %d, want 0 (no targeted spell)", body, got)
	}
	if got := EvalCount(h, c, card.Faces[0].SVars["Y"]); got != 4 {
		t.Errorf("%s on permanent target = %d, want 4", card.Faces[0].SVars["Y"], got)
	}
	if _, ok := EvalCountOK(h, c, body); !ok {
		t.Error("SpellTargeted on a non-spell target was not recognized as a legitimate zero")
	}
}
