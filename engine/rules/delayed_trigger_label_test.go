package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDelayedTriggerLabelNamesItsTriggerDescription pins the label a client
// sees for a DB$ DelayedTrigger pending trigger (xproof xpt-3858df5b).
// Arcane Denial's two slowtrip delayed triggers ("Draw a card." for its
// caster, "Draw up to two cards." for the countered spell's controller) fire
// together at the beginning of the next turn's upkeep, both controlled by the
// countering player (CR 603.7d), so the CR 603.3b order ask offers both. Each
// option must be labelled "<source>: <its TriggerDescription$>" -- exactly
// like a face trigger's -- never the bare source name twice, which no human
// client or oracle driver can bind to a trigger.
func TestDelayedTriggerLabelNamesItsTriggerDescription(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ad := mustCorpusCard(t, reg, "Arcane Denial")
	e, cfg := tokenReplGame(t, 413, ad)
	id := moveSeededCard(t, e, 0, ad, state.ZHand)
	// Resolve the card's first DelayedTrigger SA the way the stack would:
	// its SubAbility$ chain registers both slowtrip promises (NextTurn$ True,
	// so the first one fires at the NEXT turn's upkeep).
	resolveDelayedTriggerAbility(t, e, id, 0, "DelTrigSlowtrip")
	if len(e.G.Delayed) != 2 {
		t.Fatalf("registrations = %d, want 2 (DelTrigSlowtrip chains DelTrigDrawTwo)", len(e.G.Delayed))
	}
	for _, dt := range e.G.Delayed {
		if dt.Description == "" {
			t.Fatalf("registration %+v folded no TriggerDescription$", dt)
		}
	}
	driveToStepAll(t, e, 2, 1, state.StepUpkeep)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder || len(d.Options) != 2 || d.Player != 0 {
		t.Fatalf("pending = %+v, want a two-option KTriggerOrder ask for seat 0", d)
	}
	var got []string
	for _, o := range d.Options {
		got = append(got, o.Label)
	}
	want := map[string]bool{
		"Arcane Denial: Draw a card.":          true,
		"Arcane Denial: Draw up to two cards.": true,
	}
	if len(got) != 2 || !want[got[0]] || !want[got[1]] || got[0] == got[1] {
		t.Fatalf("order-ask labels = %q, want the two distinct TriggerDescription$ labels %v", got, want)
	}
	replayCheck(t, e, cfg)
}
