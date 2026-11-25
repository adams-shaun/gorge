package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The one cold object-set store: "object o is in source src's list of kind
// K". The per-source memory (CR 400.7: a source that changes zone is a new
// object and what it remembered or imprinted does not follow it; gorge keeps
// both lists on the source object, Object.Remembered and Object.Imprinted,
// and a card's later ability, trigger or static reads what an earlier one
// wrote) and the exiled-with links (CR 406.6, 607.2a; zone_exiledwith.go)
// were two parallel cold stores of the same shape; they share one region,
// coldObjSets, and one API, so the next per-source set (osNoted, osNotedFor,
// osChosen, osPaired, osHaunted, osChampioned; unused until T10) writes the
// same store. A feature's readers keep their own semantics on top of the
// primitives; no LIR change.
//
// The store is present only when a card keeps its lists (CardDef.ImpPersist
// or CardDef.SrcMem, genPrep.anySrcMem) or reads the link (CardDef.ExiledWith,
// genPrep.anyExW, or a Dig's face-down exile, genPrep.anyFaceExile); a table
// without one has an empty region that is simply full.
//
// The rules mirror gorge's and the RemPersist links' (zone_rememberpersist.go):
//
//   - the departure: a source that leaves the battlefield KEEPS its Imprinted
//     list (gorge's wasBattlefield arm keeps Object.Imprinted for the leaves
//     trigger that reads it), so leaveGen no longer folds it;
//   - the re-entry wipe: a source entering the battlefield starts with empty
//     lists (osDropImp at the enter, by the card that keeps them; zone_imprint.go
//     impEnter);
//   - the fire-time snapshot: a record is live while its source is on the
//     battlefield at the stamped ver, and a resolving trigger also reads the
//     records of the incarnation it fired from (srcVer == sver, or sver+1 for
//     a leaves trigger queued after the departure) after that source has left
//     or been replaced (osLiveFor). A clear by one incarnation therefore
//     leaves an older incarnation's records for the pending trigger that holds
//     them, and osGC frees a record only when no stack entry or pending
//     trigger can still read it. The ability arm (seAbility with srcVer,
//     legal_gen.go) is not a test device only: a resolution-time impSeed
//     (zone_imprint.go, resolve.go) reads the stamped incarnation's records
//     through it while the ability resolves.
//
// The exiled-with records' own live-incarnation rule (a forward list that
// outlives a departed source, a reverse link live at the same ver or a
// resolving trigger's fire ver) is zone_exiledwith.go's; its readers walk the
// same store through the osExw kind and its flags.
//
// A full store is StallObjSets. The records are in insertion order, a live
// prefix that ends at the first empty one (osLen), so State needs no count;
// the order within one (kind, src) set is what the Remembered and Imprinted
// readers walk. osGC frees only records no reader can reach, so the two full
// paths (a set add, an exwPut) running both of its arms changes nothing.

// osRec is one record: o is in src's set of kind, stamped sver. kind's low
// nibble is the set kind (osKindMask), its high nibble the exiled-with flags
// of an osExw record (osFlagMask). kind 0 is an empty slot.
type osRec struct {
	o, src Obj
	sver   uint8
	kind   uint8
}

// Record kinds, numbered so a reader's mask is 1<<kind (osMRem and friends;
// the high nibble would collide with a fourth flag bit). osNoted and the
// kinds after it are T10's, unused here.
const (
	osRem        uint8 = iota + 1 // the source's Remembered list (zone_srcmemrem.go)
	osImp                         // the source's Imprinted list (zone_imprint.go)
	osImpAny                      // the source's Imprinted list, for any object (not only a card)
	osExw                         // an exiled-with link (zone_exiledwith.go); its flags ride kind's high nibble
	osNoted                       // (T10) a Noted$ set
	osNotedFor                    // (T10) a NotedFor$ set
	osChosen                      // (T10) a ChosenCard set
	osPaired                      // (T10) a soulbond pair
	osHaunted                     // (T10) a HauntedBy set
	osChampioned                  // (T10) a ChampionedBy set
	osNum                         // a StoreSVar number (act_storenum.go): o the value as an int16, kind's high nibble the SVar name
	osEncoded                     // a Cipher card o exiled encoded on the creature src (kw_cipher.go); live while src is at sver and o in exile
)

// The kind and flag nibbles of osRec.kind.
const (
	osKindMask uint8 = 0x0f
	osFlagMask uint8 = 0xf0
)

// osKind is the record's set kind.
func (r *osRec) osKind() uint8 { return r.kind & osKindMask }

