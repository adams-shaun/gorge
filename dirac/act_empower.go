package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// api:Empower (CR 701.71a, gorge effEmpower). Cold, out of line.

// DebugEmpowerAlwaysMint makes empower ignore an existing Jace token and
// always mint (the lockstep mutation test): gorge reuses the candidate.
var DebugEmpowerAlwaysMint bool

// prepEmpower sets anyEmpower: some program is OpEmpower. FDN and BLB have
// none, so the op dispatch is never taken there.
func prepEmpower(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				g.anyEmpower = g.anyEmpower || in.Op == kop.OpEmpower
			}
		}
	}
}

// empowerSub is the subtype Table.Overrides[in.B.N&0xfff-1] grants (the
// Type$ word; kgen only lowers Type$ Jace, but the kernel never names it).
func (s *State) empowerSub(k *prep, in *kop.Instr) int {
	ov := int(uint16(in.B.N) & 0xfff)
	if ov == 0 || ov > len(k.t.Overrides) {
		return -1
	}
	m := k.t.Overrides[ov-1].AddSub
	for i := 0; i < kop.MaxSubtypes; i++ {
		if m.Has(i) {
			return i
		}
	}
	return -1
}

// empowerCands writes the battlefield tokens player p controls that have
// subtype sub, in battlefield order (gorge empowerCandidates: the
// layer-aware type test, so a non-token Jace never qualifies).
func (s *State) empowerCands(k *prep, p uint8, sub int, out []Slot) int {
	if sub < 0 {
		return 0
	}
	var ord [MaxPerm]Slot
	m := s.battlefieldOrder(p, s.ctrlM[p], ord[:], 0)
	n := 0
	for _, sl := range ord[:m] {
		if s.objs[s.perms[sl].obj].of&ofToken == 0 {
			continue
		}
		if !s.hasSubGen(k, sl, uint8(sub)) {
			continue
		}
		out[n] = sl
		n++
	}
	return n
}

// empower is OpEmpower (gorge effEmpower): player Obj puts A loyalty
// counters on one Jace token they control. N is read before any token is
// created. With no candidate the u_empower_jace token is created (even when N
// <= 0), each mint gets the Type$ / "<Type$> Token" override, and the
// counters go on the minted token; two or more mints (a token doubler) ask.
// false: suspended on the pick.
//
//go:noinline
func (s *State) empower(k *prep, in *kop.Instr) bool {
	n := s.value(k, in.A)
	if n < 0 {
		n = 0
	}
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		p = s.res.e.ctrl
	}
	sub := s.empowerSub(k, in)
	ov := int(uint16(in.B.N) & 0xfff)
	var cands [MaxPerm]Slot
	nc := s.empowerCands(k, p, sub, cands[:])
	if DebugEmpowerAlwaysMint {
		nc = 0
	}
	if nc == 0 {
		if ov == 0 {
			return true
		}
		t := s.replTokens(k, p, CardID(in.C), 1)
		for i := 0; i < t; i++ {
			if s.nearCapacity() {
				s.stall(StallBoard)
				return true
			}
			o := s.newToken(CardID(in.C), p)
			sl := s.enter(k, o, p, false, kop.ZoneLibrary)
			if k.gen.anyTokCr {
				s.fireTokenCreated(k, sl)
			}
			s.res.lastTok = o
			s.applyOverride(k, sl, ov-1, s.cardAt(sl), false)
			if nc == len(cands) {
				s.stall(StallBoard)
				return true
			}
			cands[nc] = sl
			nc++
		}
	}
	if n <= 0 || nc == 0 {
		return true
	}
	if nc == 1 {
		s.bolsterPut(k, cands[0], int(kop.CtrLoyalty), n)
		return true
	}
	s.res.ask, s.res.askP = raGenEmpower, p
	return false
}

// grantHostGen returns the permanent whose continuous static grants sl the
// ability (c, ab) (a global "planeswalkers you control have ..." grant), or
// NoSlot. The attached-source case is grantedHost; this is the cold path the
// FRA Ways and Avatar of Burgeoning Echoes need.
func (s *State) grantHostGen(k *prep, sl Slot, c CardID, ab uint8) Slot {
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.cardAt(src) != c {
			continue
		}
		cd := &k.t.Cards[c]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Grant == ab+1 &&
				s.matchPermGen(k, a.Static.Filter, sl, s.perms[src].ctrl, src, layerKw) {
				return src
			}
		}
	}
	return NoSlot
}

// globalGrantsGen is globalGrants for a Continuous static whose Grant is a
// non-mana ability (the FRA Ways' and Avatar of Burgeoning Echoes' "planes-
// walkers you control have ..."): the override path is globalGrants, this
// one is the continuous path. Attached-only filters are left to the attached
// walk in grantedAbilitiesN.
func (s *State) globalGrantsGen(k *prep, sl Slot, lost bool, lts uint16, f func(c CardID, ab uint8, n int, ts uint16)) {
	var seen [16]struct {
		c  CardID
		ab uint8
	}
	nseen := 0
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		cid := s.cardAt(src)
		if !k.gen.has(cid, gfGlobGrant) || s.genLost(k, src) || lost && s.layerTs(src) < lts {
			continue
		}
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Grant == 0 ||
				filterAttachedOnly(k.t, a.Static.Filter) {
				continue
			}
			ab := a.Static.Grant - 1
			dup := false
			for j := 0; j < nseen; j++ {
				if seen[j].c == cid && seen[j].ab == ab {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			st := statSrc{a: a, sl: src, ctrl: s.perms[src].ctrl, obj: s.perms[src].obj}
			if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, src, layerKw) {
				continue
			}
			if nseen < len(seen) {
				seen[nseen].c, seen[nseen].ab = cid, ab
				nseen++
			}
			f(cid, ab, 1, s.layerTs(src))
		}
	}
}

// excessLethalGen is the damage that kills sl, for the excess-damage count:
// a non-creature planeswalker loses loyalty counters, everything else
// toughness minus damage already marked.
func (s *State) excessLethalGen(k *prep, sl Slot) int {
	if s.typesAt(k, sl)&kop.TPlaneswalker != 0 && !s.isCreature(k, sl) {
		return s.perms[sl].ctrLow(int(kop.CtrLoyalty))
	}
	_, t := s.ptGen(k, sl)
	return t - int(s.perms[sl].damage)
}
