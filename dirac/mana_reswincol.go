package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A lucky Condition-split source (Gemstone Caverns with a luck counter) in a
// resolution's mana window: the trigger-cost window (winTrig, a trigger's own
// cost the planner has no witness for, Minion Reflector's {2}) and the unless
// and ward windows (winPay, winWard). gorge's planner defers the source, so
// the manual window's answer activates it, and the arm's colour ask (its
// SubAbility$ Mana's "mana_color" choice, no mana ability SA) is a decision of
// its own (oracle gorge.go manaChoiceResolving), as in a cast's window
// (mana_condsplit.go splitWinColArm). The walk taps the source and stops on
// the ask (raGenWinCol, ChGenManaChoice); the answer floats the arm's mana in
// the picked colour and runs the window on (winAskResume), CR 605.3b.

// resWinColArm is windowTapWalk's arm for a resolution's window: sl is a lucky
// Condition-split source, so the source taps and the walk waits on the colour.
func (s *State) resWinColArm(k *prep, p uint8, sl Slot, census bool) bool {
	if census || s.g.winMode == winNone || !s.splitWinColAsk(k, sl) {
		return false
	}
	s.g.winSrc = sl
	s.windowTapCost(k, sl, false)
	s.res.ask, s.res.askP = raGenWinCol, p
	return true
}

// winAskPending: a resolution's window waits on a pick (a tap-another
// source's permanent, raGenWinTap) or a lucky source's colour (raGenWinCol).
func (s *State) winAskPending() bool {
	return s.res.ask == raGenWinTap || s.res.ask == raGenWinCol
}

// splitWinColOptions lists the lucky Condition-split source sl's colour ask:
// the arm's colours in the askManaColour order (a printed order, else W U B R
// G), gorge's mana_color choice.
func (s *State) splitWinColOptions(k *prep, sl Slot, d *Decision) {
	d.Sub = ChGenManaChoice
	a := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.splitAb(s.cardAt(sl))]
	var ord [kop.NumManaKinds]uint8
	n, printed := colourAskOrder(a, uint8(a.SplitCols), &ord)
	cols := ord[:n]
	if !printed {
		cols = cols[:0]
		for c := uint8(0); c < 5; c++ {
			if a.SplitCols&(1<<c) != 0 {
				cols = append(cols, c)
			}
		}
	}
	for _, c := range cols {
		d.add(MakeAction(AkChoose, 0, 0, c))
	}
}

// applyAskWinCol floats the arm's mana in the picked colour and runs the
// paused window on.
func (s *State) applyAskWinCol(k *prep, a Action) {
	r := &s.res
	p := r.askP
	r.ask = raNone
	cost := s.winTapCost(k)
	s.splitWinColDo(k, p, s.g.winSrc, int(a.Mode()))
	s.winAskResume(k, p, &cost)
}
