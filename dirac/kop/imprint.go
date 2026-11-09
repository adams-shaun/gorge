package kop

// The resolution's imprinted set (dirac/zone_imprint.go): an Imprint$ writer
// flags its move with MvImprint, a later instruction of the same resolution
// reads the set as RefImprinted, and ExtClearImprinted empties it. It is
// apart from the remembered set: one object may be in both.

// RefImprinted: the resolution's imprinted objects (the first one where a
// single object is read; none when the set is empty).
const RefImprinted Ref = 35

// MvImprint (OpExileEx C): the moved objects join the resolution's imprinted
// set (Imprint$ True); independent of MvRemember. Bit 13 is MvFromGraveyard;
// 14 is unused by the unshifted OpExileEx C and keeps it positive in int16.
const MvImprint = 1 << 14

// ExtClearImprinted forgets the resolution's imprinted objects (Cleanup
// ClearImprinted$; the op space is full, so it is an extension sub-op).
const ExtClearImprinted ExtOp = 257

func init() { extOpNames[ExtClearImprinted] = "ClearImprinted" }
