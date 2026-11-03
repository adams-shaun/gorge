package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// evidenceActivationOffers counts the activations of id that legalActions
// offers seat 0.
func evidenceActivationOffers(e *Engine, id state.ObjID) int {
	n := 0
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == id {
			n++
		}
	}
	return n
}

// An activated ability costing CollectEvidence<N> is offered only while the
// activator's graveyard holds cards with total mana value N or more (CR
// 701.59a, 602.2b: a cost that cannot be paid cannot be activated; Forge's
// CostCollectEvidence.canPay reads the same total). The offer used to read no
// Evidence part, so a short graveyard was offered the activation and the
// evidence stage aborted it, once for every priority the seat passed through.
func TestCollectEvidenceActivationNeedsTheGraveyardTotal(t *testing.T) {
	e, _, strider := newFixtureDeck(t, 30, corpusCardText(t, "k/kyloxs_voltstrider.txt"))
	e.emit(events.Event{Kind: events.MoveZone, Obj: strider, From: state.ZHand, To: state.ZBattlefield})
	toMain1(t, e)
	if got := evidenceActivationOffers(e, strider); got != 0 {
		t.Fatalf("empty graveyard: %d evidence activations offered, want 0", got)
	}
	inZoneCard(t, e, 0, state.ZGraveyard, "Name:Four drop\nManaCost:3 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if got := evidenceActivationOffers(e, strider); got != 0 {
		t.Fatalf("graveyard of mana value 4 against collect evidence 6: %d offered, want 0", got)
	}
	inZoneCard(t, e, 0, state.ZGraveyard, "Name:One drop\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	if got := evidenceActivationOffers(e, strider); got != 0 {
		t.Fatalf("graveyard of mana value 5: %d offered, want 0", got)
	}
	inZoneCard(t, e, 0, state.ZGraveyard, "Name:Another one drop\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	if got := evidenceActivationOffers(e, strider); got != 1 {
		t.Fatalf("graveyard of mana value 6: %d offered, want 1", got)
	}
}
