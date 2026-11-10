package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CopyPermanent over Defined$ ExiledWith (The Cloning of Shredder's
// chapters II and III: "create a token that's a copy of a card exiled with
// this Saga"). gorge's effCopyPermanent resolves the selector through
// exiledWithSet, so the copy source is every card in any seat's exile zone
// exiled with the resolving source, seat 0 first and in exile order. The
// kernel keeps that association in the cold exw table (zone_exiledwith.go);
// this op walks the same zones in the same order, reads each card through the
// lowered PredExiledWith filter (s.exwCard), sizes every source's copy count
// through the CreateToken replacements first (as copyAllEach does), then
// mints the copies source by source in one entry batch (CR 603.6a).
//
// A copy that needs an as-enters choice or an Aura host parks through the same
// ask arm OpCopyAsTokenEx uses. The answer path resumes the op with the card
// the parked token carries and s.g.pickLeft copies, which is only correct when
// no later exiled card remains; when one does, the op stalls StallCopyAllAsk
// rather than mint the later cards out of order. An exiled card is not a
// battlefield permanent, so the copy has no template slot (NoSlot).
func (s *State) copyExiledWith(k *prep, in *kop.Instr) bool {
	e := &s.res.e
	var ord [maxExw]Obj
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		pl := &s.pl[p]
		for j := 0; j < int(pl.exileN); j++ {
			o := pl.exile[j]
			if n >= len(ord) {
				s.capFull(StallExiledWith)
				return false
			}
			if !s.matchCardGen(k, in.F, s.objs[o].card, s.objs[o].owner(), e.ctrl, o) {
				continue
			}
			ord[n] = o
			n++
		}
	}
	if n == 0 {
		return false // nothing exiled with the source: no copy (gorge's empty set)
	}
	ctrl := e.ctrl
	// Size every source's copy count before the first copy enters, as gorge's
	// CopyPermanent Defined arm does (a CreateToken rewriter is read once).
	var cnt [maxExw]int16
	var rwCard CardID
	rwN, rewritten := 0, false
	for i := 0; i < n; i++ {
		tc, c, rw, ok := s.copyTokenRewrite(k, ctrl, s.objs[ord[i]].card, 1, false)
		if !ok {
			return false // StallTokRewrite
		}
		if rw {
			rwCard, rwN, rewritten = tc, rwN+c, true
		}
		cnt[i] = int16(c)
	}
	if rewritten {
		// a rewriter replaced every copy with a plain mint of its card: no
		// copy is left for the riders (copyAllEach's arm)
		s.mintRewritten(k, ctrl, rwCard, rwN)
		return false
	}
	var eb entBatch
	for i := 0; i < n; i++ {
		c := s.objs[ord[i]].card
		aura := k.cards[c].types&kop.TAura != 0 && k.t.Cards[c].Enchant.Kind == kop.TgtObject
		for j := 0; j < int(cnt[i]); j++ {
			if s.nearCapacity(k) {
				s.stall(StallBoard)
				return false
			}
			host, st := s.auraHostFor(k, c, ctrl, aura)
			switch st {
			case hostNone:
				continue // CR 303.4g: the Aura token isn't created
			case hostAsk:
				rem := cnt[i] - int16(j) - 1
				if rem > 255 || !s.lastExiledCopy(cnt[:n], i, j) {
					s.capFull(StallCopyAllAsk)
					return false
				}
				s.parkCopy(c, ctrl, raGenEnchant)
				s.g.pickLeft = uint8(rem)
				return true
			}
			kind := s.enterChoiceKind(k, c)
			if kind == 0 && k.gen.anyBecomeCopy {
				kind = s.cloneTokenAsk(k, in, c)
			}
			if kind != 0 {
				rem := cnt[i] - int16(j) - 1
				if rem > 255 || !s.lastExiledCopy(cnt[:n], i, j) {
					s.capFull(StallCopyAllAsk)
					return false
				}
				t := s.parkCopy(c, ctrl, raGenTokChoice)
				s.g.pickLeft = uint8(rem)
				s.g.pre = preAsk{obj: t, kind: kind}
				return true
			}
			t := s.newToken(c, ctrl)
			s.objs[t].of |= ofCopy
			s.copyTokenEnterFrom(k, in, t, host, NoSlot, &eb)
		}
	}
	s.fireEntries(k, &eb)
	return false
}

// lastExiledCopy reports whether no copy follows source i's copy j: the ask
// arms can only hand the remaining copies of the parked card to the answer.
func (s *State) lastExiledCopy(cnt []int16, i, j int) bool {
	if j+1 < int(cnt[i]) {
		return false
	}
	for x := i + 1; x < len(cnt); x++ {
		if cnt[x] > 0 {
			return false
		}
	}
	return true
}
