package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFurgulTransformedDFCBackFacePower is the mana-ability half of the
// sacrificed-LKI class: Furgul, Quag Nurturer's "{T}, Sacrifice a creature:
// Add an amount of {G} equal to the sacrificed creature's power" must price
// the permanent as it last existed on the battlefield (CR 608.2h). A
// transforming DFC is reset to its front face by the sacrifice's Move
// (CR 712.8a), so a post-move capture reads Khenra Spellspear's front 2/2
// instead of the Gitaxian Spellstalker back face's 3/3 it had when it was
// sacrificed. The snapshot must therefore be taken before the move and
// threaded to the mana effect's Amount$ evaluation.
func TestFurgulTransformedDFCBackFacePower(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	furgul := corpusCommander(t, reg, "Furgul, Quag Nurturer")
	khenra := corpusCard(t, "Khenra Spellspear")
	e, cfg := eachColorGame(t, 401, []*cards.Card{furgul, khenra})
	fid := moveToBattlefieldByName(t, e, 0, "Furgul, Quag Nurturer")
	did := moveToBattlefieldByName(t, e, 0, "Khenra Spellspear")
	e.emit(events.Event{Kind: events.FlipFace, Obj: did, Amount: 1})
	if o := e.G.Obj(did); o.FaceIdx != 1 || o.Face().Name != "Gitaxian Spellstalker" || e.Power(did) != 3 {
		t.Fatalf("precondition: face %d name %q power %d", o.FaceIdx, o.Face().Name, e.Power(did))
	}
	if got := effects.SacrificedLKI(e, did); got.Power != 3 || got.Toughness != 3 {
		t.Fatalf("pre-move LKI = %d/%d, want 3/3", got.Power, got.Toughness)
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 1})
	e.priorityRound()
	activateMana(t, e, fid)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		submitChoices(t, e, sacrificeOption(t, d, did))
	}
	if o := e.G.Obj(did); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("sacrificed DFC zone = %v, want graveyard", o.Zone)
	}
	if got := e.G.Players[0].Pool[state.MG]; got != 3 {
		t.Fatalf("Furgul produced G=%d, want 3 (back face power)", got)
	}
	replayCheck(t, e, cfg)
}
