package dirac

// The per-player cards-discarded-this-turn tally:
// player.disc.

// DebugDiscardedNoTurnReset (TestDiscturnMutation): beginTurn keeps the
// cards-discarded-this-turn tally.
var DebugDiscardedNoTurnReset bool

// noteDisc records the discard of hand card o by its owner (a cost discard
// is paid from the payer's own hand, CR 118.2a; gorge CardsDiscardedThisTurn
// counts it by the object's owner). Saturating. discardObj calls it for a
// graveyard discard only: gorge rewrites a madness discard's move Text to
// "discarded (madness exile)" (engine/rules/replacement.go:156), which
// events.IsDiscard rejects, so its ledger never counts that one.
//
//go:noinline
func (s *State) noteDisc(o Obj) {
	pl := &s.pl[SeatIx(s.objs[o].owner())]
	if pl.disc < 255 {
		pl.disc++
	}
}

// discardedThisTurn is ExDiscardedThisTurn (gorge CardsDiscardedThisTurn).
//
//go:noinline
func (s *State) discardedThisTurn(ctx evalCtx) int {
	return int(s.pl[SeatIx(ctx.ctrl)].disc)
}
