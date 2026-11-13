package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the Recruit keyword action (gorge effRecruit,
// engine/effects/recruit.go; CR 701.xx, 701.9, 111.1): the resolving
// controller draws a card, then discards a card, then creates one 1/1 white
// Human Soldier token if the discarded card was a nonland. Cold and out of
// line: a table with no Recruit never reaches this file (kop.ExtRecruit).

// DebugRecruitAlwaysToken mints the soldier whatever was discarded (or when
// nothing was). TestRecruitLockstep's mutation flips it and requires a
// divergence from gorge.
var DebugRecruitAlwaysToken bool

// raRecruitDiscard is the Recruit discard pick. It lives above the ask
// families that offset from raGenManifestDread (the modal activation, unread,
// CzParams and Blight asks use +1..+3 and +40..+44) so appending it to the
// generated-table iota block cannot collide with a sibling family.
const raRecruitDiscard = raGenManifestDread + 60

// recruitStart is OpExt ExtRecruit: the draw, then the discard. A hand of at
// most one card discards it in place (gorge's strict-supersets no-ask rule);
// a larger hand poses the pick (raRecruitDiscard).
//
//go:noinline
func (s *State) recruitStart(k *prep) bool {
	p := s.res.e.ctrl
	s.draw(p) // CR 121.1: the draw comes first
	pl := &s.pl[p]
	switch {
	case pl.handN == 0:
		if DebugRecruitAlwaysToken {
			s.recruitToken(k, p)
		}
	case pl.handN == 1:
		o := pl.hand[0]
		nonland := !k.cards[s.objs[o].card].land
		s.discardObj(k, o)
		if nonland || DebugRecruitAlwaysToken {
			s.recruitToken(k, p)
		}
	default:
		s.res.ask = raRecruitDiscard
		s.res.askP = p
		s.res.chosen = p
		return false
	}
	return true
}

// recruitOptions poses the Recruit discard: one pick over the controller's
// hand in hand order (gorge effRecruit's KChoose Min = Max = 1, options in
// hand order). ChRecruitDiscard names gorge's option kind "discard"; the
// oracle reports the decision as DChoose, as gorge's KChoose is.
func (s *State) recruitOptions(k *prep, d *Decision) {
	d.Sub = ChRecruitDiscard
	pl := &s.pl[s.res.chosen]
	for i := 0; i < int(pl.handN); i++ {
		d.add(MakeAction(AkChoose, TCode(pl.hand[i]), 0, 0))
	}
}

// recruitPick applies the answered discard: the picked card and the soldier
// when it was a nonland (gorge reads the pick before the move, so the card
// type comes from the card definition; a token in hand cannot be a land).
func (s *State) recruitPick(k *prep, a Action) {
	p := s.res.chosen
	o := Obj(a.Source())
	pl := &s.pl[p]
	inHand := false
	for i := 0; i < int(pl.handN); i++ {
		if pl.hand[i] == o {
			inHand = true
			break
		}
	}
	if !inHand || s.objs[o].owner() != p {
		return // the pick left the hand meanwhile (gorge recruitPicks)
	}
	nonland := !k.cards[s.objs[o].card].land
	s.discardObj(k, o)
	if nonland || DebugRecruitAlwaysToken {
		s.recruitToken(k, p)
	}
}

// recruitTokenCard finds the Human Soldier token's CardID in the table by
// the pool's script key (the generated pool names a token card by it; the
// matchup keeps that name). The lowering refuses a corpus without the
// script, so a miss here is a table-generation bug and panics loudly.
func recruitTokenCard(k *prep) kop.CardID {
	const key = "w_1_1_human_soldier"
	for i := range k.t.Cards {
		if k.t.Cards[i].Name == key {
			return kop.CardID(i)
		}
	}
	panic("kernel: Recruit token " + key + " not in the table")
}

// recruitToken creates the one Human Soldier token (gorge
// h.EmitTokenCreate; a token enters untapped, from no zone, under p).
func (s *State) recruitToken(k *prep, p uint8) {
	card := recruitTokenCard(k)
	card, n, ok := s.replTokensCard(k, p, card, 1) // a rewriter's card (repl_tokrewrite.go)
	if !ok {
		return
	}
	for i := 0; i < n; i++ {
		if s.nearCapacity(k) {
			s.stall(StallBoard)
			return
		}
		o := s.newToken(card, p)
		sl := s.enter(k, o, p, false, kop.ZoneLibrary)
		if k.gen.anyTokCr {
			s.fireTokenCreated(k, sl)
		}
		s.res.lastTok = o
	}
}
