package dirac

// A scripted prefix whose taps queue a draw-then-discard trigger (Hydraulic
// Helper's tap under Rewrite History: "draw a card, then discard a card").
// gorge's prefix search (rules scriptSearch.visit) branches over every answer
// of an ask the payer's line poses, in the engine's option order, and offers
// the play when one answer leaves it playable; the kernel's resolvePrefixStack
// leaves any ask but a may-election unanswered, so the play waited above the
// discard and was never offered (CR 601.2g, 603.3, 701.9).

// prefixDiscardAsk reports whether p must choose a discard now: the
// resolution of a stack item the prefix's taps queued asks p for the cards.
func (s *State) prefixDiscardAsk(p uint8) bool {
	return s.ctl == ctlResolve && s.res.ask == raDiscard && s.res.askP == p && !s.over
}

// prefixDiscardLeft bounds the discard asks one prefix answers (a trigger that
// discards twice, or two such triggers).
const prefixDiscardLeft = 4

// resolvePrefixPlay is resolvePrefixStack for the prefix of play a by p. A
// discard ask the sorcery-speed resolutions reach is answered with the first
// answer, in option order, after which a is offered, as gorge's search does.
// With no such answer the ask stays pending, and the play is not offered.
func (s *State) resolvePrefixPlay(k *prep, a Action, p uint8, sp0 uint8, resolve bool) {
	s.resolvePrefixStack(k, sp0, resolve)
	for guard := 0; resolve && guard < prefixDiscardLeft && s.prefixDiscardAsk(p); guard++ {
		var d Decision
		s.resolveOptions(k, &d)
		pick := -1
		for i := 0; i < int(d.N) && pick < 0; i++ {
			t := s.fork()
			t.Apply(d.Options[i])
			t.resolvePrefixStack(k, sp0, true)
			var o Decision
			if !t.over && t.ctl == ctlPriority {
				t.restrOptions(k, &o, p)
				if hasOption(&o, a) {
					pick = i
				}
			}
			s.unfork()
		}
		if pick < 0 {
			return
		}
		s.Apply(d.Options[pick])
		s.resolvePrefixStack(k, sp0, true)
	}
}
