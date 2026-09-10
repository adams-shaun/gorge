package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/adams-shaun/gorge/cards"
)

// parsedCard is one line of a plain-text decklist, already split into its
// count and card name.
type parsedCard struct {
	Name  string
	Count int
}

// parsedDeck is the outcome of parsing a plain-text decklist. The sideboard
// has already been dropped, the commander (if any) has been lifted into
// Commander, and Cards is the merged maindeck in first-appearance order.
type parsedDeck struct {
	Commander string
	Cards     []parsedCard
	// Sideboard is the count of sideboard card copies dropped from the list
	// (gorge has no sideboard concept), so the report can say how many.
	Sideboard int
}

// headerKeys is the set of bare section headers a plain-text export may
// carry. A line that is one of these words (optionally followed by a colon
// and trailing text) is a section boundary, not a card, and is handled
// specially rather than being parsed. Keys are lowercased, punctuation and
// interior spaces stripped.
var headerKeys = map[string]bool{
	"sideboard":     true,
	"sb":            true,
	"commander":     true,
	"commanders":    true,
	"main":          true,
	"maindeck":      true,
	"mainboard":     true,
	"deck":          true,
	"companion":     true,
	"maybeboard":    true,
	"lands":         true,
	"creatures":     true,
	"instants":      true,
	"sorceries":     true,
	"artifacts":     true,
	"enchantments":  true,
	"planeswalkers": true,
	"spells":        true,
}

// cardLineRe matches a count-prefixed card line: one or more leading digits,
// optionally followed by an "x" (so both "4 Lightning Bolt" and
// "1x Lightning Bolt" parse), then a card name. A line that does not match
// is not a card line.
var cardLineRe = regexp.MustCompile(`^([0-9]+)\s*x?\s+(.+)$`)

// trailingSetRe matches a trailing set / collector annotation such as
// "(2X2) 117", "(M20)" or "(2X2)". The paren body is a bare alphanumeric set
// code (never a card-name parenthetical) and the trailing collector number is
// optional. A space is required before the paren, so a card name that itself
// contains parentheses is left alone. It is anchored at the end and stripped
// repeatedly so a chain of annotations is removed.
var trailingSetRe = regexp.MustCompile(`\s+\([A-Za-z0-9]+\)(?:\s+[0-9*]+)?$`)

// parseDecklist parses a plain-text decklist into a parsedDeck.
//
// Accepted shapes: blank lines, `#`/`//` comment lines, count-first card
// lines with an optional "x" ("4 Lightning Bolt", "1x Lightning Bolt"),
// `Sideboard`/`Sideboard:` (whose cards are dropped — gorge has no sideboard
// concept and silently merging them makes a 75-card "60-card" deck), and a
// `Commander`/`Commander:` section whose (single) card becomes the commander.
// Set/collector annotations after a name are stripped, because the card NAME
// is what matters.
//
// A line that cannot be parsed into a (count, name) pair is an error naming
// the line, never a silent skip: guessing a card name is exactly the failure
// mode this tool exists to avoid.
func parseDecklist(raw []byte) (*parsedDeck, error) {
	d := &parsedDeck{}
	lines := strings.Split(string(raw), "\n")
	inSideboard := false
	expectCommander := false
	merged := map[string]int{} // normalised name -> index into d.Cards

	for i, rawLine := range lines {
		line := stripComment(rawLine)
		if line == "" {
			continue
		}
		if key, ok := headerKey(line); ok {
			switch key {
			case "sideboard", "sb", "companion", "maybeboard":
				// These sections are not maindeck: in this engine only the
				// maindeck is dealt, so drop them.
				inSideboard = true
			case "commander", "commanders":
				inSideboard = false
				expectCommander = true
			default:
				// main / maindeck / mainboard / deck, and the category
				// headers (lands, creatures, ...) an export may interleave:
				// they mark the start of (or a subdivision of) the maindeck,
				// so stop dropping and keep parsing cards.
				inSideboard = false
			}
			continue
		}
		if inSideboard {
			// Sideboard cards are dropped, not merged into the deck; we only
			// count them so the report can say how many were discarded. A
			// sideboard line that does not parse is not an error (it is not
			// part of the deck we are building) -- it just is not counted.
			if m := cardLineRe.FindStringSubmatch(line); m != nil {
				var n int
				for _, r := range m[1] {
					n = n*10 + int(r-'0')
				}
				d.Sideboard += n
			}
			continue
		}

		pc, err := parseCardLine(line)
		if err != nil {
			if expectCommander {
				// A commander section often prints the commander as a bare name
				// with no count. That is the one shape we take count-less: a
				// commander is a single card.
				pc = parsedCard{Name: stripAnnotation(line), Count: 1}
				if pc.Name == "" {
					return nil, fmt.Errorf("line %d: %w", i+1, err)
				}
			} else {
				return nil, fmt.Errorf("line %d: %w (decklist line was %q)", i+1, err, line)
			}
		}
		if pc.Count <= 0 {
			return nil, fmt.Errorf("line %d: non-positive count for %q", i+1, pc.Name)
		}
		if pc.Name == "" {
			return nil, fmt.Errorf("line %d: empty card name", i+1)
		}

		if expectCommander {
			if d.Commander == "" {
				d.Commander = pc.Name
			}
			expectCommander = false
		}
		addCard(d, pc, merged)
	}

	if len(d.Cards) == 0 {
		return nil, fmt.Errorf("decklist has no cards")
	}
	reconcileCommander(d)
	return d, nil
}

