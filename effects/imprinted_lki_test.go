package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestIsImprintedMatchesZoneChangeLKICandidate is the regression for the
// IsImprinted object predicate against the LKI candidate a zone-change trigger
// supplies (CR 603.10).
//
// Knowledge Pool exiles cards with Imprint$ True (state.Object.Imprinted) and
// carries the printed departure line
//
//	T:Mode$ ChangesZone | Origin$ Exile | Destination$ Any |
//	Static$ True | ValidCard$ Card.IsImprinted | Execute$ DBForget
//
// exactly the shape `wordImprinted`'s filter branch judges. A zone-change
// trigger holds the card's pre-move snapshot, still in exile, so the predicate
// must read the imprint association's exile liveness from the CANDIDATE's own
// zone, not re-read the live object -- which the MoveZone fold has already
// placed in the destination zone. The reader ignores its supplied candidate and
// re-reads g.Obj(id), so before the fix the LKI candidate read dead.
//
// The two adjacent contracts must NOT move, and this test pins them:
//
//   - An ordinary live filter still expires once the linked card leaves exile
//     (the CR 607.2a rule: the persistent ID cannot follow it).
//   - `Defined$ Imprinted` (imprintPileTargets) stays live-zone based, so it
//     never resolves a stale association.
func TestIsImprintedMatchesZoneChangeLKICandidate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	source := corpusObject(t, reg, g, "Knowledge Pool")
	linked := corpusObject(t, reg, g, "Grizzly Bears")
	bystander := corpusObject(t, reg, g, "Grizzly Bears")

	// g.AddObject's returned pointer is not the object the game stores; the
	// canonical objects must be re-fetched by id before any field write.
	source = g.Obj(source.ID)
	linked = g.Obj(linked.ID)
	bystander = g.Obj(bystander.ID)
	if source == nil || linked == nil || bystander == nil {
		t.Fatal("precondition: source, linked and bystander must resolve through g.Obj")
	}

	source.Imprinted = []state.ObjID{linked.ID}
	linked.Zone = state.ZExile
	g.SetZone(state.ZExile, linked.Owner, []state.ObjID{linked.ID})
	// The LKI snapshot a zone-change trigger would have captured before the
	// move -- the object as it was while still in exile.
	lki := linked.CloneDeep()

	sc := SpecContext{You: 0, Source: source.ID}

	// Preconditions: the association is real, the candidate is the linked
	// card, and the LKI snapshot is in the zone the ordinary rule reads.
	if source.ID == linked.ID || linked.ID == bystander.ID {
		t.Fatalf("precondition: source %d, linked %d and bystander %d must be distinct", source.ID, linked.ID, bystander.ID)
	}
	if len(source.Imprinted) != 1 || source.Imprinted[0] != linked.ID {
		t.Fatalf("precondition: source Imprinted=%v, want exactly [%d]", source.Imprinted, linked.ID)
	}
	if lki.Zone != state.ZExile {
		t.Fatalf("precondition: LKI snapshot zone=%v, want exile", lki.Zone)
	}
	if !MatchesObjectCtx(g, "Card.IsImprinted", linked, sc) {
		t.Fatal("precondition: Card.IsImprinted must match the linked card while it is in exile")
	}
	if MatchesObjectCtx(g, "Card.IsImprinted", bystander, sc) {
		t.Fatal("precondition: Card.IsImprinted must not match an unlinked bystander")
	}

	// Simulate the MoveZone fold: the live linked object leaves exile. The LKI
	// snapshot is untouched and still reads exile.
	linked.Zone = state.ZHand
	g.SetZone(state.ZExile, linked.Owner, nil)
	g.SetZone(state.ZHand, linked.Owner, []state.ObjID{linked.ID})
	if linked.Zone == state.ZExile {
		t.Fatal("precondition: the live linked card must have left exile")
	}
	if linked.Zone == lki.Zone {
		t.Fatalf("precondition: live zone %v and LKI zone %v must actually differ", linked.Zone, lki.Zone)
	}

	// The trigger route: the LKI candidate is what the zone-change matcher
	// hands the predicate, and it must still match.
	if !MatchesObjectCtx(g, "Card.IsImprinted", &lki, sc) {
		t.Fatal("Card.IsImprinted must match an imprinted card leaving exile when judged against its pre-move LKI candidate")
	}

	// The LKI of an unrelated card is not in the association.
	bystanderLKI := bystander.CloneDeep()
	bystanderLKI.Zone = state.ZExile
	if MatchesObjectCtx(g, "Card.IsImprinted", &bystanderLKI, sc) {
		t.Error("Card.IsImprinted matched an unlinked bystander's LKI")
	}

	// Preserved contract 1: an ordinary live read expires after leaving exile.
	if MatchesObjectCtx(g, "Card.IsImprinted", linked, sc) {
		t.Error("Card.IsImprinted must stop matching an ordinary live read after the linked card leaves exile")
	}

	// Preserved contract 2: Defined$ Imprinted is live-zone based and must not
	// resolve the stale association. Assert the reader is live first (it still
	// resolves the card while in exile) so a broken reader cannot pass this.
	g.SetZone(state.ZHand, linked.Owner, nil)
	linked.Zone = state.ZExile
	g.SetZone(state.ZExile, linked.Owner, []state.ObjID{linked.ID})
	if pile := imprintPileTargets(g, &Ctx{Source: source.ID}); len(pile) != 1 || pile[0].Obj != linked.ID {
		t.Fatalf("precondition: Defined$ Imprinted must resolve the imprinted card while it is in exile, got %+v", pile)
	}
	linked.Zone = state.ZHand
	g.SetZone(state.ZExile, linked.Owner, nil)
	g.SetZone(state.ZHand, linked.Owner, []state.ObjID{linked.ID})
	if pile := imprintPileTargets(g, &Ctx{Source: source.ID}); len(pile) != 0 {
		t.Errorf("Defined$ Imprinted resolved a stale association after the linked card left exile: %+v", pile)
	}
}
