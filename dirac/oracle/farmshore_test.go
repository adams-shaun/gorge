package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestPinnedFarmShoreLurker plays standard-ECL games that reach Shore Lurker
// (3/3 flying; "when this creature enters, surveil 1", CR 603.2, 701.25) in
// lockstep. It is the regression the farm's ECL-s116 game xe:RAND:2:3 found:
// at turn 15 a dig put several permanents onto the battlefield and dirac
// queued an entrant's enters trigger before the take was answered, while
// gorge holds every entrant's enters check for the batch (CR 603.6a). The
// games under testdata/pins are clean on the fixed kernel; the subtest plays
// the recorded repro of that mechanism, so removing the hold fails here too.
func TestPinnedFarmShoreLurker(t *testing.T) {
	pinnedGames(t, "std-ecl", Config{}, onBattlefield("Shore Lurker"))
	t.Run("dig_holds_entrants_until_the_take", func(t *testing.T) {
		reg := synthReg(t)
		set := cardsettest.Synth(t, "std-ecl")
		replayRepro(t, reg, set, "dig_entry_chosen_type_sees_batch", true, false)
	})
}
