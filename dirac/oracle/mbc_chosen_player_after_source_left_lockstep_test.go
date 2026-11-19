package oracle

import (
	"testing"
)

// CR 608.2h with CR 113.7a: The Rack's "at the beginning of the chosen
// player's upkeep" trigger was put on the stack while The Rack was on the
// battlefield; Touch the Spirit Realm's channel then exiled The Rack before
// the trigger resolved. The trigger still resolves and deals its damage to
// the chosen player read from the source's last known information -- the
// as-enters choice survives the departure. The kernel once read the chosen
// player only while the source was still that live battlefield permanent
// (repl_chooseplayer.go chosenPlayerSeat), so it resolved the trigger dealing
// no damage (dr-mbc-bbbb pair 142, decision 115: ref 17, kernel 20).
func TestMBCTimeReaperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "chosen_player_choice_after_source_left")
}
