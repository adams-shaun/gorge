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
		tc, t, ok := s.replTokensCard(k, p, CardID(in.C), 1) // a rewriter's card (repl_tokrewrite.go)
		if !ok {
			return true
		}
		for i := 0; i < t; i++ {
			if s.nearCapacity(k) {
				s.stall(StallBoard)
				return true
			}
			o := s.newToken(tc, p)
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

// grantHostGen returns the permanent whose continuous or override static grants sl
// the ability (c, ab) (a global "planeswalkers you control have ..." grant, Ygra's
// Food ability), or NoSlot. The attached-source case is grantedHost; this is the
// cold path the FRA Ways and Avatar of Burgeoning Echoes need.
//
// Of two copies, the older: globalGrantsGen lists the grants in layer
// timestamp order (gorge's active() order), and the offer collapses the
// second copy's identical ability into the first's option, so the host must
// be the copy gorge's first grant names. Slot order is a slot-allocation
// artifact and can put the newer copy first.
func (s *State) grantHostGen(k *prep, sl Slot, c CardID, ab uint8) Slot {
	host := NoSlot
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.cardAt(src) != c {
			continue
		}
		cd := &k.t.Cards[c]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || !staticGrants(k.t, &a.Static, ab) {
				continue
			}
			if s.matchPermGen(k, a.Static.Filter, sl, s.perms[src].ctrl, src, layerKw) {
				if host == NoSlot || s.layerTs(src) < s.layerTs(host) {
					host = src
				}
				break
			}
		}
	}
	return host
}

// embGrantHostObj is the object of the emblem whose Continuous static grants
// sl the ability (c, ab): gorge registers an emblem's grant under its
// creator's object (the effect's Source, CR 114.4 -- the emblem itself has no
// object) and offers the activation against it wherever the creator now is.
// It applies the gates globalGrantsGen's emblem pass applies to the same
// statics (a player out of the game, an attached-only filter, the static's
// condition), so the two never disagree on whether an emblem grants.
func (s *State) embGrantHostObj(k *prep, sl Slot, c CardID, ab uint8) (Obj, bool) {
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			for j := range e.Abilities {
				a := &e.Abilities[j]
				if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Grant != ab+1 ||
					filterAttachedOnly(k.t, a.Static.Filter) {
					continue
				}
				if s.embCard(pp, i) != c {
					continue
				}
				st := statSrc{a: a, sl: NoSlot, ctrl: pp, obj: NoObj, emb: pp<<4 | uint8(i+1)}
				if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, pp, NoSlot, layerKw) {
					continue
				}
				return s.embObj(pp, i), true
			}
		}
	}
	return NoObj, false
}

// staticGrants reports whether st grants its card's ability ab.
func staticGrants(t *kop.Table, st *kop.Static, ab uint8) bool {
	switch st.Kind {
	case kop.StatContinuous:
		return st.Grant == ab+1
	case kop.StatOverride:
		if st.Override == 0 {
			return false
		}
		g := t.Overrides[st.Override-1].Grant
		return g[0] == ab+1 || g[1] == ab+1
	}
	return false
}

// prepEmbGrant marks the grant walk for a Continuous static on an emblem the
// card creates (OpCreateEmblem): the emblem is not a permanent, so its grant
// is not statM's, it is globalGrantsGen's emblem pass, and the granted
// ability is one of the creator card's (the emblem's Grant indexes it).
func (g *genPrep) prepEmbGrant(t *kop.Table, c CardID, e *kop.Emblem) {
	cd := &t.Cards[c]
	for i := range e.Abilities {
		a := &e.Abilities[i]
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Grant == 0 ||
			filterAttachedOnly(t, a.Static.Filter) {
			continue
		}
		g.anyEmbGrant = true
		if int(a.Static.Grant) <= len(cd.Abilities) && cd.Abilities[a.Static.Grant-1].Kind != kop.AbMana {
			g.anyGrant, g.globGrant = true, true
			if cd.Abilities[a.Static.Grant-1].Kind == kop.AbTriggered {
				g.globTrigGrant = true
			}
		}
	}
}

