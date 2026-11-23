package oracle

import "testing"

// TestMBCHedronAlignmentReprosLockstep replays the synthetic tail-64 game the
// evidence loop recorded: p1's only untapped mana is Urza, Lord High
// Artificer, whose mana ability is "Tap an untapped artifact you control: Add
// {U}" -- a repeatable tap-another cost that never taps Urza itself. Two
// activations (tapping Jeweled Lotus and the Construct token, battlefield
// order) float the {1}{U} Hedron Alignment's "Scry 1" activation needs, so the
// option is legal (CR 605.3a: mana abilities are activated while paying a
// cost; CR 302.6's summoning sickness covers the tap symbol only, and neither
// the enchantment source nor the tap-another cost carries one). gorge's
// potential fixpoint admitted the ability once -- one {U} -- so the offer gate
// priced {1}{U} unpayable and withheld the option (divergence at decision 279:
// options options, the kernel's menu held one extra option).
func TestMBCHedronAlignmentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "potential_tapx_repeat_bound")
}
