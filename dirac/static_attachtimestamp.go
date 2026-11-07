package dirac

// CR 613.7e: an Aura, Equipment or Fortification receives a new timestamp
// when it becomes attached to an object (gorge events.foldAttach /
// restampLayer: any attach to a DIFFERENT object restamps the attached
// permanent's layer timestamp, Object.LayerStamp; the entry timestamp,
// perm.ts here, stays the permanent's ordering identity). The layer stamp
// lives beside perm in State.lstamp, written at entry (zones.go enter) and
// on an attach move, and nowhere else: perm.ts is also restamped by a
// control change (joined: the battlefield list order), which CR 613.7d and
// gorge do not treat as a new layer timestamp (CR 613.7).

// attachTo attaches the permanent at sl to host, restamping its layer
// timestamp when that is a move (gorge: AttachedTo changes), then applies
// any lose-all onset its static brings.
func (s *State) attachTo(k *prep, sl, host Slot) {
	if old := s.attachQuiet(k, sl, host); old != host && k.gen != nil && k.gen.anyAegis {
		s.aegisMoved(k, sl, old, host) // zone_exilethenattach.go
	}
}

// attachQuiet is attachTo without the attachment's Attached triggers (the
// caller fires them); it returns the previous bearer.
func (s *State) attachQuiet(k *prep, sl, host Slot) (old Slot) {
	old = s.perms[sl].attached
	if old != host {
		s.ts++
		s.lstamp[sl] = s.ts
	}
	s.perms[sl].attached = host
	s.perms[sl].flags &^= pfLastBear
	s.loseAllOnset(k, sl, host)
	return old
}

// layerTs is the CR 613.7 timestamp the static abilities of the permanent
// at sl take (gorge Object.LayerTimestamp).
func (s *State) layerTs(sl Slot) uint16 {
	return s.lstamp[sl]
}
