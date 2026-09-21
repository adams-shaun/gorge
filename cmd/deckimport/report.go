package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/deck"
)

// missingCard is one card name the corpus does not contain and its count in
// the decklist, so the report says how big each gap is.
type missingCard struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// deckReport is the per-deck findings, in the same shape for the terminal
// report and the -json output. `ok` is whether the deck is fully playable
// (every card in the corpus). `written` is whether the deck file was written
// to disk. `commander_*` are populated only for a deck that names a
// commander.
type deckReport struct {
	Input        string        `json:"input"`
	Name         string        `json:"name"`
	Format       string        `json:"format"`
	Commander    string        `json:"commander,omitempty"`          // the first commander (legacy singular display field)
	Commanders   []string      `json:"commanders,omitempty"`         // every commander, in order (a partner pair lists two)
	Cards        int           `json:"cards"`                        // total count, and cards included in the deck
	Resolved     int           `json:"resolved"`                     // count of cards that resolved to a corpus card
	Percent      float64       `json:"resolution_percent"`           // resolved / cards * 100
	Dropped      int           `json:"sideboard_cards_dropped"`      // sideboard card count discarded
	Missing      []missingCard `json:"missing"`                      // every card name not in the corpus, with count
	CommanderOk  *bool         `json:"commander_eligible,omitempty"` // from deck.ValidateCommander's eligibility check
	CommanderWhy []string      `json:"commander_reason,omitempty"`   // why the deck is not commander-valid, if anything
	Ok           bool          `json:"ok"`                           // fully playable: no missing cards
	Path         string        `json:"path,omitempty"`               // where the deck file was written, if written
	Written      bool          `json:"written"`                      // whether the deck file was written to disk
}

// report is the whole document, one deckReport per input.
type report struct {
	Decks []deckReport `json:"decks"`
}

// deckFile is the on-disk repo deck-file shape: a {name, format, cards}
// list with the authoring-metadata fields the repo's own deck files carry
// (commander/commanders, archetype, notes). It is written to the same
// {name, count} card list that deck.Parse reads, so a deck file it produces
// loads back through deck.Load unchanged; the extra fields ride along and
// are read by deck.Parse.
//
// A one-commander deck writes the legacy singular "commander" key — so
// files written before the partner-pair field existed are byte-identical
// under this writer — and a partner pair writes the plural "commanders"
// list instead. Both carry omitempty so a constructed deck — matching the
// repo's existing constructed deck files — does not emit an empty key
// (which is precisely how splitDecks tells a commander deck from a
// constructed one: presence-of-field, not non-empty value).
type deckFile struct {
	Name       string       `json:"name"`
	Format     string       `json:"format"`
	Commander  string       `json:"commander,omitempty"`
	Commanders []string     `json:"commanders,omitempty"`
	Archetype  string       `json:"archetype,omitempty"`
	Notes      string       `json:"notes,omitempty"`
	Cards      []deck.Entry `json:"cards"`
}

// analyzeDeck turns a parsed deck and the corpus into the findings. It never
// mutates the corpus and never guesses: a card that cannot be resolved is
// reported by its exact name.
func analyzeDeck(input, name, format string, p *parsedDeck, r *cards.Registry) deckReport {
	dr := deckReport{
		Input:  input,
		Name:   name,
		Format: format,
	}
	if len(p.Commanders) > 0 {
		dr.Commanders = p.Commanders
		dr.Commander = p.Commanders[0]
	}

	total := 0
	for _, c := range p.Cards {
		total += c.Count
	}
	dr.Cards = total

	// Resolve every card once and remember per-distinct-name whether it was
	// found, so a repeated missing name is reported once with its total count.
	resolved := 0
	byMissing := map[string]int{}
	seen := map[string]bool{}
	var missingOrder []string
	for _, c := range p.Cards {
		if _, ok := r.Lookup(c.Name); ok {
			resolved += c.Count
			continue
		}
		byMissing[c.Name] += c.Count
		if !seen[c.Name] {
			seen[c.Name] = true
			missingOrder = append(missingOrder, c.Name)
		}
	}
	for _, n := range missingOrder {
		dr.Missing = append(dr.Missing, missingCard{Name: n, Count: byMissing[n]})
	}
	dr.Resolved = resolved
	dr.Dropped = p.Sideboard
	if total > 0 {
		dr.Percent = float64(resolved) / float64(total) * 100
	}
	dr.Ok = len(dr.Missing) == 0

	// Commander eligibility: only meaningful when the deck names a commander.
	if len(p.Commanders) > 0 {
		ok, why := commanderEligibility(p, r)
		dr.CommanderOk = &ok
		dr.CommanderWhy = why
	}
	return dr
}

