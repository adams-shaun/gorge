package oracle

import "testing"

// CR 118.9d: Urza, Lord High Artificer's "{5}: shuffle, then exile the top
// card; until end of turn you may play it without paying its mana cost"
// frees only the mana cost. Thorn of Amethyst's "noncreature spells cost {1}
// more" still applies, and the caster may tap an untapped source for it. The
// recording pins Gelatinous Genesis offered at that {1}, the tax paid by the
// untapped Island left after the turn's land play.
func TestMBCGelatinousGenesisReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_mayplay_tax_paid_by_source")
}
