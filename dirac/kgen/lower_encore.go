package kgen

// AB$ Encore (gorge kw_encore.go's expansion of K:Encore:<cost>, CR
// 702.141a): for each opponent, create a token copy of the source card that
// attacks that opponent this turn if able and gains haste until end of turn;
// all the copies of one activation are sacrificed together at the beginning
// of the next end step (CR 603.10a; gorge effects.Encore registers one
// end-step DelayedTrigger remembering the whole group).
//
// One OpExt ExtEncore (dirac/kw_encore.go) walks the opponents from the
// resolving controller in turn order (gorge AliveFrom), one copy each, and
// gives each copy the this-turn attack duty that names its opponent (gorge
// Object.EncoreAttackDefender; combat.encoreAttackDefender): the attackers
// decision offers the copy that player alone, never a planeswalker or battle
// of theirs, while that player is in the game and can be attacked.
//
// The copy is the copy op's CpSacAtEndDly rider (gorge scheduleAtEOT's
// __kwEncoreSacrifice, untracked: still put on the stack when the token
// already left, CR 603.7c), and a later copy of one resolution joins the
// first one's trigger (sacAtEndJoin), so the group is one trigger sacrificed
// together -- the behaviour cb7f606a27 fixed for the kernel's existing
// Encore-group paths. The haste is the copy op's PumpKeywords$ EOT grant
// (gorge encoreGrantHaste's UntilEOT layer grant: not a copiable value,
// dropped at the turn's end).
func (lc *lowerCtx) encore(ps *params) error {
	if lc.ab.Zone != zGraveyard || !lc.ab.Sorcery || !lc.ab.Cost.exileGCard || lc.ab.Cost.ExileGN != 1 {
		// the expansion's only shape (kw_encore.go): a graveyard
		// sorcery-speed activation whose cost is the printed cost plus
		// ExileFromGrave<1/CARDNAME> (the card exiles itself as the
		// cost's payment)
		return refuse("AB$ Encore outside the keyword expansion's shape")
	}
	lc.emit(lInstr{Op: "Encore", Obj: lRef{Kind: rSelf}, A: cst(1),
		CS: "SacAtEndDly|PumpEOT=Haste", Note: "one token copy per opponent, each attacking that opponent this turn if able"})
	return nil
}
