package kop

// TokImprintObj (OpCreateTokenEx B.N bit 14): each created token joins the
// resolution's imprinted set (ImprintTokens$ True; RefImprinted reads it,
// ExtClearImprinted empties it, dirac/zone_imprint.go). The low byte of B.N
// holds the other token flags and the high byte a destination Ref or an
// AtEOT kind, so a script that needs either of those beside the imprint
// stays refused. The core OpCreateToken has no room for it: the imprinting
// token always takes the Ex form.
const TokImprintObj = 1 << 14
