package main

import "github.com/adams-shaun/gorge/engine/deck"

// The plain-text decklist parser lives in engine/deck (deck.ParseText) so the
// server can take a text decklist too; these names keep the tool's code and
// tests reading as before.
type (
	parsedDeck = deck.TextList
	parsedCard = deck.Entry
)

func parseDecklist(raw []byte) (*parsedDeck, error) { return deck.ParseText(raw) }
