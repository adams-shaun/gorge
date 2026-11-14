package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted intimidate and infect (CR 702.13a, 702.90). Neither keyword has a
// KwMask bit; the printed keyword is CardDef.Evasion EvIntimidate /
// CardDef.Infect. A grant comes two ways, both read beside the printed field
// by evasionAt and infectAt (so blocking, combat and noncombat damage, the
// LKI snapshot and the withInfect filter word all see it, as gorge's derived
// keywords are blind to where the keyword came from):
//
//   - a Continuous static's kop.Static.Evasion / Infect (AddKeyword$
//     Intimidate / Infect: Bellowing Tanglewurm, Vow of Malice, Grafted
//     Exoskeleton), a layer-6 grant: a later loss of the recipient's
//     abilities removes it (landwalkGranted's rule);
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
					g.anyEvasionGrant = g.anyEvasionGrant || in.C&kop.KwMarkIntimidate != 0
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
}

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
	held := s.mSlMask(k, mechIntimidate).or(s.mSlMask(k, mechInfect))
	for _, m := range [...]mech{mechIntimidate, mechIntimidatePost, mechInfect, mechInfectPost} {
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

// evasionGranted is the permanent at sl's granted CardDef.Evasion bits.
//
//go:noinline
func (s *State) evasionGranted(k *prep, sl Slot) uint8 {
	if s.kwMarked(k, sl, mechIntimidate, mechIntimidatePost) || s.staticKwGrant(k, sl, staticGrantsIntimidate) {
		return kop.EvIntimidate
	}
	return 0
}

// infectGranted: the permanent at sl has a granted infect.
//
//go:noinline
func (s *State) infectGranted(k *prep, sl Slot) bool {
	return s.kwMarked(k, sl, mechInfect, mechInfectPost) || s.staticKwGrant(k, sl, staticGrantsInfect)
}
