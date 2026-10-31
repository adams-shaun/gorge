package draftlab_test

import (
	"bytes"

	"github.com/adams-shaun/gorge/draft/draftlab"
)

// Example shows the E5-c4 artifact writers: the 17Lands-shaped draft_data,
// game_data and decks.json exports. It compiles the calls only (no Output
// comment), so the example runs with no corpus.
func Example() {
	l, err := draftlab.Open(".cards", draftlab.Options{})
	if err != nil {
		return
	}
	names, err := l.CardNames("FDN", "")
	if err != nil {
		return
	}
	gameNames := append(append([]string(nil), names...), draftlab.BasicNames[:]...)
	var buf bytes.Buffer
	// draft_data: one row per pick of a finished event's log.
	_ = draftlab.DraftDataCSV(&buf, nil, names, true)
	// game_data: one row per player per finished game.
	_ = l.GameDataCSV(&buf, nil, "FDN", "gorge_lab", gameNames, true)
	// decks.json: the built decks with their provenance.
	_ = draftlab.WriteDecksJSON(&buf, nil)
}
