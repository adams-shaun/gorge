package kgen

// AB$ Encore (gorge kw_encore.go's expansion of K:Encore:<cost>, CR
// 702.141a): for each opponent, create a token copy of the source card that
// attacks that opponent this turn if able and gains haste until end of turn;
// all the copies of one activation are sacrificed together at the beginning
// of the next end step (CR 603.10a; gorge effects.Encore registers one
// end-step DelayedTrigger remembering the whole group).
//
// The kernel is two-seat (kop.CardDef.TwoSeat): the one opponent, one copy,
// and "attacks that opponent this turn if able" is the this-turn attack duty
// the MustAttack machinery reads (ExtAttacksIfAble,
// dirac/combat_attacksifable.go; gorge's RequirementSet names the sole
// defender, attack.go's encoreAttackDefender, and both engines force or
// withhold the declaration the same way the Heckling Fiends mark does).
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
	lc.twoSeat("one copy for each opponent read as one copy")
	lc.emit(lInstr{Op: "CopyAsToken", Obj: lRef{Kind: rSelf}, A: cst(1),
		CS: "SacAtEndDly|PumpEOT=Haste", Note: "one token copy per opponent (two seats: one)"})
	lc.emit(lInstr{Op: "AttacksIfAble", Obj: lRef{Kind: rLastCreated},
		Note: "the copy attacks the opponent this turn if able"})
	return nil
}
