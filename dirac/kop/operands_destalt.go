package kop

// LibChoiceCtrlGrave is OpToLibraryChoice's C for an unconditional
// DestinationAlternative$ Library / LibraryPositionAlternative$ -1 with no
// AlternativeDecider$ (Not Forgotten, Arashin Sovereign): the RESOLVING
// CONTROLLER chooses the top or the bottom of the card's owner's library,
// asked only while the card is still in a graveyard (gorge
// controllerPositionAsk). Clear of LibChoiceSecond.
const LibChoiceCtrlGrave = 2

// Search flags in Instr.B of the unconditional DestinationAlternative$ asks.
const (
	// SrHandPosAsk (OpChooseMove from a hand into the library only, where
	// B bits 4..11 carry no entry counter kind): once the picked cards are
	// on top, the resolving controller chooses top or bottom for the whole
	// set (ChGenTopBottom; gorge handLibraryTail, Dream Cache).
	SrHandPosAsk = 1 << 5
	// SrDestAltGrave (OpSearchEx without SrAltOrigin only, where B bits
	// 5..7 name no alternative origin zone): with a card found, the searcher chooses the search's destination or the
	// graveyard before the move (ChGenDestAlt; gorge zone_search.go
	// askUnconditionalDestination, Dina's Guidance).
	SrDestAltGrave = 1 << 5
)
