package kop

// OpTwoPiles flags (B.N). Zero is Curator of Destinies' shape: the resolving
// controller separates the remembered cards, the first live opponent picks a
// pile, and the unchosen pile goes to the graveyard in the library's order (a
// ChangeZoneAll ChangeType$ Card.IsRemembered body).
const (
	// TpOppSeparates: the first live opponent separates and the resolving
	// controller picks (Separator$ Opponent, the chooser Defined$ You: Fact or
	// Fiction, Sphinx of Uthuun).
	TpOppSeparates = 1 << iota
	// TpUnchosenPileOrder: the unchosen pile moves in its own order, pile A
	// in pick order (a ChangeZone Defined$ Remembered body: gorge's
	// moveDefinedLibraryObjects walks the pile's remembered list).
	TpUnchosenPileOrder
)
