package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The per-source Remembered of a CardDef.SrcMem card (zone_objsets.go's osRem
// records; gorge's Object.Remembered, which the writers in effects call
// eventRemember on and effCleanup clears).
//
//   - the write: remember(o) records o on the resolving source while it is the
//     battlefield incarnation that started the resolution;
//   - the clear: OpClearRemembered drops the source's records the resolution can
//     read;
//   - the seed: a leaves trigger of CardDef.SmSeed starts with the records of
//     the incarnation it fired from, in store order;
//   - the filter: a source's ValidCard$ IsRemembered is the union of the
//     resolution's set and the source's live records (gorge anyRemembered).

// srcMemCard reports whether the card at table index c keeps its Remembered.
func (s *State) srcMemCard(k *prep, c CardID) bool {
	return k != nil && k.gen != nil && k.gen.anySrcMem && int(c) < len(k.t.Cards) && k.t.Cards[c].SrcMem
}

// srcMemRemember records o on the resolving source's list.
func (s *State) srcMemRemember(o Obj) {
	k := s.k()
	e := &s.res.e
	if !s.srcMemCard(k, e.card) || e.obj == NoObj {
		return
	}
	if ob := &s.objs[e.obj]; ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer {
		return
	}
	s.osAdd(osRem, e.obj, o)
}

// srcMemClear empties the resolving source's records its resolution can read.
func (s *State) srcMemClear() {
	k := s.k()
	e := &s.res.e
	if s.srcMemCard(k, e.card) && e.obj != NoObj {
		s.osClear(e.obj, osMRem, e)
	}
}

// srcMemSeed starts a SmSeed trigger's resolution with its source's kept set.
func (s *State) srcMemSeed(k *prep, e *stackEntry) {
	if e.kind != seTrigger || !s.srcMemCard(k, e.card) || e.ab >= 32 || k.t.Cards[e.card].SmSeed>>e.ab&1 == 0 {
		return
	}
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			break
		}
		rec := &r[i]
		if rec.src != e.obj || !rec.inKinds(osMRem) || !s.osLiveFor(rec, e) || s.inSet(&s.g.rem, s.g.nrem, rec.o) {
			continue
		}
		if s.g.nrem >= maxRem {
			s.capFull(StallRemember)
			return
		}
		s.g.rem[s.g.nrem] = rec.o
		s.g.nrem++
	}
	if st := DebugSrcMemStats; st != nil {
		st.Seeded++
	}
}

// srcMemStoreHas reports whether src's live records hold o.
func (s *State) srcMemStoreHas(src, o Obj) bool {
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			break
		}
		rec := &r[i]
		if rec.src == src && rec.o == o && rec.inKinds(osMRem) && s.osLiveNow(rec) {
			return true
		}
	}
	return false
}

// flags2ObjSrc is flags2Obj for a filter read by source src (NoObj: the
// resolving entry's source). When that source keeps its Remembered, the
// Remembered tests read the source's live records, united with the
// resolution's set when the source is the one resolving.
func (s *State) flags2ObjSrc(k *prep, f kop.FilterFlags2, o, src Obj) bool {
	if f&(kop.F2Remembered|kop.F2NotRemembered) != 0 && k.gen != nil && k.gen.anySrcMem {
		if src == NoObj && s.res.active {
			src = s.res.e.obj
		}
		if src != NoObj && s.srcMemCard(k, s.objs[src].card) {
			// the resolution's set is the resolving source's own: another
			// source's trigger filter reads its records alone (gorge's
			// trigger context holds its fire-time capture, not this set)
			in := s.res.active && s.res.e.obj == src && s.inSet(&s.g.rem, s.g.nrem, o)
			st := DebugSrcMemStats
			if st != nil {
				st.Reads++
			}
			if !in {
				in = s.srcMemStoreHas(src, o)
			}
			if st != nil && in {
				st.Matched++
			}
			if f&kop.F2Remembered != 0 && !in || f&kop.F2NotRemembered != 0 && in {
				return false
			}
			f &^= kop.F2Remembered | kop.F2NotRemembered
		}
	}
	return s.flags2Obj(f, o)
}

// flags2ObjCtx is flags2ObjSrc for a card matcher read in ctx (nil: no source).
func (s *State) flags2ObjCtx(k *prep, f kop.FilterFlags2, o Obj, ctx *evalCtx) bool {
	if ctx == nil {
		return s.flags2ObjSrc(k, f, o, NoObj)
	}
	return s.flags2ObjSrc(k, f, o, ctx.src)
}
