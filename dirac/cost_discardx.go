package dirac

// cost_discardx.go: gorge's potential planner for the announced-Discard<X>
// spell shape beside no printed {X} (Vengeful Dreams' `Cost$ W W
// Discard<X/Card.card(s)>` over SVar:X:Count$xPaid, Nostalgic Dreams).
// planDeclinesAddCost already models the cast planner's decline
// ("shape:additional_cost", pay.PlanNonManaAdmissible's "cost:discard");
// this is the potential planner's composed-price answer behind it
// (gorgeLowersRider's arm, complexity kept in a helper).

// discardXLowers is gorgeLowersRider's arm for that shape: gorge's potential
// composition prices the composed cost at X = 0 (potentialPlayVerdictShared),
// whose census is the printed sources and the pool -- no script gaps, and
// every activation's own cost payable from the pool as it stands. A {1}, {T}
// Orzhov Signet beside a floating pool of 0 prices W W insufficient (Razorverge
// Thicket's W alone cannot pay the pip pair): gorge's plan never chains one
// source's float into another's activation cost. So the cast is lowered only
// when the planner census pays the mana, and only the floating pool offers it
// beside that (FeatPoolPays).
func (s *State) discardXLowers(k *prep, p uint8, c CardID, o Obj, mayPlay bool) bool {
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	if s.g.mayPlayAny {
		cost = anyType(cost)
	}
	wasX := s.g.availX
	s.g.availX &= ^(availGaps | availAny)
	ok := s.affordsUngrantedFor(k, p, &cost, NoSlot, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)})
	s.g.availX = wasX
	return ok || mayPlay && s.mayPlayPrefixPays(k, p, c, o)
}
