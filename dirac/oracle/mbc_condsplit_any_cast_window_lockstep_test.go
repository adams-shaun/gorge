package oracle

import (
	"testing"
)

// TestMBCRakdosCharmReprosLockstep replays the recorded dr-mbc-abbb game
// (decks focus_00259_WBR vs focus_00300_WBR, seed 4326465700074898313): p0
// casts Walking Ballista ({X}{X}) on turn 22 beside a lucky Gemstone Caverns
// (a luck counter on it), whose Condition-split sub adds one mana of any
// color. gorge's cast-window probe skipped that sub -- its root's {C} is the
// plain literal the shared census was assumed to have counted, but the
// census's condition-split arm had swapped in the sub and dropped the whole
// unit -- so the CR 601.2b X announcement priced the CR 601.2g window at
// seven mana and offered X to 3; the kernel, whose lucky Gemstone asks one
// colour of WUBRG (dirac/mana_condsplit.go), offered X to 4 over eight
// mana-capable sources (CR 601.2g, 605.3b). gorge was wrong: the probe now
// reads the live sub's production. (The evidence loop's own example for this
// `options options` class named Rakdos Charm; its pair is unreplayable today
// because its enabler Starfleet Academy is refused, so this recorded game
// carries the class.)
func TestMBCRakdosCharmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "condsplit_any_cast_window")
}
