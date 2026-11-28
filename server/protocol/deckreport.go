package protocol

// DeckReport is POST /api/decks/check's answer, and the deck field of a
// refused POST /api/games: what the server found in a submitted deck. Every
// field is computed on the server; every list is empty, never null.
type DeckReport struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	// Engine is the engine whose support Unsupported measures, always gorge.
	Engine     string   `json:"engine"`
	Cards      int      `json:"cards"`
	Commanders []string `json:"commanders"`
	// Identity is the deck's colour identity as a subset of WUBRG, in that
	// order: the commanders' for a commander deck, every resolved card's for
	// a constructed one.
	Identity string `json:"identity"`
	// Legal is no Problems and no Unknown names.
	Legal bool `json:"legal"`
	// Problems is one line per construction violation.
	Problems []string `json:"problems"`
	// Unknown is the names the card pool does not hold, first-seen order.
	Unknown []string `json:"unknown"`
	// Unsupported is every resolved card the engine cannot fully play,
	// sorted by name.
	Unsupported []DeckReportCard   `json:"unsupported"`
	Substituted []DeckSubstitution `json:"substituted"`
	// Playable is Legal and no Unsupported card.
	Playable bool `json:"playable"`
	// Dirac is the second engine's verdict on the deck, absent when the
	// server judges no deck on dirac.
	Dirac *DeckEngineVerdict `json:"dirac,omitempty"`
}

// DeckEngineVerdict is a second engine's verdict on the deck a report
// describes: the set it would play the deck on, every card that set does not
// play, and, when no card is named but the deck is still refused, why.
type DeckEngineVerdict struct {
	Engine      string           `json:"engine"`
	Set         string           `json:"set"`
	Playable    bool             `json:"playable"`
	Unsupported []DeckEngineCard `json:"unsupported"`
	Problems    []string         `json:"problems"`
}

// DeckEngineCard is one card a second engine's set does not play: its copies
// in the deck, its status (not in set, not lowered, refused, unverified) and
// the set's reason.
type DeckEngineCard struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// DeckReportCard is one card a deck carries that the engine cannot play:
// its copies in the deck and the primitives it needs.
type DeckReportCard struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Needs []string `json:"needs"`
}

// DeckSubstitution is one card swapped for another in a checked deck.
type DeckSubstitution struct {
	Out   string `json:"out"`
	In    string `json:"in"`
	Count int    `json:"count"`
}
