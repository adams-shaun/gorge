package kop

// TokRememberSrc (OpCreateTokenEx B.N bit 15, as a uint16 image, the int16
// sign bit): each created token remembers the creating source object
// (RememberSource$ True; gorge's minted token's Object.Remembered names the
// source). The kernel keeps it as an smRem record keyed by the token
// (dirac/zone_tokrem.go), read by ExSrcRemCounters. Like TokImprintObj it
// shares B.N's high byte with a destination Ref or an AtEOT kind, so a script
// that needs either beside it stays refused.
const TokRememberSrc = 1 << 15
