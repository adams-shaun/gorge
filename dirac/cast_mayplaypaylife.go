package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A may-play permission whose alternative cost is life (Inside Information:
// "If you cast a spell this way, pay life equal to its mana value rather
// than pay its mana cost"; MayPlayAltManaCost$ PayLife<ConvertedManaCost>,
// CR 118.9). gorge prices the cast through the grant's alternative only, so
// the permission offers no mana-cost cast: the kernel's record is a free
// record (the mana cost waived, its other costs and the cost statics still
// owed) that also owes the card's mana value in life. A land is played, never
// cast, so it pays nothing. Cold code: every site is behind a record's mark.

// mayUntilPayLife marks a free record whose cast owes its mana value in
// life. It rides the until byte's bit 7, which a limited grant uses as its
// head mark; a pay-life record never carries a cap (kgen refuses the
// pair), so payLife requires the cap bits to be clear.
const mayUntilPayLife uint8 = 0x80

// payLife reports whether r is a pay-life record.
func (r mayPlay) payLife() bool {
	return r.until&mayUntilPayLife != 0 && r.kind&kop.MayPlayLimitMask == 0
}

// payLifeMark is the until bits OpMayPlay's B puts on the record it grants.
func payLifeMark(b int16) uint8 {
	if b&kop.MayPlayPayLife != 0 {
		return mayUntilPayLife
	}
	return 0
}

// payLifeKind is the kind bits OpMayPlay's B puts on the record it grants:
// the mana cost is waived.
func payLifeKind(b int16) uint8 {
	if b&kop.MayPlayPayLife != 0 {
		return kop.MayPlayFree
	}
	return 0
}

// payLifeOwed is the life p's cast of card c owes through a pay-life
// permission: its mana value (X counts 0 off the stack).
func payLifeOwed(k *prep, c CardID) int {
	return int(k.t.Cards[c].Cost.MV())
}

// payLifeOfferable reports whether p can pay o's cast through a pay-life
// record (CR 119.4): the life covers the mana value beside any life the
// spell's own cost owes. A land is played for nothing.
func (s *State) payLifeOfferable(k *prep, p uint8, o Obj) bool {
	c := s.objs[o].card
	if k.cards[c].land {
		return true
	}
	own := 0
	if ac := spellAddCost(k, c); ac.Flags&kop.CostPayLife != 0 {
		own = int(ac.N)
	}
	return payLifeOwed(k, c)+own <= int(s.pl[p].life)
}

// payMayPlayLife pays the cast being built's mana value in life.
func (s *State) payMayPlayLife(k *prep) {
	e := &s.bld().e
	if n := payLifeOwed(k, e.card); n > 0 {
		s.loseLife(k, e.ctrl, n)
	}
}
