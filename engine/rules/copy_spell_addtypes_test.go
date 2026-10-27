package rules

// CopySpellAbility AddTypes$ (Tawnos, the Toymaker and Donal, Herald of Wings):
// "copy it, except it's an artifact/Spirit in addition to its other types".
// The exception is a copy-effect characteristic (CR 707.9a/b), so it is applied
// as the CopyExcept event Clone/CopyPermanent already use, not as a layer-4
// special case.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestTawnosCopyIsArtifactInAdditionToItsTypes(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	tawnos := lookup(t, reg, "Tawnos, the Toymaker")
	bird := lookup(t, reg, "Birds of Paradise")
	e, cfg := tokenReplGame(t, 20261027, tawnos, bird)

	doc := moveSeededCard(t, e, 0, tawnos, state.ZBattlefield)
	spell := moveSeededCard(t, e, 0, bird, state.ZStack)
	sa := cards.ResolveSVar(e.G.Obj(doc).Face().SVars, "TrigCopy")
	if sa == nil || sa.API != "CopySpellAbility" || sa.Params["AddTypes"] != "Artifact" {
		t.Fatalf("precondition: Tawnos TrigCopy = %+v, want CopySpellAbility with AddTypes$ Artifact", sa)
	}
	if derivedHasType(e, spell, "Artifact") {
		t.Fatal("precondition: the copied spell is already an artifact")
	}
	before := len(e.G.Objs)
	effects.Resolve(e, &effects.Ctx{Source: doc, Controller: 0,
		Remembered: []state.Target{{Obj: spell}}}, sa)
	if got := len(e.G.Objs) - before; got != 1 {
		t.Fatalf("precondition: minted %d copies, want one", got)
	}
	cp := e.G.Objs[before].ID
	if o := e.G.Obj(cp); o == nil || o.Zone != state.ZStack || !o.IsCopy {
		t.Fatalf("precondition: copy %d = %+v, want a stack copy", cp, o)
	}
	if !derivedHasType(e, cp, "Creature") {
		t.Fatalf("precondition: copy types %v lack Creature", e.Derived(cp).Types)
	}
	if !derivedHasType(e, cp, "Artifact") {
		t.Fatalf("copy types %v; want Artifact in addition to its other types", e.Derived(cp).Types)
	}
	e.resolveTop()
	if o := e.G.Obj(cp); o == nil || o.Zone != state.ZBattlefield || !o.IsToken || !derivedHasType(e, cp, "Artifact") {
		t.Fatalf("resolved copy = %+v, types %v; want an artifact token on the battlefield", o, e.Derived(cp).Types)
	}
	replayCheck(t, e, cfg)
}
