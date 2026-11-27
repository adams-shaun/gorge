package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The per-player sacrificed-this-turn tallies: player.sac
// indexes. They answer the three spellings kgen keeps as tallies
// (Permanent, Artifact, Food); a filtered spec reads this turn's zone-add
// ledger instead (sacrificedThisTurn, value_enteredledger.go).
const (
	sacAll      = 0 // every sacrifice
	sacArtifact = 1 // artifact cards (the printed type, as gorge reads the destination)
)

// DebugSacrificedNoTurnReset (TestSacrificedMutation): beginTurn keeps the
// sacrificed-this-turn tallies.
var DebugSacrificedNoTurnReset bool

// SacrificedCountStats counts the filtered sacrificed-this-turn reads
// (DebugSacrificedCountStats; TestDemoSacrificedCountLockstep reads it so a
// green game is one where the filter path fired; nothing in the engine reads
// it).
type SacrificedCountStats struct {
	Reads   int64 // evaluations of the filter path
	Matched int64 // matching sacrifices counted
	NonZero int64 // evaluations that returned a nonzero count
}

// DebugSacrificedCountStats, when set, receives the counts.
var DebugSacrificedCountStats *SacrificedCountStats

// DebugSacrificedReadOff is the mutation switch (TestDemoSacrificedCountMutation):
// the filtered path returns 0, as if no sacrifice matched.
var DebugSacrificedReadOff bool

// noteSac records the sacrifice of the permanent at sl BEFORE it leaves:
// the sacrificer is its controller now (CR 701.21a; the taker of a stolen
// permanent, not its owner). A token copy is skipped: gorge's
// matchesZoneSpecCtx rejects an IsCopy object off the stack/battlefield
// (CR 707.10h, engine/effects/filter.go:3298), so it matches no spec. The
// departure is flagged for removePerm, which records it on the zone entry.
//
//go:noinline
func (s *State) noteSac(k *prep, sl Slot) {
	p := &s.perms[sl]
	ob := &s.objs[p.obj]
	if ob.of&ofCopy != 0 {
		return
	}
	p.cseen |= cseenSacNext
	pl := &s.pl[SeatIx(p.ctrl)]
	if pl.sac[sacAll] < 255 {
		pl.sac[sacAll]++
	}
	if k.cards[ob.card].types&kop.TArtifact != 0 && pl.sac[sacArtifact] < 255 {
		pl.sac[sacArtifact]++
	}
	if k.cards[ob.card].food && pl.sac[sacFood] < 255 {
		pl.sac[sacFood]++ // (value_turntallies.go)
	}
}

// sacrificedThisTurn is ExSacrificedThisTurn (gorge sacrificedThisTurn).
// X.N 0/1/2 read the three tallies; a nonzero X.F is a filter read over every
// sacrifice the reading controller made this turn, matched against the card
// where it went (gorge matchesZoneSpecCtx; matchDeparture).
//
//go:noinline
func (s *State) sacrificedThisTurn(k *prep, x *kop.Expr, ctx evalCtx) int {
	if x.F != 0 {
		n := 0
		if !DebugSacrificedReadOff && s.g.enteredOn {
			for i := uint8(0); i < s.g.enteredN; i++ {
				e := &s.cold.entered[i]
				if e.from() != uint8(kop.ZoneBattlefield) || e.flags&enteredSacrificed == 0 {
					continue
				}
				if e.leftController() != SeatIx(ctx.ctrl) {
					continue
				}
				if !s.matchDeparture(k, x.F, e, ctx) {
					continue
				}
				n++
			}
		}
		if st := DebugSacrificedCountStats; st != nil {
			st.Reads++
			st.Matched += int64(n)
			if n > 0 {
				st.NonZero++
			}
		}
		return n
	}
	pl := &s.pl[SeatIx(ctx.ctrl)]
	if x.N == 1 {
		return int(pl.sac[sacArtifact])
	}
	if x.N == kop.SacrificedFood {
		return int(pl.sac[sacFood])
	}
	return int(pl.sac[sacAll])
}
