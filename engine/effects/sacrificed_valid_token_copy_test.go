package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A sacrificed token copy sits in the graveyard as an IsCopy object, which the
// copy-ceased rule (CR 707.10h) rejects from every type match; the
// Sacrificed$Valid count reads the capture instead (CR 608.2h), so a changeling
// copy (Dominion Infiltration's) is still a Creature for Gowron's "sacrificed a
// Klingon", while the printed face does not leak through the capture.
func TestSacrificedValidCountsACeasedTokenCopy(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:Bear Copy\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	token := h.g.AddObject(card, 0)
	token.IsCopy = true
	token.Zone = state.ZGraveyard
	if !token.IsCopy || token.Zone != state.ZGraveyard {
		t.Fatalf("precondition: want a copy in the graveyard, got IsCopy=%v zone=%v", token.IsCopy, token.Zone)
	}
	src := h.g.AddObject(card, 0)
	changeling := &Ctx{Source: src.ID, Controller: 0,
		Sacrificed: []state.SacrificedInfo{{Obj: token.ID, LayerTypes: true, Types: []string{"Creature"}, AllCreatureTypes: true}}}
	if got := EvalCount(h, changeling, "Sacrificed$Valid Creature"); got != 1 {
		t.Errorf("changeling token copy: Sacrificed$Valid Creature = %d, want 1", got)
	}
	if got := EvalCount(h, changeling, "Sacrificed$Valid Human"); got != 1 {
		t.Errorf("changeling token copy: Sacrificed$Valid Human = %d, want 1", got)
	}
	bear := &Ctx{Source: src.ID, Controller: 0,
		Sacrificed: []state.SacrificedInfo{{Obj: token.ID, LayerTypes: true, Types: []string{"Creature", "Bear"}}}}
	if got := EvalCount(h, bear, "Sacrificed$Valid Human"); got != 0 {
		t.Errorf("plain Bear token copy: Sacrificed$Valid Human = %d, want 0", got)
	}
}
