package kop

// ExtDigImprint marks the OpDigEx right before it: every card that dig takes
// joins the resolution's imprinted set (Dig Imprint$ True on a permanent's
// ability; RefImprinted reads it, ExtClearImprinted empties it). The marker
// does nothing when it runs itself: the dig reads it as it takes each card
// (dirac/zone_digimprint.go). The OpDigEx C word has no bit left for a flag,
// and the op space is full, so the mark is an extension sub-op.
const ExtDigImprint ExtOp = 282

func init() { extOpNames[ExtDigImprint] = "DigImprint" }
