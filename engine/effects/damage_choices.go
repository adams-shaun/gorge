package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects/params"
	"github.com/adams-shaun/gorge/engine/state"
)

// damageChoicePool is the pool a DealDamage's CardChoices$ / PlayerChoices$
// names (Comet, Stellar Pup's "a creature or player"): the battlefield objects
// matching CardChoices$, then the living seats matching PlayerChoices$, each in
// the deterministic AliveFrom seat walk and zone order. It is the ONE pool
// derivation: the ask offers exactly these and the damage walk reads the pick
// back from the same list. Nothing here targets (CR 115.10), so hexproof,
// shroud and protection do not narrow it.
func damageChoicePool(h Host, c *Ctx, dp *params.DealDamageParams) []state.Target {
	g := h.Game()
	var pool []state.Target
	if dp.CardChoices != "" {
		sc := c.SpecContext(c.Controller)
		bindSpecLayerTables(h, &sc.Layers, dp.CardChoices, false)
		for _, pl := range g.AliveFrom(0) {
			for _, id := range g.Zone(state.ZBattlefield, pl) {
				if MatchesSpecCtx(g, dp.CardChoices, id, sc) {
					pool = append(pool, state.Target{Obj: id})
				}
			}
		}
	}
	if dp.PlayerChoices != "" {
		for _, p := range g.AliveFrom(0) {
			if MatchesPlayerSpecFrom(g, dp.PlayerChoices, p, c.Controller, c.Source) {
				pool = append(pool, state.Target{Player: p, IsPlayer: true})
			}
		}
	}
	return pool
}

// dealDamageChoice is the resolution-time recipient choice of a DealDamage
// that names CardChoices$ / PlayerChoices$ (CR 608.2c): the ability's
// controller picks one entity from damageChoicePool as one 1..1 "choice" ask
// (askTargetChoice's player/card option shape, the tape-served boundary every
// mid-resolution choice shares). Random$ True (Whimsy, Faerie Dragon) draws the
// pick from the engine's rng instead of asking. A single entity is taken
// without an ask, and an unserved ask takes the first option -- the bot's own
// answer, so the stand-in and the validator agree. ok is false when the
// ability names no choice pool at all (the Defined$/target walk applies); an
// empty pool is ok with no recipient.
func dealDamageChoice(h Host, c *Ctx, sa *cards.SA, dp *params.DealDamageParams) (recipients []state.Target, ok bool) {
	if dp.CardChoices == "" && dp.PlayerChoices == "" {
		return nil, false
	}
	pool := damageChoicePool(h, c, dp)
	switch {
	case len(pool) <= 1:
		return pool, true
	case dp.Random:
		return []state.Target{pool[h.Rand(len(pool))]}, true
	}
	if picked := askTargetChoice(h, c, sa, c.Controller, "Choose a creature or player to be dealt damage", pool, 1, 1); len(picked) == 1 {
		return picked, true
	}
	return pool[:1], true
}
