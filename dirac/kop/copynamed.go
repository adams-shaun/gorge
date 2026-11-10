package kop

// ExtCopyNamed (CopyPermanent DefinedName$, Mutable Explorer): the resolving
// controller creates a token copy of the card B.N names (a CardID: kgen
// writes the pool index and the matchup remaps it, as for ExtEndure's token),
// which is no object on the battlefield but a card of the table. F stays
// zero: every instruction walker reads it as a filter. C packs the low byte
// of OpCopyAsTokenEx's CopyFlags (CpTapped and the end-step riders; no pump
// keyword, no NonRemCtrl) with its copy-exception override index+1 in the
// byte above (CopyNamedExcShift; at most 255). The count is one and the
// source names no object, so the copy has no template (no face-down copy, no
// copy of a copy). An
// Aura or a card with an as-enters choice or a clone election stalls
// (StallCopyAllAsk) rather than ask. gorge copies nothing when the name is
// not in its card universe, and kgen emits nothing for such a name.
// dirac/copy_named.go.
const ExtCopyNamed ExtOp = 314

// CopyNamedExcShift: ExtCopyNamed C >> CopyNamedExcShift (as uint16) is the
// copy exceptions' override index+1, 0 for none; C & 0xff the CopyFlags.
const CopyNamedExcShift = 8

// CopyCardKeyPrefix prefixes the name of the table entry a named-card copy
// makes its tokens from: the prefix plus the named card's name. The entry is
// that card's face (kgen lowers it as a pool token); the corpus name the
// card goes by in gorge is the name after the prefix (canon.BindSet).
const CopyCardKeyPrefix = "copycard:"

func init() { extOpNames[ExtCopyNamed] = "CopyNamed" }
