package kgen

// BlankLine is Forge's display-only spacer (DB$ BlankLine). It has no effect
// of its own: the SubAbility$ link orders the next node in the chain (the
// Path of the X cycle's DBSpace before DBVote), and the chain reader has
// already consumed that link. Gorge resolves the spacer as a silent no-op
// (effects/planechase.go effBlankLine); the kernel needs no instruction for
// it, so the lowering emits none. Any rules parameter beside the spacer stays
// unread, so the chain's ordinary unread-parameter check refuses it.
func blankLine(_ *lowerCtx, _ *params) error { return nil }

func init() { apiPS("BlankLine", blankLine) }
