package dirac

// PredHasXCost (gorge `hasXCost`, effects/filter.go) never
// matches a face-down object: it has no mana cost (CR 708.2, 708.4), so a
// morph with {X} in its printed cost (Hooded Hydra) cast or turned face down
// does not count as a spell or permanent with {X} in its mana cost.

// hasXCostPerm is PredHasXCost for the permanent at sl.
func (s *State) hasXCostPerm(k *prep, sl Slot) bool {
	return !s.faceDown(k, sl) && k.cards[s.cardAt(sl)].cost.X != 0
}

// hasXCostCard is PredHasXCost for card c as object o off the battlefield
// (a face-down spell on the stack is the face-down case; faceDownAt is false
// everywhere else, as gorge's Move clears FaceDown off the battlefield and
// stack).
func (s *State) hasXCostCard(k *prep, c CardID, o Obj) bool {
	if k.cards[c].cost.X == 0 {
		return false
	}
	return k.gen == nil || !k.gen.anyFaceDown || !s.faceDownAt(k, o)
}
