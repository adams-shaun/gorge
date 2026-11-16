package oracle

import "testing"

// TestReproKCISacrificeAnyBoundAndGrantLockstep replays two recorded
// Krark-Clan Ironworks games (dr-mbc-bbbb pairs 87 and 50): a closed potential
// bound that counts Fellwar Stone's reflected mana once per colour (gorge lists
// the {4} cast the gaps walk's three mana fell short of), and a granted
// any-colour mana ability (Abundant Growth) beside an Ironworks and a depletion
// land (gorge's one prefix search sacrifices the Ironworks, which the grant
// search skipped, tapping the depletion land instead). They play the "custom"
// table their own decks need.
func TestReproKCISacrificeAnyBoundAndGrantLockstep(t *testing.T) {
	for _, name := range []string{
		"sacrifice_mana_potential_bound_counts_each_colour",
		"sacrifice_mana_beside_granted_mana_sacrifices_itself",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
