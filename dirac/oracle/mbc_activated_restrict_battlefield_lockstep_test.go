package oracle

import "testing"

// CR 106.6: A-Base Camp's any-colour mana is spent "only to activate an
// ability of a Wizard"; Forge's script (and gorge's restrictValidTermMatches)
// writes the term Activated.Wizard+inZoneBattlefield, so the ability's source
// must be on the battlefield. Fatestitcher's Unearth {U} is activated from
// the graveyard, so the Camp's mana does not pay it and gorge withholds the
// activation. The kernel dropped the zone word (kgen's filter table reads
// inZoneBattlefield as implied) and admitted any Wizard card in any zone, so
// it offered an Unearth gorge did not (decision 280, tail-73-c000 pair 24).
func TestMBCFatestitcherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_restriction_activated_wizard_needs_battlefield")
}
