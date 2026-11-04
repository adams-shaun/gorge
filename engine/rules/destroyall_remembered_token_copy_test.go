package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDestroyAllRememberedTokenCopyCounts pins CR 111.7 and 707.10a: a token
// copy of a creature that a destroy-all puts into the graveyard still exists
// there until the next state-based check, so the same resolution's
// Remembered$Valid count reads it like any other destroyed creature. Join the
// Collective sweeps a creature and a token copy of it; both were nonartifact
// creatures, so it makes two Borg tokens. The count head used to reject the
// copy as already ceased and made one, while the remembered-set condition
// reader (and Forge, which remembers each destroyed card's battlefield state)
// counted it.
func TestDestroyAllRememberedTokenCopyCounts(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	amoeba := lookup(t, reg, "Giant Space Amoeba")
	dup := lookup(t, reg, "Electroduplicate")
	sweep := lookup(t, reg, "Join the Collective")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{amoeba, dup, sweep}, nil)

	amoebaID := moveByName(t, e, 0, "Giant Space Amoeba", state.ZBattlefield)
	dupID := moveByName(t, e, 0, "Electroduplicate", state.ZBattlefield)
	dupSA := e.G.Obj(dupID).Face().SpellAbility()
	if dupSA == nil || dupSA.API != "CopyPermanent" {
		t.Fatalf("precondition: Electroduplicate's CopyPermanent ability missing: %+v", dupSA)
	}
	effects.Resolve(e, &effects.Ctx{Source: dupID, Controller: 0,
		Targets: []state.Target{{Obj: amoebaID}}, TargetsOffered: true,
		SVars: e.G.Obj(dupID).Face().SVars}, dupSA)
	copyID := findTokenCopyOf(t, e, e.G.Obj(amoebaID).Card, amoebaID)
	if o := e.G.Obj(copyID); o == nil || !o.IsToken || !o.IsCopy || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the token copy %d is not on the battlefield", copyID)
	}

	sweepID := moveByName(t, e, 0, "Join the Collective", state.ZBattlefield)
	sweepSA := e.G.Obj(sweepID).Face().SpellAbility()
	if sweepSA == nil || sweepSA.API != "DestroyAll" {
		t.Fatalf("precondition: Join the Collective's DestroyAll ability missing: %+v", sweepSA)
	}
	effects.Resolve(e, &effects.Ctx{Source: sweepID, Controller: 0,
		SVars: e.G.Obj(sweepID).Face().SVars}, sweepSA)

	for _, id := range []state.ObjID{amoebaID, copyID} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("precondition: destroyed creature %d is not in the graveyard", id)
		}
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Borg Token"); got != 2 {
		t.Fatalf("Borg Tokens = %d, want 2: the destroyed token copy is a nonartifact creature until state-based actions remove it", got)
	}
	replayCheck(t, e, cfg)
}
