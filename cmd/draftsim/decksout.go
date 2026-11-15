package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
)

// deckFileName is the -decks-out file name of one seat's deck:
// <set lower>_e<event seed>_s<seat>.json.
func deckFileName(set string, seed uint64, seat int) string {
	return fmt.Sprintf("%s_e%d_s%d.json", strings.ToLower(set), seed, seat)
}

// writeDecks writes every seat's saved deck of a finished event to dir as a
// gorge deck JSON file, through the same SavedDeck → build.ToDeckFile path
// the games resolve. A Commander-draft deck carries Format "commander" and
// its Commanders, so deck.File.CommanderIndices seats them again; a limited
// deck carries Format "limited". The content is a pure function of the
// event: no timestamps, no paths.
func writeDecks(dir, set string, ev *event.Event, out io.Writer) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for s := 0; s < ev.NumSeats(); s++ {
		d, ok := ev.SavedDeck(s)
		if !ok {
			return fmt.Errorf("seat %d has no saved deck", s)
		}
		name := deckFileName(set, ev.Seed(), s)
		f := build.ToDeckFile(ev.Pool(s), d, strings.TrimSuffix(name, ".json"))
		f.Format = "limited"
		if ev.Format() == event.FormatCommander {
			// The deck-file spelling the Commander readers key on
			// (dirac/oracle seats commanders only under it).
			f.Format = "commander"
		}
		raw, err := json.MarshalIndent(f, "", "  ")
		if err != nil {
			return fmt.Errorf("seat %d: %w", s, err)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "  seat %d deck: %s (%d cards)\n", s, path, d.Total())
	}
	return nil
}
