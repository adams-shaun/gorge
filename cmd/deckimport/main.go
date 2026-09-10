// Command deckimport turns a plain-text decklist into a repo deck file
// (the {name, count} JSON that internal/testutil/decks/*.json already holds)
// and reports, card by card, what the corpus is missing — the gap that
// otherwise only surfaces at runtime, mid-deal, when the deck cannot be
// dealt.
//
// The near-universal export shape is one card per line, count first:
//
//	4 Lightning Bolt
//	4 Monastery Swiftspear
//	20 Mountain
//
// Real exports add sideboards, a Commander header, "1x" counts, set/collector
// annotations, comments and stray whitespace. All are handled; see parse.go
// for the exact grammar. Sideboard cards are dropped (gorge has no sideboard
// concept — silently merging them makes a 75-card "60-card" deck), a
// Commander section becomes the top-level "commander" field, and a line that
// cannot be parsed into a (count, name) pair is an error naming the line.
//
// Existence is checked through the one existing resolver, deck.File.Resolve /
// cards.Registry.Lookup, which normalises punctuation, case and split-card
// "//" already, so "a card the corpus does not have" is never a punctuation
// mismatch. The much harder question — whether a present card's Forge script
// is fully implemented by the engine — is deliberately out of scope and is
// noted as a follow-up in the report.
//
// The coverage report (§ second half) is the point: for every deck it prints
// which names have no corpus card at all, the resolved count and percentage,
// and, for a Commander list, whether the commander is commander-eligible. A
// -json flag emits the same findings as structured output, and the command
// exits non-zero when any deck has unresolvable cards so it can gate a batch
// import; -force writes the deck file anyway for the case where someone wants
// a mostly-present deck on disk to look at.
//
// Usage:
//
//	deckimport -in list.txt -out decks/my-deck.json
//	deckimport -in list.txt -out my-commander.json -json -force
//	cat list.txt | deckimport -in - -out deck.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/deck"
)

func main() {
	var in multiFlag
	flag.Var(&in, "in", "plain-text decklist to convert (repeatable; `-` reads stdin)")
	out := flag.String("out", "", "deck file to write (one input only)")
	corpus := flag.String("corpus", ".cards", "directory holding the card corpus (default .cards)")
	name := flag.String("name", "", "deck name (default: input file stem, or \"Imported Deck\")")
	format := flag.String("format", "", "deck format (default: `commander` when a commander is present, else `custom`)")
	archetype := flag.String("archetype", "", "archetype label to record in the deck file")
	notes := flag.String("notes", "", "notes to record in the deck file (default: an auto-generated coverage note)")
	jsonOut := flag.Bool("json", false, "emit the report as JSON instead of text")
	force := flag.Bool("force", false, "write the deck file even when it has unresolvable cards (still exits non-zero)")
	flag.Parse()

	if len(in) == 0 {
		fmt.Fprintln(os.Stderr, "deckimport: no input; use -in <file> (or `-in -` for stdin)")
		flag.Usage()
		os.Exit(2)
	}
	if len(in) > 1 && *out != "" {
		fmt.Fprintln(os.Stderr, "deckimport: -out applies to a single -in; multiple inputs report but write nothing")
		os.Exit(2)
	}

	var rep report
	var hadError bool

	for _, input := range in {
		d, err := convert(input, *corpus, *name, *format, *out, *force, *archetype, *notes)
		if err != nil {
			fmt.Fprintln(os.Stderr, "deckimport:", err)
			hadError = true
			continue
		}
		rep.Decks = append(rep.Decks, d)
		if !d.Ok {
			hadError = true
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintln(os.Stderr, "deckimport:", err)
			os.Exit(1)
		}
	} else {
		renderText(os.Stdout, rep)
	}

	if hadError {
		os.Exit(1)
	}
}

// convert reads one decklist, analyses it against the corpus, and writes the
// deck file when asked to. It returns the per-deck findings; a returned error
// means the input could not be converted at all (and was not written).
func convert(input, corpusDir, name, format, out string, force bool, archetype, notes string) (deckReport, error) {
	raw, err := readInput(input)
	if err != nil {
		return deckReport{}, err
	}
	p, err := parseDecklist(raw)
	if err != nil {
		return deckReport{}, fmt.Errorf("%s: %w", inputLabel(input), err)
	}

	r, err := cards.OpenCorpus(corpusDir)
	if err != nil {
		return deckReport{}, fmt.Errorf("corpus: %w", err)
	}

	format = resolveFormat(format, p.Commander)
	name = resolveName(name, input)

	dr := analyzeDeck(inputLabel(input), name, format, p, r)

	// Decide whether to write the deck file. It is written when -out names a
	// destination and there is nothing worth refusing: no missing cards, or
	// -force explicitly asked for the file anyway.
	write := out != "" && (dr.Ok || force)
	if write {
		if err := writeDeckFile(out, dr, p, format, archetype, notes); err != nil {
			return dr, err
		}
		dr.Path = out
		dr.Written = true
	}
	return dr, nil
}

// writeDeckFile marshals and writes the repo deck-file shape, then verifies it
// reads back through deck.Parse so the file is guaranteed loadable.
func writeDeckFile(path string, dr deckReport, p *parsedDeck, format, archetype, notes string) error {
	f := deckFile{
		Name:      dr.Name,
		Format:    format,
		Commander: p.Commander,
		Archetype: archetype,
		Notes:     notes,
		Cards:     toEntries(p.Cards),
	}
	if f.Notes == "" {
		f.Notes = autoNote(dr)
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	raw = append(raw, '\n')

	// Round-trip through the deck package's own parser so we never write a
	// file the match host would reject. (deck.Parse ignores archetype/notes;
	// it reads the {name, count} card list this file carries.)
	if _, err := deck.Parse(raw); err != nil {
		return fmt.Errorf("refusing to write a deck file that does not re-parse: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// autoNote writes a short coverage note so a deck written with -force records
// what it is missing on disk, not only in the terminal report.
func autoNote(dr deckReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Imported from %s. %d/%d cards resolved (%.1f%%).", dr.Input, dr.Resolved, dr.Cards, dr.Percent)
	if len(dr.Missing) > 0 {
		var names []string
		for _, m := range dr.Missing {
			names = append(names, fmt.Sprintf("%s (%d)", m.Name, m.Count))
		}
		b.WriteString(" Missing from corpus: " + strings.Join(names, ", ") + ".")
	}
	return b.String()
}

// resolveFormat picks the deck format: an explicit -format wins, else a deck
// that names a commander is "commander", else "custom".
func resolveFormat(format, commander string) string {
	if format != "" {
		return format
	}
	if commander != "" {
		return "commander"
	}
	return "custom"
}

// resolveName picks the deck name: an explicit -name wins, else the input
// file's stem, else "Imported Deck".
func resolveName(name, input string) string {
	if name != "" {
		return name
	}
	if input == "-" {
		return "Imported Deck"
	}
	return deck.Stem(input)
}

// readInput reads a decklist from a file path, or from stdin for "-".
func readInput(input string) ([]byte, error) {
	if input == "-" {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return raw, nil
	}
	raw, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", input, err)
	}
	return raw, nil
}

// inputLabel is how an input is named in the report: "-" for stdin, else the
// path as given.
func inputLabel(input string) string {
	if input == "-" {
		return "stdin"
	}
	return input
}

// multiFlag lets -in be repeated.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}
