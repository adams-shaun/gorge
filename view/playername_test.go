package view

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestPlayerViewNameProjectsThePlayerName probes the B3 name/deck split at
// the view layer: view.displayName prefers state.Player.PlayerName (the
// independent per-seat player name that host.TableConfig.PlayerNames feeds
// in) and falls back to state.Player.Name (the deck stem) when no player
// name was configured. The fallback is the load-bearing invariant that keeps
// pre-B3 sidecars replaying unchanged -- a host that never sets PlayerNames
// must project the deck stem exactly as it always did.
func TestPlayerViewNameProjectsThePlayerName(t *testing.T) {
	// A player whose PlayerName is set projects it, not the deck stem.
	g := state.NewGame([]string{"some-deck-stem"})
	g.Players[0].PlayerName = "Alice"
	v := Project(g, flatChars{g}, 0, nil)
	if got := v.Players[0].Name; got != "Alice" {
		t.Fatalf("configured PlayerName: projected %q, want %q", got, "Alice")
	}

	// A player with no PlayerName (the pre-B3 shape) falls back to the deck
	// stem, unchanged.
	g2 := state.NewGame([]string{"some-deck-stem"})
	v2 := Project(g2, flatChars{g2}, 0, nil)
	if got := v2.Players[0].Name; got != "some-deck-stem" {
		t.Fatalf("empty PlayerName fallback: projected %q, want %q", got, "some-deck-stem")
	}
}
