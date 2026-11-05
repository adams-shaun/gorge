package kgen

// lower_playlibrary.go: the tail of a Play from the remembered top of the
// library (Cosmic Cube: "look at the top six cards of your library. You may
// cast a spell from among them ... Put the rest on the bottom of your
// library in a random order"). gorge's ChangeZoneAll RandomOrder$
// (effects/zone_changeall.go) collects the remembered cards still in the
// library in library order (top first), Fisher-Yates them backward through
// the engine rng and moves each onto the bottom. The card the Play began to
// cast has already left the library in gorge; the kernel's is still queued
// (freeQ), so the kernel skips it (kernel act_discoverbottom.go remBottom).

// playLibraryBottom lowers exactly ChangeZoneAll ChangeType$ Card.IsRemembered,
// Origin$ Library, Destination$ Library, LibraryPosition$ -1, RandomOrder$
// True after a Play ValidZone$ Library in the same chain.
func (lc *lowerCtx) playLibraryBottom(ps *params) error {
	for _, kv := range [][2]string{{"ChangeType", "Card.IsRemembered"}, {"Origin", "Library"},
		{"Destination", "Library"}, {"LibraryPosition", "-1"}, {"RandomOrder", "True"}} {
		if k, want := kv[0], kv[1]; ps.str(k) != want {
			return refuse("ChangeZoneAll after a Play from the library: %s$ %q", k, ps.str(k))
		}
	}
	ps.consume("ChangeType", "Origin", "Destination", "LibraryPosition", "RandomOrder")
	lc.emit(lInstr{Op: "RemBottom"})
	return nil
}
