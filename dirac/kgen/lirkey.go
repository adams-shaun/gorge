package kgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// LIRKeys returns each card's evidence key: the first 16 lowercase hex
// characters of sha256 over its dumpLIR CARD block, keyed by card name (the
// text of the CARD line before " token="). A block starts at a line beginning
// "CARD " and runs through every following line up to, and excluding, the
// next line beginning "CARD ", "| " or "#". The first block of a name wins.
func LIRKeys(cards []*lCard) map[string]string {
	var buf bytes.Buffer
	for _, c := range cards {
		dumpLIR(&buf, c)
	}
	keys := map[string]string{}
	var name string
	var block []byte
	flush := func() {
		if name == "" {
			return
		}
		if _, dup := keys[name]; !dup {
			sum := sha256.Sum256(block)
			keys[name] = hex.EncodeToString(sum[:])[:16]
		}
		name, block = "", nil
	}
	for _, line := range strings.SplitAfter(buf.String(), "\n") {
		switch {
		case strings.HasPrefix(line, "CARD "):
			flush()
			rest := line[len("CARD "):]
			if i := strings.Index(rest, " token="); i >= 0 {
				rest = rest[:i]
			}
			name, block = rest, append(block[:0:0], line...)
		case strings.HasPrefix(line, "| "), strings.HasPrefix(line, "#"):
			flush()
		case name != "":
			block = append(block, line...)
		}
	}
	flush()
	return keys
}

// LIRKeysFor loads the named cards from the corpus as -card-list does, lowers
// them as -lir does (cards, then tokens) and returns their LIRKeys.
func LIRKeysFor(cardsDir string, names []string) (map[string]string, error) {
	p, err := loadCardNames(append([]string(nil), names...), cardsDir)
	if err != nil {
		return nil, err
	}
	lw := lowerPool(p)
	return LIRKeys(append(append([]*lCard(nil), lw.cards...), lw.tokens...)), nil
}

// sortedKeys returns the map's keys in order.
func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
