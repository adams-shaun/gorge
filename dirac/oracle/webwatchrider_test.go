package oracle

import "testing"

// A permission whose rider lets any mana pay (a card exiled and playable with
// mana of any type) is priced by the planner once Interdimensional Web Watch's
// restricted mana floats: that mana pays the cast from exile (CR 106.6, 601.2g)
// beside an untapped Swamp, so Venomized Cat is offered after the script taps
// the Web Watch. The repro is std-spm pair 1482.

func TestWebWatchRiderCastPricedByRestrictedPool(t *testing.T) {
	stdReproLockstep(t, "testdata/web_watch_exile_rider_cast.repro.json", "std-spm")
}