// osFlags is the record's exiled-with flags (0 for a set-kind record).
func (r *osRec) osFlags() uint8 { return r.kind & osFlagMask }

// inKinds reports whether the record's kind is in mask.
func (r *osRec) inKinds(mask uint16) bool { return mask&(1<<r.osKind()) != 0 }

// The kinds masks (1<<kind) the readers pass.
const (
	osMRem uint16 = 1 << osRem
	osMImp uint16 = 1<<osImp | 1<<osImpAny
	osMExw uint16 = 1 << osExw
	osMAll uint16 = (1 << (osChampioned + 1)) - 2 // every kind (T10's readers); osNum is no set kind and is never in a mask
)

// The store's capacity depends on what the table keeps. A table whose cards
// keep a set (anySrcMem) AND read or record the exiled-with link (anyExW or
// anyFaceExile) had both old stores, 64 records each (768 B); it gets
// maxObjSets, 96 (576 B), against a measured joint peak of 26. A table with
// only one of them had one 64-record store (384 B) and keeps that size,
// maxObjSetsOne, so no pool grows. A table with neither has an empty region
// that is simply full.
const (
	maxObjSets    = 96
	maxObjSetsOne = 64
)

// objSetsCap is the record capacity for a table.
func objSetsCap(g *genPrep) int {
	link := g.anyExW || g.anyFaceExile
	switch {
	case g.uses.has(useStoreNum):
		return maxObjSets // the number store's records beside whatever sets the table keeps (act_storenum.go)
	case g.anySrcMem && link:
		return maxObjSets
	case g.anySrcMem || link || g.anyCipher:
		return maxObjSetsOne // (anyCipher: the encode links, kw_cipher.go)
	}
	return 0
}

// StallObjSets is a live record past the table's capacity (StallSrcMem's
// number, renumbered never; 64 or 96 records, see objSetsCap).
const StallObjSets = 173

// coldObjSets is the records, for a table whose cards keep a set or read the
// link.
var coldObjSets = newCold[osRec]("objSets", objSetsCap).view(func(c *coldArrays) *[]osRec { return &c.objSets })

// DebugSrcMemNoRem, DebugSrcMemNoImp and DebugSrcMemNoForget are the mutation
// switches: osAdd skips that kind, osForget does nothing.
var (
	DebugSrcMemNoRem    bool
	DebugSrcMemNoImp    bool
	DebugSrcMemNoForget bool
)

// SrcMemStats counts the store's lockstep coverage.
type SrcMemStats struct {
	Recorded int // records written
	Reads    int // reads of a list
	Matched  int // reads that found a record
	Cleared  int // records dropped by osClear
	Forgot   int // records dropped by osForget and osDropImp
	GC       int // records freed by osGC
	Seeded   int // resolutions that started with their incarnation's kept set
}

// DebugSrcMemStats, when non-nil, accumulates the counters above.
var DebugSrcMemStats *SrcMemStats

// osLen is the live prefix: the records up to the first empty one.
func (s *State) osLen() int {
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			return i
		}
	}
	return len(r)
}

// osAdd records o in src's set of kind, stamped with src's current ver. An
// existing record of the key is left alone, whatever its stamp. A full store
// is collected first, then StallObjSets.
func (s *State) osAdd(kind uint8, src, o Obj) {
	s.osAddAt(kind, src, o, s.objs[src].ver)
}

// osAddAt is osAdd with the stamp given: a minted token's record is written
// before its entry, which bumps its ver (zones.go), so it carries ver+1
// (zone_tokrem.go).
func (s *State) osAddAt(kind uint8, src, o Obj, sver uint8) {
	if kind == osRem && DebugSrcMemNoRem || (kind == osImp || kind == osImpAny) && DebugSrcMemNoImp {
		return
	}
	r := s.cold.objSets
	n := 0
	for ; n < len(r) && r[n].kind != 0; n++ {
		if r[n].o == o && r[n].src == src && r[n].osKind() == kind {
			return
		}
	}
	if n >= len(r) {
		s.osGC()
		if n = s.osLen(); n >= len(r) {
			s.capFull(StallObjSets)
			return
		}
	}
	r[n] = osRec{o: o, src: src, sver: sver, kind: kind}
	if DebugSrcMemStats != nil {
		DebugSrcMemStats.Recorded++
	}
}

// osHas reports whether src's set kind holds o at stamp sver (the exact key
// osAdd dedupes on).
func (s *State) osHas(kind uint8, src, o Obj, sver uint8) bool {
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			return false
		}
		rec := &r[i]
		if rec.o == o && rec.src == src && rec.osKind() == kind {
			return rec.sver == sver
		}
	}
	return false
}

