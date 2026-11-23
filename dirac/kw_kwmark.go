package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted intimidate, infect, skulk and horsemanship (CR 702.13a, 702.90,
// 702.118a, 702.31b). None has a KwMask bit; the printed keyword is
// CardDef.Evasion EvIntimidate / EvSkulk / EvHorsemanship or CardDef.Infect. A grant comes three ways, all read beside the printed field
// by evasionAt and infectAt (so blocking, combat and noncombat damage, the
// LKI snapshot and the withInfect filter word all see it, as gorge's derived
// keywords are blind to where the keyword came from):
//
//   - a Continuous static's kop.Static.Evasion / Infect (AddKeyword$
//     Intimidate / Infect: Bellowing Tanglewurm, Vow of Malice, Grafted
//     Exoskeleton), a layer-6 grant: a later loss of the recipient's
//     abilities removes it (landwalkGranted's rule);
//   - a Pump's Duration$ Permanent KW$ Horsemanship: kop.Override.Horsemanship
//     on the grant's override record (ovHorsemanship), which a cleanup step
//     leaves alone and the permanent's leaving ends (Riding the Dilu Horse);
//   - OpExt ExtKwMark (Pump / PumpAll KW$: Tainted Strike, Hideous Visage):
//     the arena slot bits mechIntimidate / mechInfect, ended in the cleanup
//     step (CR 514.2) and when a new permanent takes the slot (arenaClrSlot,
//     CR 400.7). A mark made while the permanent had lost its abilities also
//     sets the post-loss twin, which holds through the loss (CR 613.7; the
//     static_colourprot.go rule).

// prepKwMark arms the grant reads: anyEvasion (canBlock's evasion arm) for an
// intimidate grant, anyInfect (prepInfect, run after it) for an infect grant,
// and anyKwMark (the cleanup clear) for an ExtKwMark.
func prepKwMark(t *kop.Table, g *genPrep) {
	scan := func(abs []kop.Ability) {
		for j := range abs {
			a := &abs[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous {
				g.anyEvasionGrant = g.anyEvasionGrant || a.Static.Evasion != 0
				g.anyInfectGrant = g.anyInfectGrant || a.Static.Infect
			}
			for i := range a.Prog {
				if in := &a.Prog[i]; in.Op == kop.OpExt && in.Ext() == kop.ExtKwMark {
					g.anyKwMark = true
					g.anyEvasionGrant = g.anyEvasionGrant || in.C&(kop.KwMarkIntimidate|kop.KwMarkSkulk|kop.KwMarkHorsemanship) != 0
					g.anyInfectGrant = g.anyInfectGrant || in.C&kop.KwMarkInfect != 0
				}
			}
		}
	}
	for i := range t.Cards {
		scan(t.Cards[i].Abilities)
	}
	for i := range t.Emblems {
		scan(t.Emblems[i].Abilities)
	}
	for i := range t.Overrides {
		g.anyEvasionGrant = g.anyEvasionGrant || t.Overrides[i].Horsemanship
	}
	g.anyEvasion = g.anyEvasion || g.anyEvasionGrant
}

// kwMarkUses reports the ExtKwMark masks' need (arenaUses).
func kwMarkUses(in *kop.Instr, u *[numMech]bool) {
	if in.Ext() != kop.ExtKwMark {
		return
	}
	if in.C&kop.KwMarkIntimidate != 0 {
		u[mechIntimidate], u[mechIntimidatePost] = true, true
	}
	if in.C&kop.KwMarkInfect != 0 {
		u[mechInfect], u[mechInfectPost] = true, true
	}
	if in.C&kop.KwMarkSkulk != 0 {
		u[mechSkulk], u[mechSkulkPost] = true, true
	}
	if in.C&kop.KwMarkHorsemanship != 0 {
		u[mechHorsemanship], u[mechHorsemanshipPost] = true, true
	}
}

// kwMarkExt is ExtKwMark: Obj, or every battlefield permanent matching F with
// Obj RefNone (the sweep fixed before any mark lands, CR 611.2c), gains the
// keywords C names until end of turn.
func (s *State) kwMarkExt(k *prep, in *kop.Instr) {
	if in.Obj == kop.RefNone {
		var ord [MaxPerm]Slot
		n := s.sweepHits(k, in.F, &ord)
		for _, sl := range ord[:n] {
			s.kwMark(k, sl, in.C)
		}
		return
	}
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		s.kwMark(k, sl, in.C)
	}
}

func (s *State) kwMark(k *prep, sl Slot, c int16) {
	if c&kop.KwMarkIntimidate != 0 {
		s.kwMarkSet(k, sl, mechIntimidate, mechIntimidatePost)
	}
	if c&kop.KwMarkInfect != 0 {
		s.kwMarkSet(k, sl, mechInfect, mechInfectPost)
	}
	if c&kop.KwMarkSkulk != 0 {
		s.kwMarkSet(k, sl, mechSkulk, mechSkulkPost)
	}
	if c&kop.KwMarkHorsemanship != 0 {
		s.kwMarkSet(k, sl, mechHorsemanship, mechHorsemanshipPost)
	}
	if c&kop.KwMarkSwitchPT != 0 {
		// CR 613.4d: each switch exchanges what the earlier ones left, so
		// only the parity matters (gorge derive.go's switches%2); the
		// cleanup step clears the bit with the turn's pumps (turn.go), and
		// a new permanent in the slot starts at 0 (zones.go)
		s.perms[sl].spare ^= spareSwitchPT
		s.touchSl(sl)
	}
}

// ptSwitched: the permanent at sl has an odd number of until-end-of-turn P/T
// switches in force (KwMarkSwitchPT).
func (s *State) ptSwitched(_ *prep, sl Slot) bool { return s.perms[sl].spare&spareSwitchPT != 0 }

// sbaPTMoved: something besides the inline fields can have lowered sl's
// toughness below its printed value, an effect record or a P/T switch (the
// state-based check's no-damage shortcut, turn.go sba).
func (s *State) sbaPTMoved(k *prep, sl Slot) bool { return s.fxM.has(sl) || s.ptSwitched(k, sl) }

func (s *State) kwMarkSet(k *prep, sl Slot, m, post mech) {
	if !s.mSl(k, m, sl) {
		s.mSlSet(k, m, sl)
		s.touchSl(sl)
	}
	if !s.mSl(k, post, sl) && s.genLost(k, sl) {
		s.mSlSet(k, post, sl)
		s.touchSl(sl)
	}
}

// kwMarked: the permanent at sl holds the mark m (and its abilities are not
// lost), or its post-loss twin.
func (s *State) kwMarked(k *prep, sl Slot, m, post mech) bool {
	return s.mSl(k, post, sl) || s.mSl(k, m, sl) && !s.genLost(k, sl)
}

// kwMarkClear ends the marks (the cleanup step, CR 514.2), touching each
// permanent that held one.
func (s *State) kwMarkClear(k *prep) {
	held := s.mSlMask(k, mechIntimidate).or(s.mSlMask(k, mechInfect)).or(s.mSlMask(k, mechSkulk)).or(s.mSlMask(k, mechHorsemanship))
	for _, m := range [...]mech{mechIntimidate, mechIntimidatePost, mechInfect, mechInfectPost, mechSkulk, mechSkulkPost, mechHorsemanship, mechHorsemanshipPost} {
		s.mSlZero(k, m)
	}
	for m := held.and(s.occ); m.any(); m = m.rest() {
		s.touchSl(m.first())
	}
}

// staticKwGrant reports whether a Continuous static in force grants the
// permanent at sl the keyword has picks out (a layer-6 grant: not one older
// than a loss of sl's abilities).
func (s *State) staticKwGrant(k *prep, sl Slot, has func(st *kop.Static) bool) bool {
	lts, lost := s.lossAt(k, sl)
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatContinuous || !has(&ab.Static) ||
			lost && s.grantTs(st) < lts {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, ab.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

func staticGrantsIntimidate(st *kop.Static) bool { return st.Evasion&kop.EvIntimidate != 0 }
func staticGrantsInfect(st *kop.Static) bool     { return st.Infect }
func staticGrantsSkulk(st *kop.Static) bool      { return st.Evasion&kop.EvSkulk != 0 }

// evasionGranted is the permanent at sl's granted CardDef.Evasion bits.
//
//go:noinline
func (s *State) evasionGranted(k *prep, sl Slot) uint8 {
	var ev uint8
	if s.kwMarked(k, sl, mechIntimidate, mechIntimidatePost) || s.staticKwGrant(k, sl, staticGrantsIntimidate) {
		ev = kop.EvIntimidate
	}
	if s.kwMarked(k, sl, mechSkulk, mechSkulkPost) || s.staticKwGrant(k, sl, staticGrantsSkulk) {
		ev |= kop.EvSkulk
	}
	if s.kwMarked(k, sl, mechHorsemanship, mechHorsemanshipPost) || s.ovHorsemanship(k, sl) {
		ev |= kop.EvHorsemanship
	}
	return ev
}

// ovHorsemanship: an active override record grants sl horsemanship (a Pump
// Duration$ Permanent KW$ Horsemanship, kop.Override.Horsemanship). The record
// goes with the permanent (CR 400.7), and, a layer-6 grant, one older than the
// permanent's loss of all abilities grants nothing (hasBanding's rule).
func (s *State) ovHorsemanship(k *prep, sl Slot) bool {
	if !s.fxM.has(sl) {
		return false
	}
	lts, lost := s.lossAt(k, sl)
	found := false
	s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
		if (!lost || ts >= lts) && o.Horsemanship {
			found = true
		}
	})
	return found
}

// infectGranted: the permanent at sl has a granted infect.
//
//go:noinline
func (s *State) infectGranted(k *prep, sl Slot) bool {
	return s.kwMarked(k, sl, mechInfect, mechInfectPost) || s.staticKwGrant(k, sl, staticGrantsInfect)
}

// hasSkulkPerm is the withSkulk word on a battlefield permanent: the printed
// keyword (none face down or after a lose-all-abilities effect) or a granted
// one.
func (s *State) hasSkulkPerm(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	return c != kop.NoCard && k.t.Cards[c].Evasion&kop.EvSkulk != 0 && !s.faceDown(k, sl) &&
		s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0 ||
		k.gen != nil && k.gen.anyEvasionGrant && s.evasionGranted(k, sl)&kop.EvSkulk != 0
}
