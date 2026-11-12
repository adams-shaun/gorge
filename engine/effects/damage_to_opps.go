package effects

import "github.com/adams-shaun/gorge/engine/state"

// damageToOppsThisTurn is Forge's DamageToOppsThisTurn player property
// (AbilityUtils.playerXProperty -> Player.getOpponentsAssignedDamage): the
// damage dealt THIS turn to p's opponents, summed over the living seats that
// are not p. Damage a redirect moved onto a permanent is excluded because the
// Host fold only counts player Damage events, and a lost seat leaves
// AliveFrom(0), so it stops contributing. p's own damage never counts.
//
// It lives here as one free helper so the TriggeredPlayer$-ref arm
// (evalPlayerRefProperty) and the PlayerCountPropertyYou$ arm
// (evalCov3PlayerScalar) cannot drift: both ask the same Host fold over the
// same seat set.
func damageToOppsThisTurn(h Host, g *state.Game, p state.PlayerID) int32 {
	var n int32
	for _, q := range g.AliveFrom(0) {
		if q != p {
			n += h.DamageTakenThisTurn(q)
		}
	}
	return n
}
