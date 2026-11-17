package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The per-source memory (CR 400.7: a source that changes zone is a new object
// and what it remembered or imprinted does not follow it). gorge keeps both
// lists on the source object, Object.Remembered and Object.Imprinted, and a
// card's later ability, trigger or static reads what an earlier one wrote.
// State's cold srcMem holds one record per (object, source, kind): the object
// o that source src remembered (smRem) or imprinted (smImp; smImpAny for an
// imprint of a card that is not a copy source's own, kept apart so a reader of
// either finds its own set), stamped with the source's ver when written. The
// table is present only when a card keeps its lists (CardDef.ImpPersist or
// CardDef.SrcMem, genPrep.anySrcMem); a table without one has an empty region
// that is simply full.
//
// The rules mirror gorge's and the RemPersist links' (zone_rememberpersist.go):
//
//   - the departure: a source that leaves the battlefield KEEPS its Imprinted
//     list (gorge's wasBattlefield arm keeps Object.Imprinted for the leaves
//     trigger that reads it), so leaveGen no longer folds it;
//   - the re-entry wipe: a source entering the battlefield starts with empty
//     lists (smDropImp at the enter, by the card that keeps them; zone_imprint.go
//     impEnter);
//   - the fire-time snapshot: a record is live while its source is on the
//     battlefield at the stamped ver, and a resolving trigger also reads the
//     records of the incarnation it fired from (srcVer == sver, or sver+1 for
//     a leaves trigger queued after the departure) after that source has left
//     or been replaced (smLiveFor). A clear by one incarnation therefore
//     leaves an older incarnation's records for the pending trigger that holds
//     them, and smGC frees a record only when no stack entry or pending
//     trigger can still read it.
//
// A full table is StallSrcMem. The records are in insertion order, a live
// prefix that ends at the first empty one (smLen), so State needs no count.

// Record kinds (smRec.fl bits).
const (
	smRem    uint8 = 1 << iota // the source's Remembered list
	smImp                      // the source's Imprinted list
	smImpAny                   // the source's Imprinted list, for any object (not only a card)
)

// maxSrcMem bounds the records of a table with a card that keeps them.
const maxSrcMem = 64

// StallSrcMem is a 65th live record of the per-source memory.
const StallSrcMem = 173

// coldSrcMem is the records, for a table whose cards keep their lists.
var coldSrcMem = newCold[smRec]("srcMem", func(g *genPrep) int {
	if g.anySrcMem {
		return maxSrcMem
	}
	return 0
})

// smRec is one record: o is in src's list of its kind, stamped sver.
type smRec struct {
	o, src Obj
	sver   uint8
	fl     uint8
}

// DebugSrcMemNoRem, DebugSrcMemNoImp and DebugSrcMemNoForget are the mutation
// switches: smAdd skips that kind, smForget does nothing.
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
	Cleared  int // records dropped by smClear
	Forgot   int // records dropped by smForget and smDropImp
	GC       int // records freed by smGC
	Seeded   int // resolutions that started with their incarnation's kept set
}

// DebugSrcMemStats, when non-nil, accumulates the counters above.
var DebugSrcMemStats *SrcMemStats

// smLen is the live prefix: the records up to the first empty one.
func (s *State) smLen() int {
	r := coldSrcMem.of(s)
	for i := range r {
		if r[i].fl == 0 {
			return i
		}
	}
	return len(r)
}

// smAdd records o in src's list of kind, stamped with src's current ver. An
// existing record of the key is left alone, whatever its stamp. A full table
// is collected first, then StallSrcMem.
func (s *State) smAdd(o, src Obj, kind uint8) {
	if kind&smRem != 0 && DebugSrcMemNoRem || kind&(smImp|smImpAny) != 0 && DebugSrcMemNoImp {
		return
	}
	r := coldSrcMem.of(s)
	n := 0
	for ; n < len(r) && r[n].fl != 0; n++ {
		if r[n].o == o && r[n].src == src && r[n].fl == kind {
			return
		}
	}
	if n >= len(r) {
		s.smGC()
		if n = s.smLen(); n >= len(r) {
			s.capFull(StallSrcMem)
			return
		}
	}
	r[n] = smRec{o: o, src: src, sver: s.objs[src].ver, fl: kind}
	if DebugSrcMemStats != nil {
		DebugSrcMemStats.Recorded++
	}
}

// smLiveNow: the record's source is on the battlefield at the stamped ver.
func (s *State) smLiveNow(r *smRec) bool {
	ob := &s.objs[r.src]
	return ob.zone == kop.ZoneBattlefield && ob.ver == r.sver
}

// smLiveFor: the record is live now, or e is a trigger of the stamped
// incarnation (srcVer == sver, or sver+1 for a leaves trigger).
func (s *State) smLiveFor(r *smRec, e *stackEntry) bool {
	if s.smLiveNow(r) {
		return true
	}
	return e != nil && e.kind == seTrigger && e.obj == r.src && (r.sver == e.srcVer || r.sver+1 == e.srcVer)
}

// smDrop removes the records for which drop(r) holds, keeping insertion order
// and the live prefix, and returns how many it removed.
func (s *State) smDrop(drop func(r *smRec) bool) int {
	r := coldSrcMem.of(s)
	n, i := 0, 0
	for ; i < len(r) && r[i].fl != 0; i++ {
		if !drop(&r[i]) {
			r[n] = r[i]
			n++
		}
	}
	for j := n; j < i; j++ {
		r[j] = smRec{}
	}
	return i - n
}

// smClear drops src's records in kinds that e (nil: none) can read.
func (s *State) smClear(src Obj, kinds uint8, e *stackEntry) {
	n := s.smDrop(func(r *smRec) bool { return r.src == src && r.fl&kinds != 0 && s.smLiveFor(r, e) })
	if DebugSrcMemStats != nil {
		DebugSrcMemStats.Cleared += n
	}
}

// smForget drops the records of o in src's lists of kinds.
func (s *State) smForget(o, src Obj, kinds uint8) {
	if DebugSrcMemNoForget {
		return
	}
	n := s.smDrop(func(r *smRec) bool { return r.o == o && r.src == src && r.fl&kinds != 0 })
	if DebugSrcMemStats != nil {
		DebugSrcMemStats.Forgot += n
	}
}

// smDropImp ends src's Imprinted lists: the battlefield-entry wipe
// (zone_imprint.go impEnter), where every record is from an earlier
// incarnation.
func (s *State) smDropImp(src Obj) {
	n := s.smDrop(func(r *smRec) bool { return r.src == src && r.fl&(smImp|smImpAny) != 0 })
	if DebugSrcMemStats != nil {
		DebugSrcMemStats.Forgot += n
	}
}

// smGC frees the records no one can read: the source is not live at the
// stamped ver and no stack entry or pending trigger of it holds the stamped
// incarnation (srcVer in {sver, sver+1}).
func (s *State) smGC() {
	n := s.smDrop(func(r *smRec) bool {
		if s.smLiveNow(r) {
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

func init() { stallNames[StallSrcMem] = "srcmem" }
