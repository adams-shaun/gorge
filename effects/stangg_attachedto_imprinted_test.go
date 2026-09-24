package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestStanggEchoWarriorCopyAttachesToTheImprintedTwin pins the one corpus
// `AttachedTo$ Imprinted` leg of the ImprintTokens$ True class end to end at
// the effects boundary, on the real token face (Stangg, Echo Warrior's
// `SVar:TrigStangg:DB$ Token | TokenScript$ stangg_twin | ImprintTokens$
// True | ... | SubAbility$ CreateCopy` followed by `SVar:CreateCopy:DB$
// CopyPermanent | Defined$ Valid Equipment.Attached,Aura.Attached |
// AttachedTo$ Imprinted`). It is the only one of the 14 corpus carriers of
// `ImprintTokens$ True` that reads the pile back through `AttachedTo$`
// (measured with GNU grep over .cards/cardsfolder at the current FORGE_REF;
// 11 use `Defined$ Imprinted`/`ImprintCards$ Imprinted`, 2 use
// `RememberObjects$ ImprintedLKI`), so its sibling spellings already have
// their own pins -- TestImprintTokensBindsCreatedTokensToTheSource and
// TestCopyPermanentImprintTokensDelTrigReadBack -- and this is the leg those
// two explicitly leave uncovered.
//
// Two merged changes make this chain work, and this test is what protects
// them from a silent refactor:
//
//   - commit fd12eabf ("feat(effects): bind ImprintTokens$ to the creating
//     source") taught definedSpec's Imprinted/ImprintedLKI cases to read
//     Object.ImprintTokens through the shared imprintPileTargets resolver
//     (effects/context.go), so the pile the trigger just wrote is what
//     `AttachedTo$ Imprinted` resolves; and
//   - agent-20260921T012459Z-a863ff76 (merged) resolves CopyPermanent's
//     `AttachedTo$` endpoint through Defined (effects/copypermanent.go), so
//     the named imprint pile becomes the minted copy's Attach target.
//
// This test can fail three ways and each is the point: the trigger's mint
// does not write the token onto the source's ImprintTokens pile (no imprint
// written), the CopyPermanent legs mints no copy at all (no copy minted), or
// the copy enters unattached because the endpoint resolved to nothing (copy
// unattached) -- exactly the symptom the ticket was filed for.
func TestStanggEchoWarriorCopyAttachesToTheImprintedTwin(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	h, c := fixtureHostWithTokens(t)

	// The token face comes from the registry's Tokens map keyed by file stem
	// -- the way TokenScript$ resolves it. reg.Lookup must NOT resolve tokens.
	twinCard, ok := reg.Tokens["stangg_twin"]
	if !ok {
		t.Fatalf("corpus registry has no token script %q", "stangg_twin")
	}
	if twinCard.Faces[0].Name != "Stangg Twin" {
		t.Fatalf("precondition: stangg_twin face is %q, want Stangg Twin", twinCard.Faces[0].Name)
	}
	h.g.Tokens["stangg_twin"] = twinCard

	stangg := corpusObject(t, reg, h.g, "Stangg, Echo Warrior")
	if stangg == nil || stangg.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Stangg must be on the battlefield, got %+v", stangg)
	}
	c.Source = stangg.ID
	c.Controller = 0

	// A real Equipment attached to Stangg is the copy's `Defined$ Valid
	// Equipment.Attached,Aura.Attached` selector target. The AttachedTo write
	// is the same test-fixture shape attached_predicate_test.go uses.
	collar := corpusObjectInSeat(t, reg, h.g, "Basilisk Collar", 0, state.ZBattlefield)
	collar.AttachedTo = stangg.ID
	if live := h.Game().Obj(collar.ID); live.AttachedTo != stangg.ID {
		t.Fatalf("precondition: collar.AttachedTo = %d, want Stangg %d", live.AttachedTo, stangg.ID)
	}

	// Leg 1 -- the trigger's mint (`DB$ Token ... ImprintTokens$ True`). The
	// source is Stangg, and the mint must bind the fresh Twin to Stangg's own
	// ImprintTokens pile, NOT to the token.
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{
		"TokenScript":    "stangg_twin",
		"TokenOwner":     "You",
		"ImprintTokens":  "True",
		"RememberTokens": "True",
		"TokenAttacking": "True",
		"TokenTapped":    "True",
	}})

	bf0 := h.Game().Zone(state.ZBattlefield, 0)
	var twin state.ObjID
	for _, id := range bf0 {
		if o := h.Game().Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Stangg Twin" {
			twin = id
		}
	}
	if twin == 0 {
		t.Fatalf("no Stangg Twin on the battlefield after the mint; battlefield = %v", bf0)
	}
	if twin == stangg.ID || twin == collar.ID {
		t.Fatal("precondition failed: the minted Twin is one of the existing permanents; the assertions below would be vacuous")
	}
	// Assert its own precondition: the token is live and in the zone the
	// resolver reads (battlefield; ImprintTokens has no zone filter).
	if o := h.Game().Obj(twin); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: minted Twin %d is not a battlefield object: %+v", twin, o)
	}
	// The imprint write (legs 2/3 depend on it).
	if got := h.Game().Obj(stangg.ID).ImprintTokens; len(got) != 1 || got[0] != twin {
		t.Fatalf("no imprint written: Stangg.ImprintTokens = %v, want [%d] (the minted Twin)", got, twin)
	}

	// Leg 2 -- the copy (`DB$ CopyPermanent ... AttachedTo$ Imprinted`).
	before := len(h.log)
	Resolve(h, c, &cards.SA{Kind: "DB", API: "CopyPermanent", Params: map[string]string{
		"Defined":        "Valid Equipment.Attached,Aura.Attached",
		"AttachedTo":     "Imprinted",
		"RememberTokens": "True",
	}})

	var copyID state.ObjID
	for _, id := range h.Game().Zone(state.ZBattlefield, 0) {
		if id == twin || id == collar.ID {
			continue
		}
		if o := h.Game().Obj(id); o != nil && o.IsToken {
			copyID = id
		}
	}
	if copyID == 0 {
		t.Fatalf("no copy minted: battlefield = %v, log tail = %+v", h.Game().Zone(state.ZBattlefield, 0), h.log[before:])
	}
	if o := h.Game().Obj(copyID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: minted copy %d is not a battlefield object: %+v", copyID, o)
	}
	// The copy must be attached to the imprinted Twin, not unattached.
	liveCopy := h.Game().Obj(copyID)
	if liveCopy.AttachedTo != twin {
		t.Fatalf("copy unattached: copy %d AttachedTo = %d, want the imprinted Twin %d (log tail %+v)",
			copyID, liveCopy.AttachedTo, twin, h.log[before:])
	}
	// And the attachment was made through the event log, not by a direct
	// field write: an Attach event naming the copy and the Twin.
	attached := false
	for _, ev := range h.log[before:] {
		if ev.Kind == events.Attach && ev.Obj == copyID && len(ev.IDs) > 0 && ev.IDs[0] == twin {
			attached = true
		}
	}
	if !attached {
		t.Fatalf("no events.Attach for copy %d -> Twin %d; log tail = %+v", copyID, twin, h.log[before:])
	}
}
