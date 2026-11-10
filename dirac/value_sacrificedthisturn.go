package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The per-player sacrificed-this-turn tallies: player.sac
// indexes.
const (
	sacAll      = 0 // every sacrifice
	sacArtifact = 1 // artifact cards (the printed type, as gorge reads the destination)
)

// DebugSacrificedNoTurnReset (TestSacrificedMutation): beginTurn keeps the
// sacrificed-this-turn tallies.
var DebugSacrificedNoTurnReset bool

// noteSac records the sacrifice of the permanent at sl BEFORE it leaves:
// the sacrificer is its controller now (CR 701.21a; the taker of a stolen
// permanent, not its owner). A token copy is skipped: gorge's
// matchesZoneSpecCtx rejects an IsCopy object off the stack/battlefield
// (CR 707.10h, engine/effects/filter.go:3298), so it matches no spec.
//
//go:noinline
func (s *State) noteSac(k *prep, sl Slot) {
	p := &s.perms[sl]
	ob := &s.objs[p.obj]
	if ob.of&ofCopy != 0 {
		return
	}
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
//
//go:noinline
func (s *State) sacrificedThisTurn(k *prep, x *kop.Expr, ctx evalCtx) int {
	pl := &s.pl[SeatIx(ctx.ctrl)]
	if x.N == 1 {
		return int(pl.sac[sacArtifact])
	}
	if x.N == kop.SacrificedFood {
		return int(pl.sac[sacFood])
	}
	return int(pl.sac[sacAll])
}
