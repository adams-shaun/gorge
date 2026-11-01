package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "whenever you tap an untapped creature an opponent
// controls" (Mode$ Taps | ValidCard$ Creature.OppCtrl | ValidPlayer$ You;
// Hylda of the Icy Crown, Icewrought Sentry, Sharae of Numbing Depths,
// Solitary Sanctuary). gorge tapsMatches (trigmatch/tap.go) reads
// ValidPlayer$ against the TAPPER (TapActor), not the permanent's
// controller: a Tap / TapAll / TapOrUntap effect taps as the resolving
// ability's controller (combatfx.go effTap, effTapAll; taporuntap.go);
// attack, cost, convoke, sneak and regeneration taps tap as the permanent's
// own controller (CR 701.26a, CR 603.2e).

// DebugTapsValidAnyTapper is a mutation hook: tapsWhoseOK ignores the
// tapper (an opponent's attack taps fire "whenever you tap" lines).
var DebugTapsValidAnyTapper bool

// tapsWhoseOK: the tapper (fireTapsBy's amt) satisfies listener l's Taps
// ValidPlayer$ (Trig.Whose, relative to the listener's controller).
//
//go:noinline
func (s *State) tapsWhoseOK(l trigListener, a *kop.Ability, amt int16) bool {
	if DebugTapsValidAnyTapper {
		return true
	}
	return whoseOK(a.Trig.Whose, l.ctrl, uint8(amt))
}

// effTapper is the player tapping the permanent at sl from an effect: the
// resolving spell's or ability's controller, else the permanent's
// controller (the CounterAddedOnce putter precedent, triggers_gen.go).
//
//go:noinline
func (s *State) effTapper(sl Slot) uint8 {
	if s.res.active {
		return s.res.e.ctrl
	}
	return s.perms[sl].ctrl
}
