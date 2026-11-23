package dirac

// Soulshift (CR 702.46a) is gorge kwSoulshift's expansion (cards/
// kw_soulshift.go): an optional ChangesZone dies trigger whose body returns
// a Spirit card with mana value N or less from its owner's graveyard to
// their hand. The kernel reads no soulshift of its own: the trigger and its
// ChangeZone body lower like every other one (kgen lower_expandedkw4.go),
// the mana-value bound riding the target filter's PredCMCLE.

// DebugSoulshiftNoBound (TestExpandedKeywordsSoulshiftMutation): a card
// filter's PredCMCLE always matches, so the soulshift return's mana-value
// bound is gone -- the kernel offers a return of any Spirit card in the
// graveyard where gorge offers only mana value N or less, and a game whose
// graveyard holds a dearer Spirit when a soulshift trigger fires diverges at
// the target ask. Set only by the mutation test, which runs before the
// parallel tests (the venture mutation tests' shape), never in parallel.
var DebugSoulshiftNoBound bool

// predCardCMCLE is a card filter's PredCMCLE read, kept out of line so
// matchCardOneAt keeps its shape (the complexity ratchet): the
// DebugSoulshiftNoBound mutation ignores the bound entirely.
//
//go:noinline
func predCardCMCLE(mv, n int) bool {
	if DebugSoulshiftNoBound {
		return true
	}
	return mv <= n
}
