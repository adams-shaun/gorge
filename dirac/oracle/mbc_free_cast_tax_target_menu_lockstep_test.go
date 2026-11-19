package oracle

import "testing"

// CR 601.2c with CR 601.2f, 118.9d: Aether Tradewinds cast free off Urza, Lord
// High Artificer's exile-and-play ability still owes Kasmina, Enigmatic Mentor's
// {2} tax once its second target is a creature or planeswalker, and the caster's
// one untapped Swamp cannot pay it. Urza's "tap an untapped artifact: add {U}"
// has no artifact to tap, which leaves gorge's mana-window census incomplete, so
// gorge's target menu keeps every candidate (affordableTargetCandidates); the
// kernel's free-cast arm priced the candidates on the plain sources alone and
// dropped Guttersnipe, Kasmina, Ophidian and the Wizard token (decision 515 of
// 515; dr-mbc-baaa pair 133).
func TestMBCGuttersnipeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_cast_tax_target_menu_census_gap")
}
