package dirac

// Jump-start (CR 702.84a), cold: reached only through the retrace cast
// (prep.retrace), whose card also carries CardDef.JumpStart.
//
// "You may cast this card from your graveyard by discarding a card in
// addition to paying its other costs. Then exile this card." gorge offers it
// in legal_walk_alt.go's jump-start walk and charges it in beginCast's
// "jumpstart" mode, both through jumpstartExtra (Discard<1/Card>) folded onto
// the printed base: the retrace shape with any card discardable in place of a
// land card. So the kernel offers and builds it as the retrace cast
// (kw_retrace.go: offerRetraceCasts, the retraceTgt target byte, the
// trigPlRetrace mark and RetraceCost as the pick cost), kgen building
// RetraceCost as an unfiltered CostDiscard of one card.
//
// The one difference is the destination: gorge stamps FlagJumpstart
// (modeFlags), one of state.ExilesLeavingStack's flashback-family flags, so
// the spell is exiled whenever it leaves the stack (resolved, countered or
// fizzled). The kernel's flashback family is sfFlashback (exilesOnLeave,
// counterTo, spellToLib). finishBuild stamps it once the cast is paid, never
// before: the build reads sfFlashback as "cast for a flashback cost" (the
// mana price, the granted-flashback bookkeeping), which a jump-start cast is
// not -- it pays its printed cost, as gorge's retrace and jumpstart modes do
// alike (cast_begin.go's mode list).
//
// Canonically the cast is gorge's "jumpstart" option, CastModeOther like
// "retrace" (canon/translate.go), so the two share one canonical action.

// jumpStartEntry reports whether the spell entry e is a jump-start cast.
func jumpStartEntry(k *prep, e *stackEntry) bool {
	return e.kind == seSpell && k.t.Cards[e.card].JumpStart && retraceEntry(&k.t.Cards[e.card], e)
}

// DebugJumpStartNoExile skips the exile stamp (TestMBCJumpStartExileMutation:
// the lockstep must diverge on the card left in the graveyard).
var DebugJumpStartNoExile bool

// FeatJumpStartCard reports whether o's card has Jump-start (the oracle
// names its retrace-shaped cast gorge's "jumpstart" option).
func (s *State) FeatJumpStartCard(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].JumpStart
}
