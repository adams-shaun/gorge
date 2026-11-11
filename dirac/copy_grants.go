package dirac

// As-enters copy grants over the copy layers (CR 707.9b, 707.3, 613.1a).
//
// A copy's AddAbilities$ (Mercurial Pretender) is one of its copiable values:
// gorge folds it into the copy's layer-1 face, so it is covered with the rest
// of the copy when a newer copy lands on the permanent, comes back when a
// re-base gives that layer back, and a copy of the copy has it. Here it is a
// fxOverride record marked ovCopyGrant (becomeCopyEnter), and every step a
// copy layer takes treats those records like the layer's fxCopyExc ones:
//
//   - a newer layer drops them (cloneLayerPrep, cloneCleanup, aegisRefresh);
//     the template's come with the new layer (copyGrantsAdd), a self-copy's own
//     are kept beside the layer (fxCopySelf records marked ovCopyGrant, the
//     ones that hold its inherited exceptions);
//   - an Aegis copy with no marker under it holds them aside (fxAegisHeld);
//   - a re-base derives them again (copyGrantsRebase): the template's, then,
//     for an as-enters copy marker, its own (fxCopyPerm.r.fl, the grant
//     override index+1), sourced from the permanent's printed card.

// cgrantRef is one as-enters grant: the override index+1 and the card whose
// abilities its Grant indexes name.
type cgrantRef struct {
	idx uint8
	src CardID
}

// copyGrantRecs lists slot sl's as-enters grant records in pool order.
func (s *State) copyGrantRecs(sl Slot, out *[maxOvr]cgrantRef) (n int) {
	if !s.fxM.has(sl) {
		return 0
	}
	for i := 0; i < int(s.g.nfx) && n < len(out); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxOverride && r.r.idx != 0 && r.r.copyGrant() {
			out[n] = cgrantRef{r.r.idx, r.r.src}
			n++
		}
	}
	return n
}

// copyGrantPut applies grant g to sl as an as-enters copy grant unless sl
// already has that grant: a copy of a copy of the same exception has the
// ability once, as gorge's fold keeps it once.
func (s *State) copyGrantPut(k *prep, sl Slot, g cgrantRef) {
	var have [maxOvr]cgrantRef
	for i, n := 0, s.copyGrantRecs(sl, &have); i < n; i++ {
		if have[i] == g {
			return
		}
	}
	s.applyOverrideFl(k, sl, int(g.idx)-1, g.src, ovCopyGrant)
}

// copyGrantsAdd carries the as-enters grant records of the template at src
// (a copy of a Pretender, CR 707.3) onto the new copy at sl, in pool order,
// each stamped anew.
//
//go:noinline
func (s *State) copyGrantsAdd(k *prep, sl, src Slot) {
	if src == NoSlot || !s.fxM.has(src) {
		return
	}
	var from [maxOvr]cgrantRef
	for i, n := 0, s.copyGrantRecs(src, &from); i < n; i++ {
		s.copyGrantPut(k, sl, from[i])
	}
}

// dropCopyGrants drops slot sl's as-enters grant records: a newer copy layer
// covers the copy they are copiable values of.
//
//go:noinline
func (s *State) dropCopyGrants(sl Slot) {
	if !s.fxM.has(sl) {
		return
	}
	s.dropFx(func(i int) bool {
		r := &s.g.fx[i]
		return r.sl == sl && r.kind == fxOverride && r.r.idx != 0 && r.r.copyGrant()
	})
}

// clonePermGrant records on slot sl's permanent copy marker the grant override
// (index+1) its as-enters copy carries, for a re-base to apply again.
func (s *State) clonePermGrant(sl Slot, grant uint8) {
	if i := s.copyRec(sl, fxCopyPerm); i >= 0 {
		s.g.fx[i].r.fl = grant
	}
}

// cloneSelfGrantsPush keeps a self-copy layer's inherited grants beside the
// layer, stamped with its timestamp: fxCopySelf records marked ovCopyGrant
// (cloneSelfHeldPush's twin for the grants).
//
//go:noinline
func (s *State) cloneSelfGrantsPush(sl Slot, kept *[maxOvr]cgrantRef, n int, ts uint16) {
	for i := 0; i < n; i++ {
		if int(s.g.nfx) >= maxFx {
			s.stall(StallTables)
			return
		}
		s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxCopySelf, r: ovRec{idx: kept[i].idx, src: kept[i].src, fl: ovCopyGrant, ts: ts}}
		s.g.nfx++
		s.fxM.set(sl)
	}
}

// copyGrantList is the grants layer l shows on host, in the order they are
// applied: the template's (inherited, CR 707.3), then an as-enters copy
// marker's own (sourced from host's printed card). An Aegis copy has none; a
// self-copy's template is host itself, so its inherited grants are the held
// ones.
func (s *State) copyGrantList(host Slot, l copyLayer, out *[maxOvr]cgrantRef) (n int) {
	if l.kind != copyPerm && l.kind != copyEOT {
		return 0
	}
	if l.src != NoObj && l.src == s.perms[host].obj {
		for i := 0; i < int(s.g.nfx) && n < len(out); i++ {
			if r := &s.g.fx[i]; r.sl == host && r.kind == fxCopySelf && r.r.ts == l.ts && r.r.fl&ovCopyGrant != 0 {
				out[n] = cgrantRef{r.r.idx, r.r.src}
				n++
			}
		}
	} else if src := s.copySrcSlot(l.src); src != NoSlot {
		n = s.copyGrantRecs(src, out)
	}
	if l.grant != 0 && n < len(out) {
		out[n] = cgrantRef{l.grant, s.perms[host].card}
		n++
	}
	return n
}

// copyGrantsRebase replaces host's grant records with the ones layer l shows.
//
//go:noinline
func (s *State) copyGrantsRebase(k *prep, host Slot, l copyLayer) {
	s.dropCopyGrants(host)
	var gs [maxOvr]cgrantRef
	for i, n := 0, s.copyGrantList(host, l, &gs); i < n; i++ {
		s.copyGrantPut(k, host, gs[i])
	}
}

// copyGrantsShow reports whether host's grant records already are the ones
// layer l shows (excShows' twin for the grants).
func (s *State) copyGrantsShow(host Slot, l copyLayer) bool {
	var want, have [maxOvr]cgrantRef
	n := s.copyGrantList(host, l, &want)
	if n != s.copyGrantRecs(host, &have) {
		return false
	}
	for i := 0; i < n; i++ {
		if want[i] != have[i] {
			return false
		}
	}
	return true
}

// aegisHoldGrants: an Aegis copy that covers a copy with
// no marker of its own holds that copy's grants aside (fxAegisHeld, the mark
// kept in fl); aegisRestore gives them back as live overrides (grantHeldKind).
func (s *State) aegisHoldGrants(host Slot) {
	for i := 0; s.fxM.has(host) && i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == host && r.kind == fxOverride && r.r.idx != 0 && r.r.copyGrant() {
			r.kind = fxAegisHeld
		}
	}
}

// grantHeldKind is the kind an fxAegisHeld record returns to: a held grant
// goes back to an override, a held exception to a copy exception.
func grantHeldKind(r *fxRec) uint8 {
	if r.r.copyGrant() {
		return fxOverride
	}
	return fxCopyExc
}