// globalGrantsGen is globalGrants for a Continuous static whose Grant is a
// non-mana ability (the FRA Ways' and Avatar of Burgeoning Echoes' "planes-
// walkers you control have ..."): the override path is globalGrants, this
// one is the continuous path. Attached-only filters are left to the attached
// walk in grantedAbilitiesN.
func (s *State) globalGrantsGen(k *prep, sl Slot, lost bool, lts uint16, f func(c CardID, ab uint8, n int, ts uint16)) {
	var seen [16]struct {
		src Slot
		ab  uint8
	}
	nseen := 0
	// the sources in layer timestamp order (gorge lists active statics by
	// timestamp; grantedAbilitiesN interleaves the attached grants by it)
	for m := s.statM; m.any(); {
		src := m.first()
		for r := m.rest(); r.any(); r = r.rest() {
			if c := r.first(); s.layerTs(c) < s.layerTs(src) {
				src = c
			}
		}
		m = m.without(src)
		cid := s.cardAt(src)
		if !k.gen.has(cid, gfGlobGrant) || lost && s.layerTs(src) < lts {
			continue
		}
		srcLost := s.genLost(k, src)
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Grant == 0 ||
				filterAttachedOnly(k.t, a.Static.Filter) || srcLost && !keptOnLoss(k.t, a) {
				continue
			}
			if cd.Abilities[a.Static.Grant-1].Kind == kop.AbMana {
				// a granted mana ability is contGrants' (gfContGrant): listed
				// here too, a board with any other global grant gave every
				// grantee the ability twice, and the prefix search asked
				// which of the two to activate
				continue
			}
			ab := a.Static.Grant - 1
			dup := false
			for j := 0; j < nseen; j++ {
				if seen[j].src == src && seen[j].ab == ab {
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
				seen[nseen].src, seen[nseen].ab = src, ab
				nseen++
			}
			f(cid, ab, 1, s.layerTs(src))
		}
	}
	// the emblems' Continuous grants: an emblem is not a permanent, it is
	// never in statM, so its statics are read off the held emblems (the layer-6
	// emblem pass reads them the same way, static_layer6.go). The stamps are
	// the emblems' creation timestamps, sorted before the emission so the g
	// closure's grantsBetween windows go forward; no table carries both an
	// emblem grant and another global static grant, so the pass never has to
	// interleave with the statM walk's order.
	if k.gen.anyEmbGrant {
		// one entry per emblem the state can hold (maxEmbTotal per seat): every
		// corpus grant emblem carries one Continuous grant, so only an emblem
		// with several matching grants could fill it, and that stalls loudly
		// rather than drop a grant (StallGrants)
		var embs [MaxSeats * maxEmbTotal]struct {
			cid CardID
			ab  uint8
			ts  uint16
		}
		n := 0
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			for i := 0; i < int(s.g.nemb[pp]); i++ {
				e := &k.t.Emblems[s.embIdx(pp, i)-1]
				for j := range e.Abilities {
					a := &e.Abilities[j]
					if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Grant == 0 ||
						filterAttachedOnly(k.t, a.Static.Filter) {
						continue
					}
					cid := s.embCard(pp, i)
					ab := a.Static.Grant - 1
					if k.t.Cards[cid].Abilities[ab].Kind == kop.AbMana {
						// a granted mana ability is contGrants' (gfContGrant)
						continue
					}
					st := statSrc{a: a, sl: NoSlot, ctrl: pp, obj: NoObj, emb: pp<<4 | uint8(i+1)}
					if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, pp, NoSlot, layerKw) {
						continue
					}
					ts := s.embStamp(pp, i)
					if lost && ts < lts {
						continue
					}
					if n == len(embs) {
						s.capFull(StallGrants)
						continue
					}
					embs[n].cid, embs[n].ab, embs[n].ts = cid, ab, ts
					n++
				}
			}
		}
		for i := 1; i < n; i++ {
			for j := i; j > 0 && embs[j].ts < embs[j-1].ts; j-- {
				embs[j], embs[j-1] = embs[j-1], embs[j]
			}
		}
		for i := 0; i < n; i++ {
			f(embs[i].cid, embs[i].ab, 1, embs[i].ts)
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