// osEach calls f with the object of each of src's live records in mask's
// kinds, in store order. e (nil: none) is the resolving entry osLiveFor reads;
// a nil e is the live-now rule. f gets the object, not the record: a pointer into the store handed to an unknown func makes the State
// leak to the heap through every caller (TestNoKernelLocalEscapes).
func (s *State) osEach(mask uint16, src Obj, e *stackEntry, f func(o Obj)) {
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			return
		}
		rec := &r[i]
		if rec.src != src || !rec.inKinds(mask) || !s.osLiveFor(rec, e) {
			continue
		}
		f(rec.o)
	}
}

// osAnyOf reports whether the store holds a live record of mask's kinds.
func (s *State) osAnyOf(mask uint16) bool {
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			return false
		}
		if r[i].inKinds(mask) {
			return true
		}
	}
	return false
}

// osLiveNow: the record's source is on the battlefield at the stamped ver.
func (s *State) osLiveNow(r *osRec) bool {
	ob := &s.objs[r.src]
	return ob.zone == kop.ZoneBattlefield && ob.ver == r.sver
}

// osLiveFor: the record is live now, or e is a trigger or an activated
// ability of the stamped incarnation (srcVer == sver, or sver+1 for a leaves
// trigger): gorge's source keeps its lists through the departure, so an
// ability already on the stack still reads them.
func (s *State) osLiveFor(r *osRec, e *stackEntry) bool {
	if s.osLiveNow(r) {
		return true
	}
	return e != nil && (e.kind == seTrigger || e.kind == seAbility) && e.obj == r.src && (r.sver == e.srcVer || r.sver+1 == e.srcVer)
}

// osDrop removes the records for which drop(r) holds, keeping insertion order
// and the live prefix, and returns how many it removed.
func (s *State) osDrop(drop func(r *osRec) bool) int {
	r := s.cold.objSets
	n, i := 0, 0
	for ; i < len(r) && r[i].kind != 0; i++ {
		if !drop(&r[i]) {
			r[n] = r[i]
			n++
		}
	}
	for j := n; j < i; j++ {
		r[j] = osRec{}
	}
	return i - n
}

// osClear drops src's records in mask's kinds that e (nil: none) can read.
func (s *State) osClear(src Obj, mask uint16, e *stackEntry) {
	n := s.osDrop(func(r *osRec) bool { return r.src == src && r.inKinds(mask) && s.osLiveFor(r, e) })
	if DebugSrcMemStats != nil {
		DebugSrcMemStats.Cleared += n
	}
}

// osForget drops the records of o in src's sets of mask.
func (s *State) osForget(o, src Obj, mask uint16) {
	if DebugSrcMemNoForget {
		return
	}
	n := s.osDrop(func(r *osRec) bool { return r.o == o && r.src == src && r.inKinds(mask) })
	if DebugSrcMemStats != nil {
		DebugSrcMemStats.Forgot += n
	}
}

// osDropImp ends src's Imprinted sets: the battlefield-entry wipe
// (zone_imprint.go impEnter), where every record is from an earlier
// incarnation.
func (s *State) osDropImp(src Obj) {
	n := s.osDrop(func(r *osRec) bool { return r.src == src && r.inKinds(osMImp) })
	if DebugSrcMemStats != nil {
		DebugSrcMemStats.Forgot += n
	}
}

// osGC frees the records no one can read: a set-kind record whose source is
// not live at the stamped ver and that no stack entry or pending trigger of
// it holds (srcVer in {sver, sver+1}), and an osExw record whose flags a
// clear has taken (unreadable; the exiled-with readers' own rules, not this
// GC's, decide when a flagged record is read).
func (s *State) osGC() {
	n := s.osDrop(func(r *osRec) bool {
		if r.osKind() == osExw {
			return r.osFlags() == 0
		}
		if r.osKind() == osNum {
			return false // a number lives until its object leaves the battlefield (numClear), never by a reader's reach
		}
		if s.osLiveNow(r) {
			return false
		}
		for i := 0; i < int(s.sp); i++ {
			if e := &s.stack[i]; e.obj == r.src && (e.srcVer == r.sver || e.srcVer == r.sver+1) {
				return false
			}
		}
		for i := 0; i < int(s.npend); i++ {
			if p := &s.pend[i]; p.obj == r.src && (p.srcVer == r.sver || p.srcVer == r.sver+1) {
				return false
			}
		}
		return true
	})
	if DebugSrcMemStats != nil {
		DebugSrcMemStats.GC += n
	}
}

func init() { stallNames[StallObjSets] = "objsets" }
