package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Copy layers on an Assimilation Aegis bearer (CR 613.7e, 707.2).
//
// A permanent can hold several layer-1 copy effects at once: permanent copy
// markers (a Clone), until-end-of-turn ones (Fleeting Reflection), and one
// per Aegis copy attached to it. Gorge keeps every effect and re-bases the
// permanent on the newest one left whenever one ends. The kernel keeps one
// copy slot, so it also keeps each layer's timestamp:
//
//   - fxAegisBase: a copy the Aegis stack covered that has no record of its
//     own (a token minted as a copy, or a copy a table without ExtCloneCopy
//     made as it entered). It shows again when the last Aegis copy ends.
//   - fxAegisHeld: that copy's exception records, put aside while the Aegis
//     copy shows and given back with the floor (aegisHold/aegisRestore).
//   - fxCopyPerm / fxCopyEOT: the newest permanent and until-end-of-turn
//     copy markers, with the clone's card and layer timestamp (copy_clone.go).
//
// copyTop then picks the newest live layer by timestamp, so the layer the
// Aegis copy covered comes back when it ends, and the Aegis copy comes back
// when a newer clone ends, whichever order the layers were added in. A later
// Aegis attach is a newer layer than both, and each Aegis copy ends on its
// own; the newest surviving Aegis copy is chosen by its attachment's layer
// stamp (CR 613.7e). An Aegis copy that covers a clone is not tracked in a
// side record: its own layer is enough to find it again.

// aegisUnderPush keeps the copy host wears aside, for the last Aegis copy's
// end to give back. It is only needed for a copy no marker records (see the
// file comment). False when the pool is full.
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

// aegisHold puts host's copy-exception records aside while an Aegis copy
// shows (fxAegisHeld). They are the covered copy's, and it has no marker to
// rebuild them from; aegisRestore gives them back with the copyBase floor.
func (s *State) aegisHold(host Slot) {
	if !s.fxM.has(host) {
		return
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == host && r.kind == fxCopyExc {
			r.kind = fxAegisHeld
		}
	}
	s.aegisHoldGrants(host)
}

// aegisRestore puts the covered copy's exceptions back (copyBase).
func (s *State) aegisRestore(host Slot) {
	if !s.fxM.has(host) {
		return
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == host && r.kind == fxAegisHeld {
			r.kind = grantHeldKind(r)
		}
	}
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

// copyLayer is one layer-1 copy effect for copyTop: its card, layer
// timestamp, and (for a copy marker) the template its exceptions come from.
type copyLayer struct {
	kind  uint8
	card  CardID
	ts    uint16
	idx   uint8 // the copy's own exception override (index+1, 0 none)
	grant uint8 // an as-enters copy marker's AddAbilities$ override (index+1, 0 none)
	src   Obj   // the template object (copy markers only)
}

const (
	copyNone  uint8 = iota // no copy layer: the printed card
	copyPerm               // the permanent copy marker (fxCopyPerm)
	copyEOT                // the until-end-of-turn copies (fxCopyEOT)
	copyAegis              // a live Assimilation Aegis copy
	copyBase               // the floor kept by fxAegisBase
)

// copyRec is the index of host's kind record, -1 when none. Copy markers are
// one per permanent; Aegis layers live in the link table, not here.
func (s *State) copyRec(host Slot, kind uint8) int {
	if !s.fxM.has(host) {
		return -1
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == host && r.kind == kind {
			return i
		}
	}
	return -1
}

// copyTop is host's newest live copy layer by layer timestamp (CR 613.7e):
// its live Aegis copies, its permanent copy marker and its until-end-of-turn
// copies. The cleanup step drops the EOT record first, so it is not a
// candidate while it settles (cloneCleanup); everywhere else all three are
// live. skip is an attachment that is leaving. Aegis copies are ordered by
// the tick their trigger resolved on (genState.links ts, as gorge stamps the
// continuous effect at resolution); a copy marker
// by the clock tick it landed on. copyNone means no layer: the printed card
// (or fxAegisBase).
func (s *State) copyTop(host, skip Slot) copyLayer {
	var l copyLayer
	best, has := uint16(0), false
	if s.g.linkAeg != 0 {
		for i := 0; i < int(s.g.nlinks); i++ {
			if s.g.linkAeg>>uint(i)&1 == 0 {
				continue
			}
			lk := &s.g.links[i]
			ob := &s.objs[lk.src]
			if ob.zone != kop.ZoneBattlefield || ob.ver != lk.sver || ob.slot == skip || s.perms[ob.slot].attached != host {
				continue
			}
			ts := lk.ts
			if !has || ts > best {
				l = copyLayer{kind: copyAegis, card: s.objs[lk.obj].card, ts: ts}
				best, has = ts, true
			}
		}
	}
	if r := s.copyRec(host, fxCopyPerm); r >= 0 {
		// the card the marker locked when it was applied (CR 707.2b): the
		// source is not read again, as gorge's re-base restores the layer's
		// locked face (state.CloneLayer)
		src := Obj(s.g.fx[r].r.p)
		c := s.g.fx[r].r.src
		if ts := s.g.fx[r].r.ts; !has || ts >= best {
			l = copyLayer{kind: copyPerm, card: c, ts: ts, idx: s.g.fx[r].r.idx, src: src, grant: s.g.fx[r].r.fl}
			best, has = ts, true
		}
	}
	if r := s.copyRec(host, fxCopyEOT); r >= 0 {
		if ts := s.g.fx[r].r.ts; !has || ts >= best {
			l = copyLayer{kind: copyEOT, card: s.g.fx[r].r.src, ts: ts, idx: s.g.fx[r].r.idx, src: Obj(s.g.fx[r].r.p)}
		}
	}
	return l
}
