package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The copy under an Assimilation Aegis copy (CR 613.7e, 707.2).
//
// A bearer that already wears a copy (Fleeting Reflection's until-end-of-turn
// copy, a Clone) when an Aegis becomes attached to it becomes a copy of the
// exiled card: the Aegis effect is the newest, so it applies last and wins.
// Gorge keeps every copy effect and re-bases the permanent on the newest one
// that is left whenever one ends. The kernel keeps one copy slot, so the
// copy the Aegis covers waits in an fxAegisBase record (r.src its card):
// aegisRefresh brings it back when the last Aegis copy on the bearer ends,
// and the cleanup step (aegisSettle) ends an until-end-of-turn one beneath a
// live Aegis copy without touching the Aegis copy.
//
// The other order is symmetric: a clone that lands on a live Aegis bearer is
// the newer layer and wins while it lasts, and the cleanup step gives the
// Aegis copy back (CR 514.2). Its card waits in an fxAegisCover record, kept
// up to date by aegisRefresh while the Aegis stays live (a later Aegis
// attachment is newer than the first but still older than the clone). When
// the Aegis ends first, the record goes and the clone stays.

// aegisCoverRec is the index of host's covered Aegis copy, -1 when none.
func (s *State) aegisCoverRec(host Slot) int {
	for i := 0; s.fxM.has(host) && i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == host && r.kind == fxAegisCover {
			return i
		}
	}
	return -1
}

// aegisCovered reports whether a clone layer covers host's Aegis copy.
func (s *State) aegisCovered(host Slot) bool {
	return s.aegisCoverRec(host) >= 0
}

// aegisCoverPush records the Aegis copy host wears before a newer clone
// covers it. An existing record already holds it. False when the pool is full.
func (s *State) aegisCoverPush(host Slot) bool {
	if s.aegisCoverRec(host) >= 0 {
		return true
	}
	if int(s.g.nfx) >= maxFx {
		s.stall(StallTables)
		return false
	}
	s.g.fx[s.g.nfx] = fxRec{sl: host, kind: fxAegisCover, r: ovRec{src: s.perms[host].ovr.copyOf}}
	s.g.nfx++
	s.fxM.set(host)
	return true
}

// aegisUnderPush keeps the copy host wears aside. False when the pool is full.
func (s *State) aegisUnderPush(host Slot) bool {
	if int(s.g.nfx) >= maxFx {
		s.stall(StallTables)
		return false
	}
	s.g.fx[s.g.nfx] = fxRec{sl: host, kind: fxAegisBase, r: ovRec{src: s.perms[host].ovr.copyOf}}
	s.g.nfx++
	s.fxM.set(host)
	return true
}

// aegisUnderTake removes host's kept copy and returns its card.
func (s *State) aegisUnderTake(host Slot) (c CardID, ok bool) {
	if !s.fxM.has(host) {
		return 0, false
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == host && r.kind == fxAegisBase {
			c, ok = r.r.src, true
		}
	}
	if ok {
		s.dropFx(func(i int) bool { r := &s.g.fx[i]; return r.sl == host && r.kind == fxAegisBase })
	}
	return c, ok
}

// aegisSettle is the cleanup step (CR 514.2) for sl's until-end-of-turn copies
// when an Aegis copy is live on it: they end beneath it, the permanent copy
// marker it holds (if any) becomes the copy that shows when the Aegis copy
// ends, and the Aegis copy stays. False when no Aegis copy is live.
//
//go:noinline
func (s *State) aegisSettle(k *prep, sl Slot) bool {
	if !k.gen.anyAegis || s.aegisLive(sl) == 0 {
		return false
	}
	s.dropFx(func(i int) bool {
		x := &s.g.fx[i]
		return x.sl == sl && (x.kind == fxCopyEOT || x.kind == fxCopyExc || x.kind == fxAegisBase)
	})
	for i := 0; i < int(s.g.nfx); i++ {
		x := &s.g.fx[i]
		if x.sl != sl || x.kind != fxCopyPerm {
			continue
		}
		// the marker's source as it is now (cloneCleanup's re-base)
		src := Obj(x.r.p)
		ob := &s.objs[src]
		c := ob.card
		if ob.zone == kop.ZoneBattlefield {
			c = s.cardAt(ob.slot)
		}
		if int(s.g.nfx) >= maxFx {
			s.stall(StallTables)
			break
		}
		s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxAegisBase, r: ovRec{src: c}}
		s.g.nfx++
		s.fxM.set(sl)
		break
	}
	return true
}
