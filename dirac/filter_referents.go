package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugReferentOwnerSwap is the mutation test's switch:
// the off-battlefield owner role reads the OTHER seat. Test-only and cold.
var DebugReferentOwnerSwap bool

// cardRoleF2 is the card matcher's reading of F2TgtPlayerCtrl /
// F2DefendingCtrl (gorge controlReferentPlayers op "OwnedBy"): off the
// battlefield a card has no controller, so the role is the card's OWNER
// (OwnedBy TriggeredDefendingPlayer, OwnedBy Targeted). On the battlefield
// the same bits keep meaning "controlled by" (ctrlRoleF2 in the perm matcher).
//
//go:noinline
func (s *State) cardRoleF2(k *prep, f2 kop.FilterFlags2, owner uint8) bool {
	if DebugReferentOwnerSwap {
		owner ^= 1 // seat-ok: a test-only mutation (DebugReferentOwnerSwap)
	}
	return s.ctrlRoleF2(k, f2, owner)
}
