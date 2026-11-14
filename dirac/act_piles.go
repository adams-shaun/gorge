package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ext TwoPilesSac (gorge effTwoPiles' battlefield arm,
// cr-twopiles-battlefield-validcards): "separate all permanents target
// player controls into two piles; that player sacrifices all permanents
// in the pile of their choice" (Liliana of the Veil's -6). The pool is the
// battlefield permanents matching F that the player in Obj controls, read
// as the effect starts (nothing leaves while the asks are open). The
// resolving controller (Separator$ You) picks pile A one permanent at a
// time (ChGenPileSplit, gorge's Min 0 subset ask: Done at every step,
// picking the whole pool ends it); a one-permanent pool poses no split
// (pile A is that permanent, as gorge). The pool player then picks a pile
// (ChGenPileChoose: 0 pile A, 1 pile B) and sacrifices it as gorge's
// ChosenPile$ SacrificeAll does (fireSacAll, leaveAll). Pile A rides
// g.sacM (no sacrifice pick is open meanwhile) and the pool player
// g.pickP; both are cleared at the end. 0 B.

const (
	raPileSplitSac  uint8 = 246
	raPileChooseSac uint8 = 247
)

// pilesPool is the pool: the battlefield permanents matching in.F that
// player p controls.
func (s *State) pilesPool(k *prep, in *kop.Instr, p uint8) (pool slotMask) {
	src := s.objRef(k, kop.RefSelf)
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if sl := m.first(); s.matchPermGen(k, in.F, sl, s.res.e.ctrl, src, layerFull) {
			pool = pool.with(sl)
		}
	}
	return pool
}

// pilesStart reads the pool and poses the first ask; true: nothing to do.
func (s *State) pilesStart(k *prep, in *kop.Instr) bool {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return true
	}
	pool := s.pilesPool(k, in, p)
	if !pool.any() {
		return true // gorge returns on an empty set: no ask
	}
	s.g.pickP, s.g.sacM = p, slotMask{}
	s.res.ask, s.res.askP = raPileSplitSac, s.res.e.ctrl
	if !pool.many() {
		s.g.sacM = pool
		s.res.ask, s.res.askP = raPileChooseSac, p
	}
	return false
}

// askOptionsPiles builds the split and pick asks (the tail of
// askOptionsStdmech).
//
//go:noinline
func (s *State) askOptionsPiles(k *prep, d *Decision) bool {
	switch s.res.ask {
	case raPileSplitSac:
		d.Sub = ChGenPileSplit
		in := &s.prog(k)[s.res.pc]
		for m := s.pilesPool(k, in, s.g.pickP).andNot(s.g.sacM); m.any(); m = m.rest() {
			d.add(MakeAction(AkChoose, TCode(s.perms[m.first()].obj), 0, 0))
		}
		d.add(MakeAction(AkDone, 0, 0, 0))
	case raPileChooseSac:
		d.Sub = ChGenPileChoose
		d.add(MakeAction(AkChoose, 0, 0, 0))
		d.add(MakeAction(AkChoose, 0, 0, 1))
	default:
		return s.askOptionsWardDisc(k, d) // kw_wardcost.go
	}
	return true
}

// applyAskPiles takes a split pick (or Done), then the pile pick, which
// sacrifices the chosen pile.
//
//go:noinline
func (s *State) applyAskPiles(k *prep, a Action) bool {
	r := &s.res
	switch r.ask {
	case raPileSplitSac:
		in := &s.prog(k)[r.pc]
		pool := s.pilesPool(k, in, s.g.pickP)
		if a.Kind() == AkChoose {
			s.g.sacM = s.g.sacM.with(s.objs[Obj(a.Source())].slot)
			if pool.andNot(s.g.sacM).any() {
				return true // gorge's Max is the pool: picking it all ends the split
			}
		}
		r.ask, r.askP = raPileChooseSac, s.g.pickP
	case raPileChooseSac:
		r.ask = raNone
		in := &s.prog(k)[r.pc]
		hit := s.g.sacM
		if a.Mode() == 1 {
			hit = s.pilesPool(k, in, s.g.pickP).andNot(hit)
		}
		s.g.sacM, s.g.pickP = slotMask{}, 0
		if hit.any() {
			s.fireSacAll(k, hit)
			s.leaveAll(k, hit, kop.ZoneGraveyard)
		}
		s.continueRes(k, 0)
	default:
		return s.applyAskWardDisc(k, a) // kw_wardcost.go
	}
	return true
}
