package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the remember/scope params gorge already honours
// (RememberChanged$ on ChangeZoneAll, ForgetOtherRemembered$, RememberLKI$,
// RememberControlled$, RememberDamaged$, Discard UnlessType$). Every path
// here is gated on an instruction bit no FDN/BLB table sets.

// rememberLeaveAll is leaveAll for a ChangeZoneAll with RememberChanged$:
// the swept permanents join the remembered set in gorge's move order (seat
// 0's battlefield list, then seat 1's: effChangeZoneAll's emitMove).
//
//go:noinline
func (s *State) rememberLeaveAll(k *prep, mask slotMask, z kop.Zone) {
	var order [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		n = s.battlefieldOrder(p, mask.and(s.ctrlM[p]), order[:], n)
	}
	var objs [MaxPerm]Obj
	for i := 0; i < n; i++ {
		objs[i] = s.perms[order[i]].obj
	}
	s.leaveAll(k, mask, z)
	for i := 0; i < n; i++ {
		s.remember(objs[i])
	}
}

// forgetOtherMove runs an ExileEx/ReturnEx with MvForgetOther (gorge
// effChangeZone ForgetOtherRemembered$): the object is resolved first (a
// RefRemembered operand names the whole set, copied out), then the
// remembered set is forgotten, then each object moves, MvRemember re-adding
// it. It reports a pending ask (a returned Aura's host choice).
//
//go:noinline
func (s *State) forgetOtherMove(k *prep, in *kop.Instr) bool {
	var objs [maxRem]Obj
	n := 0
	if in.Obj == kop.RefRemembered {
		n = copy(objs[:], s.g.rem[:s.g.nrem])
	} else if o := s.objRefObj(in.Obj); o != NoObj {
		objs[0], n = o, 1
	}
	s.g.nrem, s.g.remCnt = 0, 0 // both halves (gorge forgetOther)
	for i := 0; i < n; i++ {
		o := objs[i]
		if in.Op == kop.OpReturnEx {
			if s.execReturnObj(k, in, NoSlot, o) {
				return true // known: a CR 508.4 defender ask here drops the later remembered objects (nrem is zeroed)
			}
			continue
		}
		if in.C&kop.MvFromBattlefield != 0 && s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		if in.C&kop.MvFromGraveyard != 0 && s.objs[o].zone != kop.ZoneGraveyard {
			continue // moved since: a new object (CR 400.7), as execGen's OpExile
		}
		if s.moveObj(k, in.Obj, o, kop.ZoneExile, false) && in.C&kop.MvRemember != 0 {
			s.remember(o)
		}
	}
	return false
}

// markRemLKI: a RememberLKI$ move (MvLKI) took target o off the
// battlefield; when g.tli holds o's snapshot (snapResTarget), a remembered
// read of o takes the snapshot's controller (gorge c.Snap.ChangeZone,
// lkiControllerFor; CR 608.2h).
//
//go:noinline
func (s *State) markRemLKI(o Obj) {
	if i := s.g.tli & tliIdx; i != 0 && s.lk(i).obj == o {
		s.g.tli |= tliRemLKI
	}
}

// remLKIRec is the RememberLKI$ capture of remembered object o, nil when
// o is on the battlefield or has none (its live controller is read).
func (s *State) remLKIRec(o Obj) *lkiRec {
	if s.g.tli&tliRemLKI == 0 || o == NoObj || s.objs[o].zone == kop.ZoneBattlefield {
		return nil
	}
	if l := s.lk(s.g.tli & tliIdx); l.obj == o {
		return l
	}
	return nil
}

// matchRemLKI matches remembered object o off the battlefield with its
// captured controller substituted, every other characteristic live (gorge
// rememberedControllerLKI: no counter substitution, unlike matchTargetLKI).
// ok is false when no capture covers o.
//
//go:noinline
func (s *State) matchRemLKI(k *prep, f kop.FilterID, o Obj, ctx evalCtx) (m int, ok bool) {
	l := s.remLKIRec(o)
	if l == nil {
		return 0, false
	}
	// a token copy bounced to hand is still the permanent the capture
	// describes (CR 608.2h, 111.8): it is read, not ceased, until the next check
	if s.matchCardChain(k, f, s.objs[o].card, l.ctrl, ctx.ctrl, o, nil) {
		return 1, true
	}
	return 0, true
}

