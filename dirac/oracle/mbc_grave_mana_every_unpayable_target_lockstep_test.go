package oracle

import "testing"

// CR 601.2c, 601.2g, 605.1a: Banish from Edoras costs {2} less when it targets
// a tapped creature, so its target menu keeps a candidate only when the repriced
// cost is payable from the pool plus the CR 601.2g window. With {W}{W}{G}
// floating and every mana-ability source p0 controls tapped, the window census
// is incomplete while a graveyard mana source can be activated (a Jack-o'-Lantern
// sits in p0's graveyard, a second one on the battlefield): gorge's menu keeps
// all four creatures, the untapped Rampaging Monument and Skyshroud Troll among
// them (each reprices the cast to {4}{W}); dirac's census ignored the graveyard
// and withheld both (decision 669, pair 3 of the evidence loop's tail-32-c000,
// seed 17338).
func TestMBCRampagingMonumentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grave_mana_source_keeps_every_unpayable_target")
}
