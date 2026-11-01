package wire

import (
	"encoding/json"
	"fmt"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
)

// ViewOmitNames is the fixed, ordered name table of the handshake's
// view_omit field: the view.Omit bits a consumer may declare it never
// reads. OmitDecision is NOT a name — the decision is the one thing a
// decide item exists to carry, so it can never be omitted.
var ViewOmitNames = []struct {
	Name string
	Bit  view.Omit
}{
	{"library", view.OmitLibrary},
	{"available", view.OmitAvailable},
	{"archetype", view.OmitArchetype},
	{"ability_costs", view.OmitAbilityCosts},
	{"effective_cost", view.OmitEffectiveCost},
	{"own_deck", view.OmitOwnDeck},
	{"potential", view.OmitPotential},
	{"card_lists", view.OmitCardLists},
	{"derived_chars", view.OmitDerivedChars},
}

// ParseViewOmit ORs the bits of the named omissions. An unknown name —
// including "decision" — is an error. A nil or empty list omits nothing.
func ParseViewOmit(names []string) (view.Omit, error) {
	var omit view.Omit
	for _, n := range names {
		known := false
		for _, e := range ViewOmitNames {
			if e.Name == n {
				omit |= e.Bit
				known = true
				break
			}
		}
		if !known {
			return 0, fmt.Errorf("%w: field %q: unknown view_omit name %q", ErrProtocol, "view_omit", n)
		}
	}
	return omit, nil
}

// DecisionOf decodes just view.decision out of a decide item's raw view,
// for agents that never need the rest of the projection. A view without a
// decision is an error: the decision is what a decide item asks to answer.
func DecisionOf(raw json.RawMessage) (*decision.Decision, error) {
	var v struct {
		Decision *decision.Decision `json:"decision"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("wire: bad decide view: %v", err)
	}
	if v.Decision == nil {
		return nil, fmt.Errorf("wire: bad decide view: no decision")
	}
	return v.Decision, nil
}

// DecodeView decodes a decide item's whole view.View.
func DecodeView(raw json.RawMessage) (view.View, error) {
	var v view.View
	if err := json.Unmarshal(raw, &v); err != nil {
		return view.View{}, fmt.Errorf("wire: bad decide view: %v", err)
	}
	return v, nil
}