// remController is RefRememberedController: the first remembered object's
// controller, its RememberLKI$ capture once it left the battlefield (gorge
// rememberedControllersLKI), 255 with nothing remembered.
//
//go:noinline
func (s *State) remController() uint8 {
	if s.g.nrem == 0 {
		return 255
	}
	o := s.g.rem[0]
	if l := s.remLKIRec(o); l != nil {
		return l.ctrl
	}
	return s.ctrlOfObj(o)
}

// remOwner is RefRememberedOwner: the first remembered object's owner (CR
// 108.3: an object's owner never changes, so no LKI capture), 255 with
// nothing remembered (gorge ownersOf over an empty set: nobody manifests).
//
//go:noinline
func (s *State) remOwner() uint8 {
	if s.g.nrem == 0 {
		return 255
	}
	return s.objs[s.g.rem[0]].owner()
}

// bounceRemember is OpBounce with move flags (RememberLKI$: MvRemember and
// MvLKI): the bounced object is remembered, and its target capture marked.
//
//go:noinline
func (s *State) bounceRemember(k *prep, in *kop.Instr) {
	if in.C&kop.BncDelayedMany != 0 {
		s.bounceMany(k) // trig_delayedmany.go
		return
	}
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return
	}
	if in.C&kop.MvPermOrSpell != 0 && s.bouncePermOrSpell(k, o) {
		return // a spell, or no longer a permanent (tgt_permorspell.go)
	}
	fromLib := in.C&kop.BncFromLibrary != 0
	if fromLib && s.objs[o].zone != kop.ZoneLibrary {
		return // Origin$ Library: only a library card moves (zone_forget.go)
	}
	v := s.objs[o].ver
	s.moveRef(k, in.Obj, kop.ZoneHand, false)
	if s.objs[o].ver == v {
		return // nothing moved
	}
	if fromLib && in.C&kop.BncShuffleOwed != 0 {
		s.shuffleLib(s.objs[o].owner()) // the script owes a shuffle (a Defined$ fetch is not a search)
	}
	if in.C&kop.MvRemember != 0 {
		s.remember(o)
	}
	if in.C&kop.MvForgetChanged != 0 {
		s.forgetChanged(o) // zone_forget.go
	}
	if in.C&kop.MvLKI != 0 {
		s.markRemLKI(o)
	}
}

// pumpAllScoped is OpPumpAll over the permanents of the player Obj names
// ("creatures target player controls": gorge targetPlayerKindScope); an
// unresolvable player sweeps nothing (gorge fails closed).
//
//go:noinline
func (s *State) pumpAllScoped(k *prep, in *kop.Instr) {
	only := s.playerRef(k, in.Obj)
	if only == 255 {
		return
	}
	e := &s.res.e
	src := s.objRef(k, kop.RefSelf)
	a, b := s.value(k, in.A), s.value(k, in.B)
	for m := s.occ.and(s.ctrlM[only]); m.any(); m = m.rest() {
		sl := m.first()
		if s.isCreature(k, sl) && s.matchPerm(k, in.F, sl, e.ctrl, src) {
			s.touchSl(sl)
			if !s.eotAdd(k, sl, a, b) {
				break
			}
			s.grantEOT(sl, kop.KwMask(uint16(in.C)))
			s.promoKw(k, sl, kop.KwMask(uint16(in.C)))
		}
	}
}

// matchRemSource matches a remembered object that is the resolving
// ability's own source, off the battlefield (Arid Archway returning itself):
// matchCardOne has no source to test FOther against, so a clause with
// Other excludes it here (gorge's Other is "not the source card":
// Desert.Other). The controller is a RememberLKI$ capture's when one
// covers it.
//
//go:noinline
func (s *State) matchRemSource(k *prep, f kop.FilterID, o Obj, ctx evalCtx) int {
	ctrl := s.objs[o].owner()
	if l := s.remLKIRec(o); l != nil {
		ctrl = l.ctrl
	}
	if f == 0 {
		return 1
	}
	for ; f != 0; f = k.t.Filters[f].Or {
		fl := &k.t.Filters[f]
		if fl.Flags&kop.FOther == 0 && s.matchCardOne(k, fl, s.objs[o].card, ctrl, ctx.ctrl, o) {
			return 1
		}
	}
	return 0
}
