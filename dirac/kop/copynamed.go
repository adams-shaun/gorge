package kop

// ExtCopyNamed (CopyPermanent DefinedName$, Mutable Explorer): the resolving
// controller creates a token copy of the card F names (a CardID, kgen writes
// the pool index and the matchup remaps it; the field is no filter here),
// which is no object on the battlefield but a card of the table. B is the
// CopyFlags of OpCopyAsTokenEx and C its copy-exception override index+1,
// read as they are there; the count is one and the source names no object,
// so the copy has no template (no face-down copy, no copy of a copy). An
// Aura or a card with an as-enters choice or a clone election stalls
// (StallCopyAllAsk) rather than ask. gorge copies nothing when the name is
// not in its card universe, and kgen emits nothing for such a name.
// dirac/copy_named.go.
const ExtCopyNamed ExtOp = 314

func init() { extOpNames[ExtCopyNamed] = "CopyNamed" }
