package agent

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestAutoPayKeepsManaWindowActivations: only a priority decision loses the
// manual mana activations an engine_autopay wire does not offer. A choose that
// is a mid-resolution mana window (echo, cumulative upkeep, a ward or unless
// cost) carries those activations on the wire, so the bot asked it must still
// be able to pick one.
func TestAutoPayKeepsManaWindowActivations(t *testing.T) {
	s := &Server{autoPay: true}
	window := decision.Decision{Kind: decision.KChoose, Options: []decision.Option{
		{Index: 0, Kind: "activate", Label: "Tap Forest for {G}"},
		{Index: 1, Kind: "done", Label: "Done"},
	}}
	if len(window.Options) != 2 || window.Options[0].Kind != "activate" {
		t.Fatalf("precondition: mana window %+v does not offer an activation", window.Options)
	}
	got := s.withoutManualMana(window)
	if len(got.Options) != 2 || got.Options[0].Kind != "activate" || got.Options[1].Kind != "done" {
		t.Fatalf("auto-pay stripped the mana window's activations: %+v", got.Options)
	}

	// The priority window is the one the wire hides manual taps from.
	priority := decision.Decision{Kind: decision.KPriority, Options: []decision.Option{
		{Index: 0, Kind: "activate", Label: "Tap Forest for {G}"},
		{Index: 1, Kind: "pass", Label: "Pass"},
	}}
	got = s.withoutManualMana(priority)
	if len(got.Options) != 1 || got.Options[0].Kind != "pass" {
		t.Fatalf("auto-pay kept a priority manual activation: %+v", got.Options)
	}
}
