package oracle

import "testing"

// CR 109.2: a bare card-type word in a spell or ability's description, with no
// zone and no "card"/"spell"/"source"/"scheme", means a permanent of that type
// on the battlefield. Vedalken Engineer's "Spend this mana only to cast
// artifact spells or activate abilities of artifacts" therefore pays no
// ability of an artifact card off the battlefield: Mirrorshell Crab's channel
// from hand is an ability of an artifact card, not of an artifact. The WotC
// rulings say the same (Power Depot: the mana "can't be spent to activate
// abilities of artifact cards in other zones, such as cycling or unearth
// costs"; Dalakos, Crafter of Wonders: not "artifact cards that aren't on the
// battlefield").
//
// The forge script spells the zone out as Activated.Artifact+inZoneBattlefield;
// gorge restrictValidTermMatches' needsBattlefield reads it as that zone test
// and the kernel's restrActMatch skips a kop.ZoneNone alternative for a source
// that is no permanent. The recording is the tail-17 game the old seat commit
// 3dcc7a3bf2 read the other way: with the qualifier stripped, gorge offered
// activate(#62) at decision 332 and the kernel offered only pass. On the
// decided reading both withhold it, so the whole recorded prefix replays and
// the game reaches that priority decision in lockstep.
func TestMBCMirrorshellCrabReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "restricted_mana_pays_a_hand_activation")
}
