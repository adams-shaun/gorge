package events

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestExtraPhaseRidersRoundTrip pins the Text-encoded rider payload (the
// ExtraPhaseDelayedTrigger$ forwarding: the DELAY= step ordinal and the
// VP= ValidPlayer$ value) on the canonical encoding order and on the
// decode's totality: hostile text decodes to no riders, never a step.
func TestExtraPhaseRidersRoundTrip(t *testing.T) {
	enc := EncodeExtraPhaseRiders(ExtraPhaseRiders{
		HasDelayedPhase: true, DelayedPhase: state.StepBeginCombat, ValidPlayer: "You"})
	if enc != "DELAY=4|VP=You" {
		t.Fatalf("encoded riders = %q, want \"DELAY=4|VP=You\"", enc)
	}
	dec := DecodeExtraPhaseRiders(enc)
	if !dec.HasDelayedPhase || dec.DelayedPhase != state.StepBeginCombat || dec.ValidPlayer != "You" {
		t.Fatalf("decoded riders = %+v", dec)
	}
	// VP only (a rider whose DelTrig named no phase but has a ValidPlayer$).
	dec = DecodeExtraPhaseRiders(EncodeExtraPhaseRiders(ExtraPhaseRiders{ValidPlayer: "Opponent"}))
	if dec.HasDelayedPhase || dec.ValidPlayer != "Opponent" {
		t.Fatalf("VP-only riders decoded = %+v", dec)
	}
	// No riders at all: the empty marker both ways.
	if EncodeExtraPhaseRiders(ExtraPhaseRiders{}) != "" {
		t.Fatal("empty riders encoded non-empty")
	}
	if dec := DecodeExtraPhaseRiders("not riders"); dec.HasDelayedPhase || dec.ValidPlayer != "" {
		t.Fatalf("hostile text decoded = %+v, want no riders", dec)
	}
	// An out-of-range DELAY ordinal is dropped, never folded as a step.
	if dec := DecodeExtraPhaseRiders("DELAY=99"); dec.HasDelayedPhase {
		t.Fatalf("out-of-range delay decoded = %+v", dec)
	}
}

// TestExtraPhaseFoldGrantConsumeComplete pins the three-message fold: +1
// appends one queue entry per granted phase with the splice/entry/resume
// fields (RangeEnd derived from the entry step), -1 marks the FIRST
// identity-matching entry consumed (and registers nothing without the
// delayed rider), -2 removes the first consumed identity match, and
// TurnChange clears whatever is left. The identity of a consume that
// matches nothing is a no-op.
func TestExtraPhaseFoldGrantConsumeComplete(t *testing.T) {
	g, _ := twoPlayer(t)
	grant := Event{Kind: ExtraPhase, Player: 0, Obj: 7, Amount: 1,
		Step: state.StepMain1, IDs: []state.ObjID{state.ObjID(state.StepBeginCombat)},
		Text: EncodeExtraPhaseRiders(ExtraPhaseRiders{HasDelayedPhase: true,
			DelayedPhase: state.StepBeginCombat, ValidPlayer: "You"})}
	Apply(g, grant)
	if len(g.ExtraPhases) != 1 {
		t.Fatalf("after grant: %+v", g.ExtraPhases)
	}
	ep := g.ExtraPhases[0]
	if ep.AfterStep != state.StepMain1 || ep.Entry != state.StepBeginCombat ||
		ep.RangeEnd != state.StepEndCombat || ep.Consumed || ep.Source != 7 ||
		!ep.HasDelayedPhase || ep.DelayedPhase != state.StepBeginCombat || ep.ValidPlayer != "You" {
		t.Fatalf("folded entry = %+v", ep)
	}
	// A NumPhases-style multi-grant appends one entry per granted phase.
	Apply(g, Event{Kind: ExtraPhase, Player: 0, Obj: 7, Amount: 2,
		Step: state.StepMain1, IDs: []state.ObjID{state.ObjID(state.StepUpkeep)}})
	if len(g.ExtraPhases) != 3 {
		t.Fatalf("after the x2 grant: %d entries", len(g.ExtraPhases))
	}
	// Consume marks the FIRST identity match; the second identical entry
	// stays pending.
	consume := Event{Kind: ExtraPhase, Player: 0, Obj: 7, Amount: -1,
		Step: state.StepMain1, IDs: []state.ObjID{state.ObjID(state.StepBeginCombat)},
		Text: EncodeExtraPhaseRiders(ExtraPhaseRiders{HasDelayedPhase: true,
			DelayedPhase: state.StepBeginCombat, ValidPlayer: "You"})}
	Apply(g, consume)
	if !g.ExtraPhases[0].Consumed || g.ExtraPhases[1].Consumed || g.ExtraPhases[2].Consumed {
		t.Fatalf("consume marked the wrong entries: %+v", g.ExtraPhases)
	}
	// A consume with no Execute$ name and a rider-less source registers no
	// delayed trigger; with the rider and a resolvable face SVar it does.
	if len(g.Delayed) != 0 {
		t.Fatalf("a rider-less consume registered %+v", g.Delayed)
	}
	// The remaining two entries are the Upkeep grants: consuming THEIR
	// identity marks the first of them, not the already-consumed combat one.
	upkeepConsume := Event{Kind: ExtraPhase, Player: 0, Obj: 7, Amount: -1,
		Step: state.StepMain1, IDs: []state.ObjID{state.ObjID(state.StepUpkeep)}}
	Apply(g, upkeepConsume)
	if !g.ExtraPhases[0].Consumed || !g.ExtraPhases[1].Consumed || g.ExtraPhases[2].Consumed {
		t.Fatalf("second consume: %+v", g.ExtraPhases)
	}
	// Complete removes the first CONSUMED identity match.
	Apply(g, Event{Kind: ExtraPhase, Player: 0, Obj: 7, Amount: -2,
		Step: state.StepMain1, IDs: []state.ObjID{state.ObjID(state.StepBeginCombat)}})
	if len(g.ExtraPhases) != 2 || g.ExtraPhases[0].Entry != state.StepUpkeep || g.ExtraPhases[1].Consumed {
		t.Fatalf("after complete: %+v", g.ExtraPhases)
	}
	// A stale message (nothing left matching) is a no-op.
	Apply(g, Event{Kind: ExtraPhase, Player: 0, Obj: 7, Amount: -2,
		Step: state.StepMain1, IDs: []state.ObjID{state.ObjID(state.StepBeginCombat)}})
	if len(g.ExtraPhases) != 2 {
		t.Fatalf("stale complete removed an entry: %+v", g.ExtraPhases)
	}
	// TurnChange clears the queue wholesale: an extra phase never survives
	// into the next turn.
	Apply(g, Event{Kind: TurnChange, Player: 1, Amount: 2})
	if len(g.ExtraPhases) != 0 {
		t.Fatalf("the queue survived the turn boundary: %+v", g.ExtraPhases)
	}
	if g.CombatsThisTurn != 0 {
		t.Fatalf("the combat count survived the turn boundary: %d", g.CombatsThisTurn)
	}
}
