package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// chainInvestigateHost records every posed ask in order and marks the host
// Suspended after each, so Resolve stops descending into the SubAbility$ walk
// exactly as rules.Engine does (the fx42AskHost shape, reused here).
type chainInvestigateHost struct {
	fakeHost
	asks      []*decision.Decision
	suspended bool
}

func (h *chainInvestigateHost) Ask(d *decision.Decision) bool {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.asks = append(h.asks, &cp)
	h.suspended = true
	return true
}

func (h *chainInvestigateHost) Suspended() bool { return h.suspended }

// investigateChainBoard builds a 2-seat game whose game Tokens carry the real
// corpus Clue script (the registry effInvestigate mints from), a source
// permanent on the battlefield in seat 0's control, and a Ctx resolving for
// seat 0. The returned host records asks; ctx is the shared resolution
// context the whole chain re-enters through.
func investigateChainBoard(t *testing.T) (*chainInvestigateHost, *Ctx) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	h := &chainInvestigateHost{}
	h.g = state.NewGame(names(2))
	h.g.Tokens = reg.Tokens
	src := h.g.AddObject(creature(t, "Investigator"), 0)
	src.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
	return h, &Ctx{Source: src.ID, Controller: 0}
}

// TestChainedOptionalInvestigatePosesItsOwnElection pins the fx42 scoping
// contract effInvestigate's election walk must keep: a SECOND Optional$ True
// Investigate in the same resolution is its own election, not a consumer of
// the first walk's answer. The corpus reaches the repeated shape only through
// a hand-authored chain (no real card carries two optional Investigate
// sub-abilities), so the fixture is a synthetic two-sa chain — the shape a
// future card could introduce, the same reasoning the fx42 nested-counter pin
// documents for its own zero-corpus chain.
//
// Each SA's `Defined$ Opponent` resolves to the single opponent (seat 1). The
// outer walk's cursor (Ctx.InvestigateOptIdx) is reset when the walk
// finishes; without that reset the inner optional Investigate would resume at
// the outer walk's end index and pose NOTHING. So the chain poses exactly two
// elections (one per SA), each answered "yes", minting one Clue for seat 1
// and emitting one Investigate marker.
func TestChainedOptionalInvestigatePosesItsOwnElection(t *testing.T) {
	h, ctx := investigateChainBoard(t)

	inner := &cards.SA{Kind: "DB", API: "Investigate",
		Params: map[string]string{"Optional": "True", "Defined": "Opponent"}}
	outer := &cards.SA{Kind: "SP", API: "Investigate",
		Params: map[string]string{"Optional": "True", "Defined": "Opponent"}, Sub: inner}

	// Precondition: the Clue script is present, so effInvestigate does not
	// take its "unknown token script" early return, and no Clue exists yet.
	if h.g.Tokens[clueTokenKey] == nil {
		t.Fatalf("precondition: the corpus Clue script %q is missing from the game's token registry", clueTokenKey)
	}
	if got := clueCountOn(h, 1); got != 0 {
		t.Fatalf("precondition: %d Clue(s) already on seat 1", got)
	}

	// Outer walk: seat 1's election, answered "yes" on resume.
	Resolve(h, ctx, outer)
	if len(h.asks) != 1 {
		t.Fatalf("outer optional Investigate posed %d asks, want 1 (the opponent's election)", len(h.asks))
	}
	if h.asks[0].Kind != decision.KChoose || h.asks[0].Player != 1 {
		t.Fatalf("outer ask = %+v, want a KChoose to the opponent seat 1", h.asks[0])
	}
	ctx.InvestigateOpt = "yes"
	ctx.InvestigateOptIdx = 0 // the engine resumes at the SAME cursor index (ResumeTarget = idx)
	h.suspended = false
	Resolve(h, ctx, outer)

	// The outer walk is done and the chain descends into the inner optional
	// Investigate. It must pose its OWN election to seat 1: without the cursor
	// reset it resumes at index 1 (the outer walk's end) and asks nothing, so
	// len(asks) stays at 1.
	if len(h.asks) != 2 {
		t.Fatalf("after the outer walk the chained optional Investigate posed %d asks total, want 2 (the inner walk's own election; a stale cursor would pose nothing)", len(h.asks))
	}
	if h.asks[1].Kind != decision.KChoose || h.asks[1].Player != 1 {
		t.Fatalf("inner ask = %+v, want a KChoose to the opponent seat 1 (the chained walk's own election)", h.asks[1])
	}
	ctx.InvestigateOpt = "yes"
	ctx.InvestigateOptIdx = 0
	h.suspended = false
	Resolve(h, ctx, outer)

	// Two accepted elections mint two Clues on seat 1 and emit two markers.
	if got := clueCountOn(h, 1); got != 2 {
		t.Errorf("seat 1 Clue tokens = %d, want 2 (one per accepted election)", got)
	}
	if got := investigateMarkers(h); got != 2 {
		t.Errorf("events.Investigate markers = %d, want 2 (one per accepted election)", got)
	}
}

// clueCountOn counts Clue tokens in seat p's control by their token face
// name, the identity effInvestigate's TokenCreate mints.
func clueCountOn(h *chainInvestigateHost, p state.PlayerID) int {
	n := 0
	for _, id := range h.g.Zone(state.ZBattlefield, p) {
		o := h.g.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == "Clue Token" {
			n++
		}
	}
	return n
}

// investigateMarkers counts events.Investigate records for any seat.
func investigateMarkers(h *chainInvestigateHost) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.Investigate {
			n++
		}
	}
	return n
}
