package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// spellTargetsFeasibleHand: a spell cast from the hand has left it for the
// stack by the time its targets are chosen (CR 601.2a, 601.2c), so a target
// whose characteristics count the cards in a hand is judged without the spell:
// Sturmgeist ("power and toughness are each equal to the number of cards in
// your hand") is a 4/4 with four cards in hand and no target for Sungold
// Barrage ("toughness 4 or greater") once the Barrage itself is cast
// (gorge offerAsSpellOnStack probes the target census the same way).
//
// A static gated on a hand size reads it the same way: Demon's Jester
// (Hellbent, +2/+1 with no cards in hand) is a 4/3 target for Repel the Vile
// ("power 4 or greater") once the Repel itself has left the hand.
//
// Only a characteristic-defining P/T or a hand-gated static on the battlefield
// reads the hand this way, so every other offer keeps the plain check; the probe is a
// fork with the card on the stack, as planHoldsOnStack's second fork.
func (s *State) spellTargetsFeasibleHand(k *prep, c CardID, p uint8, o Obj) bool {
	if k.gen == nil || !s.battlefieldHasCDAPT(k) {
		return s.spellTargetsFeasible(k, c, p)
	}
	u := s.fork()
	defer s.unfork()
	u.detach(o)
	u.objs[o].zone = kop.ZoneStack
	return u.spellTargetsFeasible(k, c, p)
}

// battlefieldHasCDAPT: some permanent carries a characteristic-defining P/T or
// a static gated on a hand size.
func (s *State) battlefieldHasCDAPT(k *prep) bool {
	for seat := range s.ctrlM {
		for m := s.ctrlM[seat]; m.any(); m = m.rest() {
			c := s.cardAt(m.first())
			if k.gen.flags[c]&gfCDAPT != 0 || (len(k.handStat) > 0 && k.handStat[c]) {
				return true
			}
		}
	}
	return false
}

// prepHandStat marks every card with a static whose condition reads a hand
// size (ExHandSizeOf), directly or through an And.
func prepHandStat(t *kop.Table, p *prep) {
	any := false
	for i := range t.Exprs {
		any = any || t.Exprs[i].Kind == kop.ExHandSizeOf
	}
	if !any {
		return
	}
	p.handStat = make([]bool, len(t.Cards))
	for c := range t.Cards {
		ab := t.Cards[c].Abilities
		for i := range ab {
			if ab[i].Kind == kop.AbStatic && condReadsHand(t, ab[i].Static.Cond2) {
				p.handStat[c] = true
			}
		}
	}
}

func condReadsHand(t *kop.Table, id kop.CondID) bool {
	if id == 0 || int(id) >= len(t.Conds) {
		return false
	}
	c := &t.Conds[id]
	return valReadsHand(t, c.A) || valReadsHand(t, c.B) || condReadsHand(t, c.Sub[0]) || condReadsHand(t, c.Sub[1])
}

func valReadsHand(t *kop.Table, v kop.ValueRef) bool {
	if v.K != kop.VExpr || v.N <= 0 || int(v.N) >= len(t.Exprs) {
		return false
	}
	x := &t.Exprs[v.N]
	return x.Kind == kop.ExHandSizeOf || valReadsHand(t, x.Sub[0]) || valReadsHand(t, x.Sub[1])
}
