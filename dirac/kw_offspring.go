package dirac

// Offspring (CR 702.175, Bloomburrow), copied from gorge (rules
// kw_offspring.go, legal_walk_hand.go's "(offspring)" offer, cast_begin.go
// castModeOffspring; cards kw_offspring.go's expansion):
//
//   - the offspring cost is an optional ADDITIONAL cost offered as its own
//     cast option beside the plain cast (gorge Mode "offspring", the
//     Buyback shape: base + offspring, never an ask). The kernel's kicked
//     cast is that option: CardDef.Offspring cards carry the offspring
//     cost in CardDef.Kicker (no KwKicker), and the paid flag is sfKicked
//     on the entry (gorge's FlagOffspringPaid / Object.OffspringPaid), so
//     stackEntry and State grow by nothing. kgen refuses a card with both.
//   - as the spell resolves into a permanent the kicked mark moves to the
//     permanent (mechKicked, turn_gen.go), cleared when it leaves.
//   - the expansion's enter trigger is unconditional and mints
//     Count$OffspringPaid copies (lowered as Count$Valid Card.Self+kicked):
//     one when the permanent's cast paid the cost, none for a plain cast,
//     a source that left, or the token copy itself (never cast).
//   - the copy is a 1/1 (SetPower$/SetToughness$ 1): the copy op's
//     exception override, a layer-1 copiable value (copy_exceptions.go).

// offspringOffer: card c has Offspring and its "(offspring)" cast is
// offered beside the plain one (the kicked cast's offer, legal.go).
func offspringOffer(k *prep, c CardID) bool {
	return k.t.Cards[c].Offspring || k.t.Cards[c].Buyback || k.t.Cards[c].Entwine
}

// FeatOffspringCast reports that object o's kicked cast (AkCast mode bit 0)
// is gorge's "(offspring)" cast: its card has Offspring.
func (s *State) FeatOffspringCast(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Offspring
}

// FeatBuybackCast reports that object o's kicked cast is gorge's
// "(buyback)" cast: its card has Buyback (kw_buyback.go).
func (s *State) FeatBuybackCast(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Buyback
}