// reconcileCommander enforces that the deck's commander is exactly one
// copy. Some exports print the commander both in a Commander section and
// again in the maindeck 100, and addCard would otherwise merge those into a
// count of two; the commander is always a single singleton copy (CR 903.4),
// so the first occurrence wins with count 1 and any duplicate is dropped. A
// commander declared in a header but missing from the list is inserted at the
// front. It is a no-op for a deck with no commander.
func reconcileCommander(d *parsedDeck) {
	if d.Commander == "" {
		return
	}
	key := cards.NormalizeName(d.Commander)
	first := -1
	for i, c := range d.Cards {
		if cards.NormalizeName(c.Name) == key {
			first = i
			break
		}
	}
	if first == -1 {
		d.Cards = append([]parsedCard{{Name: d.Commander, Count: 1}}, d.Cards...)
		return
	}
	out := d.Cards[:0]
	for i, c := range d.Cards {
		if i == first {
			out = append(out, parsedCard{Name: d.Commander, Count: 1})
			continue
		}
		if cards.NormalizeName(c.Name) == key {
			continue
		}
		out = append(out, c)
	}
	d.Cards = out
}

// headerKey reports whether line is a bare section header and, if so, its
// canonical key. It returns ok=false for anything else (including a card
// line, whose key is a count-prefixed word and is not in headerKeys).
func headerKey(line string) (string, bool) {
	k := strings.ToLower(strings.TrimSpace(line))
	k = strings.TrimSuffix(k, ":")
	k = strings.TrimSpace(k)
	k = strings.ReplaceAll(k, " ", "")
	k = strings.ReplaceAll(k, "-", "")
	if headerKeys[k] {
		return k, true
	}
	return "", false
}

// stripComment removes a leading `#` or `//` comment. A comment line is not a
// card; the empty string means "not a card line", and parseDecklist skips it.
func stripComment(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
		return ""
	}
	return line
}

// parseCardLine splits one count-first card line into its count and (set
// annotation already stripped) card name.
func parseCardLine(line string) (parsedCard, error) {
	m := cardLineRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return parsedCard{}, fmt.Errorf("cannot parse %q into a (count, name) pair", line)
	}
	var count int
	for _, r := range m[1] {
		count = count*10 + int(r-'0')
	}
	name := strings.TrimSpace(stripAnnotation(m[2]))
	if name == "" {
		return parsedCard{}, fmt.Errorf("missing card name in %q", line)
	}
	return parsedCard{Name: name, Count: count}, nil
}

// stripAnnotation removes trailing set/collector annotations from a name:
// "Lightning Bolt (2X2) 117" -> "Lightning Bolt". The card NAME is what the
// corpus is keyed by, so the annotation is discard-only. It loops because an
// export may append several annotations.
func stripAnnotation(s string) string {
	for {
		loc := trailingSetRe.FindStringIndex(s)
		if loc == nil || loc[1] != len(s) {
			break
		}
		s = strings.TrimSpace(s[:loc[0]])
	}
	return strings.TrimSpace(s)
}

// addCard appends pc, merging counts when the same normalised card name
// already appears (which a split or multi-line list can produce), keeping
// first-appearance order.
func addCard(d *parsedDeck, pc parsedCard, merged map[string]int) {
	k := cards.NormalizeName(pc.Name)
	if j, ok := merged[k]; ok {
		d.Cards[j].Count += pc.Count
		return
	}
	merged[k] = len(d.Cards)
	d.Cards = append(d.Cards, pc)
}
