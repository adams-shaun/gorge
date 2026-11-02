package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Permission kinds. Cold code, out of line: a
// play permission record (an impulse draw's OpMayPlay) carries the
// granting static's riders, gorge's MayPlayFree (MayPlayWithoutManaCost$
// True), MayPlayIgnoreType (MayPlayIgnoreType$ True),
// MayPlayIgnoreColor (MayPlayIgnoreColor$ True) and MayPlayLimit (an
// integer once-per-turn cap), on mayPlay.kind. The hot walks reach this
// file only behind genPrep.anyMayKind / anyMayLimit.

// tableMayPlayKinds reports whether some program of t grants a play
// permission record with a kind rider (genPrep.anyMayKind).
func tableMayPlayKinds(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpMayPlay && (in.B.N&kop.MayPlayKinds != 0 || in.B.N&(kop.MayPlayAdventure|kop.MayPlayOwner) != 0) {
					return true
				}
			}
		}
	}
	return false
}

// tableMayPlayLimit reports whether some program of t grants a play
// permission record with a MayPlayLimit cap (genPrep.anyMayLimit).
func tableMayPlayLimit(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpMayPlay && in.B.N&kop.MayPlayLimitMask != 0 {
					return true
				}
			}
		}
	}
	return false
}

// mayLimitSpent reports whether r's MayPlayLimit cap is already reached.
// An Effect grant counts per grant (gorge, mayPlayCapOpen: only the
// plays stamped with its own key this turn), so the answer is read off the
// grant's records (cast_mayplaylimit.go); a printed limited static keeps
// the per-player histMayPlay count and never has a record.
//
//go:noinline
func (s *State) mayLimitSpent(r mayPlay) bool {
	lim := r.kind >> kop.MayPlayLimitShift
	return lim > 0 && r.until&mayUntilUsed>>4 >= lim
}

// enterFromGrantedZone puts o onto the battlefield and folds a land that
// came from a graveyard or exile into the may-play count (gorge
// scanMayPlaysThisTurn's MoveZone clause). The return-to-battlefield
// paths use it (a graveyard return counts before enterFromGrave), so a
// new one follows the same pattern.
func (s *State) enterFromGrantedZone(k *prep, o Obj, ctrl uint8, tapped bool, from kop.Zone) Slot {
	if k.gen != nil && k.gen.anyMayLimit {
		s.countMayPlayLand(k, o, from)
	}
	return s.enter(k, o, ctrl, tapped, from)
}

// countMayPlayLand folds a land moving from a graveyard or exile onto the
// battlefield into the may-play count (gorge scanMayPlaysThisTurn's
// MoveZone clause, also counted for a player's own cards an opponent
// plays). Callers gate on genPrep.anyMayLimit.
//
//go:noinline
func (s *State) countMayPlayLand(k *prep, o Obj, from kop.Zone) {
	if from != kop.ZoneGraveyard && from != kop.ZoneExile {
		return
	}
	ob := &s.objs[o]
	if k.t.Cards[ob.card].Types&kop.TLand == 0 {
		return
	}
	if s.g.hist[ob.owner()][histMayPlay] < 255 {
		s.g.hist[ob.owner()][histMayPlay]++
	}
}

// offerMayKind offers the play record r carries a kind rider for: a free
// record is gorge's MayPlayFree grant (the mana part waived, CR 118.9; the
// cast's other costs still apply), an any-type record its
// MayPlayIgnoreType rider (pipRider.AnyType: every pip, {C} included,
// payable by any mana), an any-colour record its MayPlayIgnoreColor rider
// (pipRider.AnyColor: a coloured pip is payable by any colour, {C} stays
// colourless-only). Under Config.GorgeUnpriced a rider cast the plan's mana
// under the rider cannot pay is unpriced, as for the play-from-zone statics
// (priorityMayPlay).
//
//go:noinline
func (s *State) offerMayKind(k *prep, d *Decision, p uint8, r mayPlay, sorc, landOK bool, av *avail) {
	if r.kind&kop.MayPlayAdvKind != 0 {
		s.offerAdventureGrant(k, d, p, r, sorc) // cast_adventuregrant.go
		return
	}
	mode := castMayPlay
	if r.kind&kop.MayPlayFree != 0 {
		mode |= castFree
	}
	anyMana := r.kind&kop.MayPlayAnyType != 0 && mode&castFree == 0
	anyColor := r.kind&kop.MayPlayIgnoreColor != 0 && !anyMana && mode&castFree == 0
	unpriced := false
	if (anyMana || anyColor) && s.cfg&cfgGorgeUnpriced != 0 {
		c := s.objs[r.obj].card
		cost := s.adjustCostGen(k, p, c, r.obj, k.cards[c].cost)
		cost = planUnderRider(cost, anyMana, anyColor)
		unpriced = !av.affords(&cost, pipMaskOf(&cost), 0)
	}
	if r.kind&mayKindRaise != 0 {
		s.g.raiseO = uint8(r.turn) // priced into adjustCostGen while offered
	}
	s.offerPlay(k, d, p, r.obj, mode, sorc, landOK, anyMana, anyColor, unpriced, av)
	s.g.raiseO = 0
}

// spendMayKind spends the permission record a cast in mode used when that
// record has a kind rider: a free cast spends a free record, any other
// the first record that is not free; an any-type record's rider rides
// the entry (sfAnyMana), an any-colour one the build (bflAnyColor). false:
// no such record (the caller spends a plain one).
//
//go:noinline
func (s *State) spendMayKind(k *prep, o Obj, p uint8, mode uint8) bool {
	free := mode&castFree != 0
	for i := 0; i < int(s.g.nmay); i++ {
		r := s.g.may[i]
		if r.obj != o || r.ctrl != p || r.ver != s.objs[o].ver || (r.kind&kop.MayPlayFree != 0) != free || s.mayLimitSpent(r) ||
			k.gen.anyMayWhileSrc && s.maySrcDead(r) {
			continue
		}
		if r.kind&(kop.MayPlayKinds|mayKindRaise) == 0 {
			// a plain record did not make a rider cast legal: keep
			// looking for the record that did (an any-type one may be
			// the only reason the printed cost was affordable). If none
			// exists the caller's fallback spends a plain record.
			continue
		}
		if r.kind&kop.MayPlayAnyType != 0 {
			s.bld().e.flags |= sfAnyMana
		} else if r.kind&kop.MayPlayIgnoreColor != 0 {
			s.bld().bfl |= bflAnyColor
		}
		if r.kind&mayKindRaise != 0 {
			s.g.raiseB = uint8(r.turn) // the cast's surcharge (cast_whileexiled.go)
		}
		s.removeMay(i)
		return true
	}
	return false
}

// mayPlayAnyType reports whether a permission lets p spend mana of any type on the
// card o: a record of that kind, or a static (Tinybones).
func (s *State) mayPlayAnyType(k *prep, p uint8, o Obj) bool {
	if k.gen.anyMayKind {
		for i := 0; i < int(s.g.nmay); i++ {
			if r := s.g.may[i]; r.obj == o && r.ctrl == p && r.ver == s.objs[o].ver && r.kind&kop.MayPlayAnyType != 0 && !s.mayLimitSpent(r) {
				return true
			}
		}
	}
	anyType := false
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		anyType = st.a.Static.StFlags&kop.StAnyManaType != 0
		return false
	})
	return anyType
}
