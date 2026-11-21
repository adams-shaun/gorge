package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Gemstone Caverns' opening-hand effect (CR 103.6a; kop.CardDef.LeylineGem):
// "If this is in your opening hand and you're not the starting player, you
// may begin the game with it on the battlefield with a luck counter on it.
// If you do, exile a card from your hand." It rides the Leyline ask
// (turn_gen.go): gorge's opening round (rules/opening_hand.go) leaves the
// card out of the starting player's asks (the !PlayFirst gate,
// newOpeningRound), puts the counter as the card enters (applyOpeningEffect)
// and then poses its "opening_exile" KChoose over the player's hand, in hand
// order, before the next "begin the game with" ask. The pending pick is
// genState.ley[2] (1 = asked); no State field is added.

// ChGenOpeningExile is the pick: source = the hand card exiled (gorge's
// "opening_exile", Min=Max=1).
const ChGenOpeningExile ChoiceKind = 214

// leyEligible reports whether o in player p's opening hand is asked about:
// a Leyline, and for the Gemstone shape only when p is not the starting
// player.
func (s *State) leyEligible(k *prep, p uint8, o Obj) bool {
	c := &k.t.Cards[s.objs[o].card]
	return c.Leyline && !(c.LeylineGem && p == s.startPl && !DebugLeylineGemUngated)
}

// DebugLeylineGemUngated asks the starting player about a Gemstone too, and
// DebugLeylineGemNoExile skips its hand-exile pick (mutation testing only:
// the lockstep must catch each).
var DebugLeylineGemUngated, DebugLeylineGemNoExile bool

// leyGemEnter finishes an accepted Leyline that is the Gemstone shape: the
// counters, then the hand-exile pick (none when the hand is empty).
func (s *State) leyGemEnter(k *prep, o Obj, sl Slot) {
	c := &k.t.Cards[s.objs[o].card]
	if !c.LeylineGem {
		return
	}
	if sl != NoSlot {
		s.placeCounters(k, sl, int(c.LeylineCtr), int(c.LeylineCtrN), s.g.leyP, ctrPutBy)
	}
	if s.pl[s.g.leyP].handN > 0 && !DebugLeylineGemNoExile {
		s.g.ley[2] = 1
	}
}

// leyGemOptions poses the pending hand-exile pick.
func (s *State) leyGemOptions(d *Decision) {
	pl := &s.pl[s.g.leyP]
	d.Kind, d.Sub, d.Player = DkChoose, ChGenOpeningExile, s.g.leyP
	for i := 0; i < int(pl.handN); i++ {
		d.add(MakeAction(AkChoose, TCode(pl.hand[i]), 0, 0))
	}
}

// leyGemAnswer exiles the picked hand card. A declined Leyline the pick
// takes is no longer one of the player's declined ones (ley[1] counts the
// eligible cards left in hand ahead of the next ask).
func (s *State) leyGemAnswer(k *prep, a Action) {
	s.g.ley[2] = 0
	o := Obj(a.Source())
	p := s.g.leyP
	if s.leyEligible(k, p, o) {
		pl := &s.pl[p]
		before := uint8(0)
		for i := 0; i < int(pl.handN) && pl.hand[i] != o; i++ {
			if s.leyEligible(k, p, pl.hand[i]) {
				before++
			}
		}
		if before < s.g.ley[1] {
			s.g.ley[1]--
		}
	}
	from := s.detach(o)
	s.placeFrom(o, kop.ZoneExile, false, from, false, false)
}
