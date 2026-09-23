package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestPlayerSpecFx20Grammar pins the fx20 player-spec qualifiers this build
// now resolves: Player.EnchantedController, Player.TriggeredDefendingPlayer
// and the Player.counters_<CMP><n>_<KIND> comparison, each read from the
// event-backed state the rule needs. The two the row still leaves
// fail-closed -- Player.EnchantedBy (the engine has no player attachment)
// and Player.descended (no descend state) -- are asserted to stay closed.
func TestPlayerSpecFx20Grammar(t *testing.T) {
	g := state.NewGame([]string{"a", "b", "c"})
	bearer := qualifierObject(t, g, 1, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	aura := qualifierObject(t, g, 0, "Name:Aura\nTypes:Enchantment Aura\nOracle:x\n")
	aura.AttachedTo = bearer.ID
	// Precondition: the aura's controller (0) differs from the bearer's (1),
	// so a fallback to the source's own controller would be observably wrong.
	if aura.Controller == bearer.Controller {
		t.Fatal("precondition: aura and bearer share a controller")
	}
	pc := PlayerSpecCtx{Source: aura.ID, DefendingPlayer: state.Target{Player: 2, IsPlayer: true}}

	for _, tc := range []struct {
		spec string
		p    state.PlayerID
		want bool
	}{
		{"Player.EnchantedController", 0, false},
		{"Player.EnchantedController", 1, true},
		{"Player.EnchantedController", 2, false},
		{"Player.TriggeredDefendingPlayer", 1, false},
		{"Player.TriggeredDefendingPlayer", 2, true},
		{"Player.TriggeredDefendingPlayer", 0, false},
	} {
		if got := MatchesPlayerSpecCtx(g, tc.spec, tc.p, 0, pc); got != tc.want {
			t.Errorf("MatchesPlayerSpecCtx(%q, seat %d) = %v, want %v", tc.spec, tc.p, got, tc.want)
		}
	}

	// Player.counters_<CMP><n>_<KIND>. Seat 2 has no contract counter; add
	// one and the EQ0 spelling flips, proving the read is live state.
	if !MatchesPlayerSpecCtx(g, "Player.counters_EQ0_Contract", 2, 0, pc) {
		t.Fatal("seat 2 with no contract counter must satisfy Player.counters_EQ0_Contract")
	}
	g.Players[2].AddCounter("Contract", 1)
	if MatchesPlayerSpecCtx(g, "Player.counters_EQ0_Contract", 2, 0, pc) {
		t.Fatal("seat 2 with one contract counter must fail Player.counters_EQ0_Contract")
	}
	if !MatchesPlayerSpecCtx(g, "Player.counters_GE1_Contract", 2, 0, pc) {
		t.Fatal("Player.counters_GE1_Contract must match seat 2")
	}

	// The remainders stay fail-closed: no player attachment (EnchantedBy)
	// and no descend state (descended) in this build.
	for _, spec := range []string{"Player.EnchantedBy", "Player.descended", "Player.Descended"} {
		for p := state.PlayerID(0); p < 3; p++ {
			if MatchesPlayerSpecCtx(g, spec, p, 0, pc) {
				t.Errorf("%s matched seat %d; it must stay fail-closed", spec, p)
			}
		}
	}

	// No source: EnchantedController fails closed. No trigger role bound:
	// TriggeredDefendingPlayer fails closed.
	if MatchesPlayerSpecCtx(g, "Player.EnchantedController", 1, 0, PlayerSpecCtx{}) {
		t.Fatal("Player.EnchantedController with no source must fail closed")
	}
	if MatchesPlayerSpecCtx(g, "Player.TriggeredDefendingPlayer", 2, 0, PlayerSpecCtx{Source: aura.ID}) {
		t.Fatal("Player.TriggeredDefendingPlayer with no defending player must fail closed")
	}
}
