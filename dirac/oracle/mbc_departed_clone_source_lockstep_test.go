package oracle

import "testing"

// CR 113.7a / 608.2h: Spellbane Centaur's "creatures you control can't be the
// targets of blue spells or abilities from blue sources" is judged against the
// departed source's last known information. A Quicksilver Gargantuan that
// entered as a Robot Chicken (colorless) dies with an Egg; the Egg trigger's
// "destroy target artifact or creature" was offered without the opponent's
// creatures because gorge read the graveyard card's printed blue (decision 763
// of tail-37-c000 pair 43), while the kernel read the copy's colorless.
func TestMBCOrochiMergeKeeperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "departed_clone_source_color_cant_target")
}