// commanderEligibility reports whether the deck's commander is commander-
// eligible (deck.ValidateCommander is the one arbiter) and, when not, why.
// It appends the commander-construction violations so a reader sees not just
// "not eligible" but "the deck has 99 cards" or "X has a colour identity
// outside the commander's".
func commanderEligibility(p *parsedDeck, r *cards.Registry) (bool, []string) {
	f := deck.File{Name: "imported", Cards: toEntries(p.Cards), Commanders: p.Commanders}
	if len(p.Commanders) > 0 {
		f.Commander = p.Commanders[0]
	}
	// The eligibility check itself (IsCommanderEligible) is what the brief
	// asks about; ValidateCommander is its superset (it also checks 100-card
	// / singleton / colour identity) and names the offender. Run both so the
	// report can say where the commander falls short. A commander the corpus
	// does not have fails its own message.
	if err := f.ValidateCommander(r); err != nil {
		return false, splitReasons(err.Error())
	}
	return true, nil
}

func toEntries(pc []parsedCard) []deck.Entry {
	out := make([]deck.Entry, 0, len(pc))
	for _, c := range pc {
		out = append(out, deck.Entry{Name: c.Name, Count: c.Count})
	}
	return out
}

// commanderLabel is the commander line the text report prints: the pair
// joined with " & " when the deck carries two, else the first commander.
func commanderLabel(d deckReport) string {
	if len(d.Commanders) > 0 {
		return strings.Join(d.Commanders, " & ")
	}
	return d.Commander
}

// splitReasons breaks a multi-line ValidateCommander error into its individual
// messages, dropping the shared prefix line so the terminal report stays
// one-issue-per-line.
func splitReasons(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "commander deck invalid:") {
			continue
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

// renderText prints the human-readable report for every deck.
func renderText(w io.Writer, rep report) {
	for i, d := range rep.Decks {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "== %s (%s) ==\n", d.Name, d.Format)
		if label := commanderLabel(d); label != "" {
			fmt.Fprintf(w, "commander: %s\n", label)
		}
		fmt.Fprintf(w, "cards:   %d   resolved: %d   (%.1f%%)\n", d.Cards, d.Resolved, d.Percent)
		if d.Dropped > 0 {
			fmt.Fprintf(w, "sideboard cards dropped: %d\n", d.Dropped)
		}
		if len(d.Missing) > 0 {
			fmt.Fprintf(w, "missing from corpus (%d names):\n", len(d.Missing))
			for _, m := range d.Missing {
				fmt.Fprintf(w, "  %-40s x%d\n", m.Name, m.Count)
			}
		} else {
			fmt.Fprintln(w, "all cards present in corpus")
		}
		if d.CommanderOk != nil {
			if *d.CommanderOk {
				fmt.Fprintln(w, "commander eligible: yes")
			} else {
				fmt.Fprintln(w, "commander eligible: NO")
				for _, r := range d.CommanderWhy {
					fmt.Fprintf(w, "  - %s\n", r)
				}
			}
		}
		if d.Written {
			fmt.Fprintf(w, "written: %s\n", d.Path)
		}
	}
}
