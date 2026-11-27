package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The Player.Active / Opponent.Active ValidPlayer$ qualifiers on a
// player-event trigger (Valgavoth, Harrower of Souls' LifeLost line). gorge
// playerSpecQualifierActive is "the matched player is the active player"
// (engine/effects/player_filter.go); the kernel reads the value as the event
// player against State.active (dirac/triggers_gen.go playerTrigGenObj).
// build_ab.go casts the value to kop.Whose, so the constants ARE the kop
// values; there is no separate number to drift.

const (
	whoActive    uint8 = uint8(kop.WhoseActive)
	whoOppActive uint8 = uint8(kop.WhoseOppActive)
)

// activePlayerWho maps the two Active spellings of ValidPlayer$. It reports
// false for every other value, which keeps its caller's whoOfPlayer path.
func activePlayerWho(v string) (uint8, bool) {
	switch v {
	case "Player.Active":
		return whoActive, true
	case "Opponent.Active":
		return whoOppActive, true
	}
	return 0, false
}
