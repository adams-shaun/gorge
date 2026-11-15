package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The filtered scoped prevention (kop.PrevScopeFiltered; Heavy Fog's attacking
// creatures, Pack Leader's Dogs): the record holds only the bit, because the
// filters are static card data. The Ext PreventDamageScope instruction that
// made it is found again from the record's obj, the Effect's source card, and
// carries the damage source's filter in F, the recipient permanents' in B.N
// and the player-recipient bits in C. kgen refuses a card with two filtered
// instructions that differ, so the first one found is the one.

// scopeInstr is the filtered Ext PreventDamageScope in record r's source card.
func (s *State) scopeInstr(k *prep, r *prevRec) *kop.Instr {
	if r.obj == NoObj || int(r.obj) >= MaxObj {
		return nil
	}
	ab := k.t.Cards[s.objs[r.obj].card].Abilities
	for i := range ab {
		for j := range ab[i].Prog {
			if in := &ab[i].Prog[j]; in.Ext() == kop.ExtPreventDamageScope && in.C&kop.PrevScopeFiltered != 0 {
				return in
			}
		}
	}
	return nil
}

// prevScopeFiltered is prevScopeHit past its combat, colour and creature-source
// reads for a filtered record: the damage source (the permanent at srcSl, else
// the card srcCard as a spell or a departed source) against the instruction's
// source filter, then the recipient.
func (s *State) prevScopeFiltered(k *prep, r *prevRec, srcSl Slot, srcCard CardID, t prevTgt) bool {
	in := s.scopeInstr(k, r)
	if in == nil {
		return false
	}
	ctl := uint8(r.ctl) // code-ok: a scoped record's ctl is a seat
	if f := in.F; f != 0 {
		if srcSl != NoSlot {
			if !s.matchPerm(k, f, srcSl, ctl, NoSlot) {
				return false
			}
		} else if !s.matchCardGen(k, f, srcCard, ctl, ctl, NoObj) {
			// a departed or spell source has no live controller here: owner and
			// controller are the Effect's, so an OppCtrl source filter reads that
			// side, not gorge's last-known information (not lockstep-tested)
			return false
		}
	}
	c := int(in.C)
	if c&kop.PrevScopeAll != 0 {
		return true
	}
	if t.dst == NoSlot {
		return c&kop.PrevScopeAnyPlayer != 0 ||
			c&kop.PrevScopePlayer != 0 && t.rcp == ctl ||
			c&kop.PrevScopeOppPlayer != 0 && t.rcp != ctl
	}
	if c&kop.PrevScopeCreatures != 0 && t.rcp == ctl && s.isCreature(k, t.dst) {
		return true
	}
	return in.B.N != 0 && s.matchPerm(k, kop.FilterID(in.B.N), t.dst, ctl, NoSlot)
}
