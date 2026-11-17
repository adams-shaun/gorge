package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The resolution's imprinted set (genState.imp): objects an Imprint$ writer
// (kop.MvImprint) set aside for a later instruction of the same resolution
// to read (kop.RefImprinted), emptied by kop.ExtClearImprinted and at the
// start of the next resolution. It is separate from the remembered set
// (gorge's Imprinted and Remembered are independent lists, and one object
// may be in both) and from the chosen set the older imprint shapes borrow.
//
// A card whose CardDef.ImpPersist is set keeps the list on the source, as
// gorge's Object.Imprinted: each write is also recorded in the per-source
// memory (zone_srcmem.go), the next resolution of that source starts with the
// records its stamped incarnation keeps (impSeed; a leaves trigger reads them
// after the departure), and a read away from the resolving resolution answers
// from them (impHasAt). The list is cleared at the source's next battlefield
// entry (impEnter), not on the departure.

// StallImprint: more than maxImp (gen.go) objects imprinted in one resolution.
const StallImprint = 145

func init() { stallNames[StallImprint] = "imprint" }

// impHas reports whether o is in the resolution's imprinted set.
func (s *State) impHas(o Obj) bool {
	for i := 0; i < int(s.g.nimp); i++ {
		if s.g.imp[i] == o {
			return true
		}
	}
	return false
}

// impSource reports whether src's card keeps its Imprinted on the source.
func (s *State) impSource(k *prep, src Obj) bool {
	if k == nil || k.gen == nil || !k.gen.anySrcMem || src == NoObj {
		return false
	}
	c := s.objs[src].card
	return int(c) < len(k.t.Cards) && k.t.Cards[c].ImpPersist
}

// impZoneGate is gorge's IsImprinted zone gate: a non-token candidate matches
// only in exile or on the battlefield, a token in any zone.
func (s *State) impZoneGate(o Obj) bool {
	ob := &s.objs[o]
	return ob.of&ofToken != 0 || ob.zone == kop.ZoneExile || ob.zone == kop.ZoneBattlefield
}

// impStoreHas reports whether o is one of src's kept imprints a reader can
// still see: the records live now, or of the incarnation a resolving trigger
// fired from (a leaves trigger reads the list after the departure).
func (s *State) impStoreHas(src, o Obj) bool {
	var e *stackEntry
	if s.res.active {
		e = &s.res.e
	}
	r := coldSrcMem.of(s)
	for i := range r {
		if r[i].fl == 0 {
			break
		}
		if r[i].src != src || r[i].fl&(smImp|smImpAny) == 0 || !s.smLiveFor(&r[i], e) {
			continue
		}
		if r[i].o == o && s.impZoneGate(o) {
			return true
		}
	}
	return false
}

// impHasAt reports whether o is in the imprinted list of src: the resolving
// resolution's set when src is the resolving entry, else the source's kept
// records (a source without ImpPersist reads the resolution's set as before).
func (s *State) impHasAt(k *prep, src, o Obj) bool {
	if src == NoObj {
		return s.impHas(o)
	}
	if !s.impSource(k, src) {
		return s.impHas(o)
	}
	st := DebugSrcMemStats
	if st != nil {
		st.Reads++
	}
	ok := false
	if s.res.active && s.res.e.obj == src {
		ok = s.impHas(o) && s.impZoneGate(o)
	} else {
		ok = s.impStoreHas(src, o)
	}
	if st != nil && ok {
		st.Matched++
	}
	return ok
}

// extClearImprinted empties the resolution's set and, for a source that
// keeps its list, the source's live records it can read.
func (s *State) extClearImprinted(k *prep) {
	s.g.nimp = 0
	if k.gen.anySrcMem && int(s.res.e.card) < len(k.t.Cards) && k.t.Cards[s.res.e.card].ImpPersist {
		s.smClear(s.res.e.obj, smImp|smImpAny, &s.res.e)
	}
}

// impSeedOnResolve seeds the resolving ImpPersist entry's kept list.
func (s *State) impSeedOnResolve(k *prep, e *stackEntry) {
	if k.gen.anySrcMem && int(e.card) < len(k.t.Cards) && k.t.Cards[e.card].ImpPersist {
		s.impSeed(e)
	}
}

// impSeed starts a resolution of an ImpPersist source with the records its
// stamped incarnation keeps, in store order. A leaves trigger (srcVer+1) also
// seeds: gorge keeps the list through the departure and clears it at the next
// battlefield entry.
func (s *State) impSeed(e *stackEntry) {
	r := coldSrcMem.of(s)
	for i := range r {
		if r[i].fl == 0 {
			break
		}
		if r[i].src != e.obj || r[i].fl&(smImp|smImpAny) == 0 || !s.smLiveFor(&r[i], e) {
			continue
		}
		if s.g.nimp >= maxImp {
			s.capFull(StallImprint)
			return
		}
		s.g.imp[s.g.nimp] = r[i].o
		s.g.nimp++
	}
	if st := DebugSrcMemStats; st != nil {
		st.Seeded++
	}
}

// impEnter is gorge's clear point for the kept Imprinted list: the next
// battlefield entry of the source (a departure keeps it for the leaves
// trigger that reads it). enterNF calls it after the new incarnation's ver,
// so every record of the source is from an earlier incarnation and goes.
func (s *State) impEnter(o Obj) {
	s.smDropImp(o)
}

// imprintStore records o on the resolving ImpPersist source, if it is still
// the battlefield incarnation that started the resolution. smImpAny marks a
// token, smImp any other object.
func (s *State) imprintStore(o Obj, e *stackEntry) {
	k := s.k()
	if k == nil || k.gen == nil || !k.gen.anySrcMem || e == nil || int(e.card) >= len(k.t.Cards) || !k.t.Cards[e.card].ImpPersist {
		return
	}
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer {
		return
	}
	kind := smImp
	if s.objs[o].of&ofToken != 0 {
		kind = smImpAny
	}
	s.smAdd(o, e.obj, kind)
}

// imprint adds o to the imprinted set, once, and records it on a resolving
// ImpPersist source.
func (s *State) imprint(o Obj) {
	for i := 0; i < int(s.g.nimp); i++ {
		if s.g.imp[i] == o {
			return
		}
	}
	if s.g.nimp >= maxImp {
		s.capFull(StallImprint)
		return
	}
	s.g.imp[s.g.nimp] = o
	s.g.nimp++
	if s.res.active {
		s.imprintStore(o, &s.res.e)
	}
}
