package dirac

// gorge's effDiscardRandom (engine/effects/cardflow.go:854,
// CR 701.8b) draws the engine RNG once per pick, including a pick from a
// one-card eligible list, and removes each pick from the ordered eligible
// list. The OpDiscard arm's zero-draw whole-hand shortcut is for the asking
// choosers only: ChooseRandom must draw for every card even when NumCards
// covers the hand, or every later random event in the game desyncs.

// DebugDiscRandomFirst makes the random discard pick eligible[0] (the
// mutation run TestDiscrandomMutation), still consuming the RNG draw so only
// the card identity changes. DebugDiscRandomHand1/Hand2 count picks from an
// eligible hand of exactly one / at least two cards (the lockstep's reach
// log).
var (
	DebugDiscRandomFirst bool
	DebugDiscRandomHand1 int
	DebugDiscRandomHand2 int
)

// discRandom discards up to n random cards from p's hand, one RNG draw per
// pick (gorge effDiscardRandom). gorge computes n = min(NumCards, eligible)
// from an ordered snapshot of the hand once, then draws from the shrinking
// snapshot with order-preserving removal; a discard trigger that adds a card
// to the hand mid-way is not eligible for the rest of this instruction.
func (s *State) discRandom(k *prep, p uint8, n int) {
	var eligible [MaxHand]Obj
	m := int(s.pl[p].handN)
	copy(eligible[:m], s.pl[p].hand[:m])
	if n > m {
		n = m
	}
	for i := 0; i < n; i++ {
		j := s.rng.IntN(m)
		if DebugDiscRandomFirst {
			j = 0
		}
		if m >= 2 {
			DebugDiscRandomHand2++
		} else {
			DebugDiscRandomHand1++
		}
		s.discardObj(k, eligible[j])
		copy(eligible[j:], eligible[j+1:m])
		m--
	}
}
