package oracle

import (
	"testing"
)

// TestMBCOutnumberReprosLockstep replays the recorded dr-mbc-abbb game
// (pool deal focus-ou_00364_BR vs focus-ou_00305_WBR, seed 6690448154065553270,
// pair 150 of the Outnumber-focused deal, seed 921789): p0 casts Mana Flare
// through a lucky Gemstone Caverns and a Swamp, and at the next priority its
// {3}{R} Fathom Fleet Swordjack is a cast only the reflected extra funds.
// gorge's planner prices the tap's reflected extra of the pick (CR 106.12a:
// Mana Flare adds one mana of any type that land produced) and offers the
// cast; the kernel's deferred-gap float (tapDeferFloat) read the extra
// through addExtra, which cannot know the pick and silently floated nothing,
// so the kernel priced three mana and withheld the cast.
//
// The evidence loop's own Outnumber game (dr-mbc-abbb-s000 pair 36) is
// unreplayable here: its focus names Starfleet Academy, refused by name
// after that land (735bcb96f2), and Outnumber itself is evidenced clean in
// the pool's ledger (13 plays, no missing branches). This recording pins the
// same class the loop's game held Outnumber back on: the pool's
// "options options" part where the kernel withholds a cast gorge offers
// beside a StatTapExtra carrier.
func TestMBCOutnumberReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "deferred_gap_reflextra_float")
}
