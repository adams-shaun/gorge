package dirac

// CharmShapeStats counts, over a lockstep game, the casts the demo Charm
// shapes decided (TestDemoCharmShapesLockstep reads them so a green game is
// not one where no shape was reached). DebugCharmShapeStats, when set,
// receives the counts; the kernel's hot path reads it behind a nil check.
type CharmShapeStats struct {
	// KickedAnyNumber: kicked casts whose mode step took no minimum
	// (CardDef.ModesMinKicked).
	KickedAnyNumber int64
	// UpToXSlots: sequential per-mode target asks of a 0..X slot (a
	// multi-pick charm's "up to X target ..." mode).
	UpToXSlots int64
	// CloneBecomers: permanents that became a copy of a template other than
	// themselves (Masterful Replication's mode: each other artifact).
	CloneBecomers int64
}

// DebugCharmShapeStats, when set, receives the CharmShapeStats counts.
var DebugCharmShapeStats *CharmShapeStats

// DebugCharmUpToX makes a multi-pick charm's 0..X slot take the combined
// per-mode ask a 1..1 slot takes (mutation test only).
var DebugCharmUpToX bool
