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
// gorge's Object.Imprinted: each write is also recorded in the object-set
// store (zone_objsets.go), the next resolution of that source starts with the
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
// only in exile or on the battlefield, a token in any zone. origin is the
// event-time zone when a matcher names one (a leave-exile trigger reads the
// card as it was in exile, CR 603.10a); ZoneNone reads the live zone.
func (s *State) impZoneGate(o Obj, origin kop.Zone) bool {
	ob := &s.objs[o]
	if origin != kop.ZoneNone {
		return ob.of&ofToken != 0 || origin == kop.ZoneExile || origin == kop.ZoneBattlefield
	}
	return ob.of&ofToken != 0 || ob.zone == kop.ZoneExile || ob.zone == kop.ZoneBattlefield
}

// impStoreHas reports whether o is one of src's kept imprints a reader can
// still see: the records live now, or of the incarnation a resolving trigger
// fired from (a leaves trigger reads the list after the departure). origin is
// the matcher's event-time zone (ZoneNone: the live one).
func (s *State) impStoreHas(src, o Obj, origin kop.Zone) bool {
	var e *stackEntry
	if s.res.active {
		e = &s.res.e
	}
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			break
		}
		rec := &r[i]
		if rec.src != src || !rec.inKinds(osMImp) || !s.osLiveFor(rec, e) {
			continue
		}
		if rec.o == o && s.impZoneGate(o, origin) {
			return true
		}
	}
	return false
}

// impHasAt reports whether o is in the imprinted list of src: the resolving
// resolution's set when src is the resolving entry, else the source's kept
// records (a source without ImpPersist reads the resolution's set as before).
func (s *State) impHasAt(k *prep, src, o Obj, origin kop.Zone) bool {
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
		ok = s.impHas(o) && s.impZoneGate(o, origin)
	} else {
		ok = s.impStoreHas(src, o, origin)
	}
	if st != nil && ok {
		st.Matched++
	}
	return ok
}

// impRefSrc reports whether a RefImprinted value read names an expression
// source that keeps its Imprinted list and is not the resolving entry: the
// read then answers from the store (impStoreEach), as gorge's Defined$
// Imprinted does from the source's own list. A read outside the resolution's
// own context (a static's, ctx.e nil) while another ability of the source
// resolves answers from the store too, unless that resolution has imprinted
// (gorge's list already holds what it exiled): Phyrexian Ingester's +X/+Y
// while a granted upkeep trigger of it resolves (Vile Consumption).
func (s *State) impRefSrc(ctx evalCtx) (Obj, bool) {
	src := ctx.src
	if src == NoObj || !s.impSource(s.k(), src) {
		return NoObj, false
	}
	if s.res.active && s.res.e.obj == src && (ctx.e == &s.res.e || s.g.nimp > 0) {
		return NoObj, false
	}
	return src, true
}

// impStoreEach calls f for each of src's kept imprints, in store order, with
// no zone gate (gorge's Defined$ Imprinted): the records live now (the
// source on the battlefield at the stamped ver).
func (s *State) impStoreEach(src Obj, f func(Obj)) {
	s.osEach(osMImp, src, nil, f)
}

// impCreatureSub is kop.Override ImpSub (Duplicant's AddType$
// ImprintedCreatureType): the printed creature types of the last card in
// src's kept imprints whose printed face is a creature and that is still in
// exile (gorge layers.go reads o.Imprinted from its end, the live object's
// zone the CR 607.2a exile test). The records live now; the list order is
// the store's insertion order.
func (s *State) impCreatureSub(k *prep, src Obj) kop.SubMask {
	var out kop.SubMask
	if src == NoObj {
		return out
	}
	r := s.cold.objSets
	for i := s.osLen() - 1; i >= 0; i-- {
		rec := &r[i]
		if rec.src != src || !rec.inKinds(osMImp) || !s.osLiveNow(rec) {
			continue
		}
		ob := &s.objs[rec.o]
		if ob.zone != kop.ZoneExile {
			continue
		}
		c := &k.t.Cards[ob.card]
		if c.Types&kop.TCreature == 0 {
			continue
		}
		subs := c.Subtypes
		if c.AllCreatureTypes {
			subs = c.PrintedSubtypes // a changeling's printed face names its type word alone (gorge CreatureTypeWords over Face().Types)
		}
		out = out.Or(k.gen.creatSub.And(subs))
		break
	}
	return out
}

// imprintExt is execExt's shared ExtClearImprinted/ExtForgetImprinted arm:
// clear the whole kept list, or forget the one object the op's ref names.
func (s *State) imprintExt(k *prep, in *kop.Instr) {
	if kop.ExtOp(in.A.N) == kop.ExtForgetImprinted {
		s.forgetImprinted(k, in)
		return
	}
	s.extClearImprinted(k)
}

// forgetImprinted is ExtForgetImprinted: the object the op's Obj ref names
// (RefTriggered, gorge Pump ForgetImprinted$ TriggeredCard) leaves the
// resolution's imprinted set, keeping order; for a card that keeps its list
// the store record goes too (gorge forgetImprinted on Pump).
func (s *State) forgetImprinted(k *prep, in *kop.Instr) {
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return
	}
	n := uint8(0)
	for i := 0; i < int(s.g.nimp); i++ {
		if s.g.imp[i] == o {
			continue
		}
		s.g.imp[n] = s.g.imp[i]
		n++
	}
	for j := int(n); j < int(s.g.nimp); j++ {
		s.g.imp[j] = NoObj
	}
	s.g.nimp = n
	e := &s.res.e
	if k.gen != nil && k.gen.anySrcMem && int(e.card) < len(k.t.Cards) && k.t.Cards[e.card].ImpPersist {
		s.osForget(o, e.obj, osMImp)
	}
}

// extClearImprinted empties the resolution's set and, for a source that
// keeps its list, the source's live records it can read.
func (s *State) extClearImprinted(k *prep) {
	s.g.nimp = 0
	if k.gen.anySrcMem && int(s.res.e.card) < len(k.t.Cards) && k.t.Cards[s.res.e.card].ImpPersist {
		s.osClear(s.res.e.obj, osMImp, &s.res.e)
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
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			break
		}
		rec := &r[i]
		if rec.src != e.obj || !rec.inKinds(osMImp) || !s.osLiveFor(rec, e) {
			continue
		}
		if s.g.nimp >= maxImp {
			s.capFull(StallImprint)
			return
		}
		s.g.imp[s.g.nimp] = rec.o
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
	s.osDropImp(o)
}

// imprintStore records o on the resolving ImpPersist source, if it is still
// the battlefield incarnation that started the resolution. osImpAny marks a
// token, osImp any other object.
func (s *State) imprintStore(o Obj, e *stackEntry) {
	k := s.k()
	if k == nil || k.gen == nil || !k.gen.anySrcMem || e == nil || int(e.card) >= len(k.t.Cards) || !k.t.Cards[e.card].ImpPersist {
		return
	}
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer {
		return
	}
	kind := osImp
	if s.objs[o].of&ofToken != 0 {
		kind = osImpAny
	}
	s.osAdd(kind, e.obj, o)
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
