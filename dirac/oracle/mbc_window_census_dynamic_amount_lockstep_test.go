package oracle

import "testing"

// CR 601.2c, 601.2g: Price of Fame costs {2} less if it targets a legendary creature. Its caster
// floats {R}{B} beside an untapped Cliffgate and an untapped Alena, Kessig Trapper, whose {T} adds
// red equal to the greatest power among its controller's creatures that entered this turn: none did,
// so the amount is zero. gorge's window census leaves that zero amount unpriced (castWindowAmount),
// so it is incomplete and the target menu keeps the opponent's non-legendary Woodland Mystic beside
// Alena (affordableTargetCandidates' census fallback); dirac once kept only Alena
// (dr-mbc-abba pair 164, decision 279 of 279).
func TestMBCWoodlandMysticReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "window_census_zero_dynamic_amount_keeps_target")
}
