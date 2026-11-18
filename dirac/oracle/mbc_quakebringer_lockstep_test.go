package oracle

import "testing"

// CR 605.1a, 601.2g: Brightcap Badger gives each Saproling its controller has
// "{T}: Add {G}", so two untapped Saprolings beside two Mountains and a Gold Myr
// pay a five-mana Quakebringer or Tephraderm. The recorded game offers both
// at decision 338 (tail-65-c000, seed 305826).
func TestMBCQuakebringerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_saproling_mana_pays_five_drop")
}
