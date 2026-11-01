package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Count$LastStateBattlefieldWithFallback: the number
// of battlefield permanents matching CardDef.CastBF as the spell was cast.
// beginCast freezes the count into the spell entry's trigMV the moment the
// spell reaches the stack (CR 601.2a: before targets, divisions and costs;
// gorge castBattlefieldEvent), and ExCastBattlefield reads it at resolution
// (value_exprs.go). A stack copy inherits the entry and with it the count
// (gorge state/cast_battlefield.go: same). A spell entry's trigMV is used
// for nothing else: only trigger entries write and read it
// (ExTriggerSpellMVLKI), and never the trigAmt whose spell-side role is the
// mana-spend record (recordSpent, copy_asenters.go).

// DebugCastBFLive makes ExCastBattlefield read the live battlefield at
// resolution instead of the frozen count: the mutation of
// TestCastbfMutation (a Mount or Faerie dying between cast and resolution
// must change the answer).
var DebugCastBFLive bool

// DebugCastBFStats, when set, tallies every ExCastBattlefield evaluation
// for TestCastbfLockstep's coverage log.
var (
	DebugCastBFStats bool
	castBFFrozenN    int // evaluations whose frozen count is >= 1
	castBFFrozen0    int // evaluations whose frozen count is 0
	castBFFrozenLive int // evaluations whose frozen and live counts differ
)

// castBFLiveCount counts the battlefield permanents matching f, the
// filter's You clauses reading ctrl, saturating at 255 (the trigMV byte).
// f = 0 counts nothing: the frozen count is written only for a card that
// reads the head.
func (s *State) castBFLiveCount(k *prep, ctrl uint8, src Slot, f kop.FilterID) uint8 {
	if f == 0 {
		return 0
	}
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		if s.matchPermGen(k, f, m.first(), ctrl, src, layerFull) {
			if n++; n >= 255 {
				return 255
			}
		}
	}
	return uint8(n)
}

// CastBFStatsReset zeroes the DebugCastBFStats tallies, and CastBFStats
// reports them (TestCastbfLockstep's coverage log: frozen counts >= 1,
// frozen counts of 0, and reads whose frozen and live counts differ).
func CastBFStatsReset() {
	castBFFrozenN, castBFFrozen0, castBFFrozenLive = 0, 0, 0
}

func CastBFStats() (frozenGE1, frozen0, differ int) {
	return castBFFrozenN, castBFFrozen0, castBFFrozenLive
}
