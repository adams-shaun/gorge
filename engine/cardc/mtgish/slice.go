package mtgish

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// legacyDeckNames is a copy of testkit/testutil/decks.go's legacyDeckNames,
// the 12 60-card Legacy decks (testkit is not imported).
var legacyDeckNames = []string{
	"death-n-taxes",
	"dimir-tempo",
	"eldrazi-stompy",
	"mono-black-aggro",
	"mono-blue-tempo",
	"mono-green-stompy",
	"mono-red-goblins",
	"the-epic-storm",
	"tron",
	"ur-delver",
	"uw-control",
	"uw-tempo",
}

// SliceNames lists the card names of a slice, sorted and deduplicated: fdn
// and blb (dirac/cardset/pools/<slice>/cards.txt), decks (every repo deck),
// legacy (the 12 Legacy decks), or all (nil: every card). root is the module
// root.
func SliceNames(root, slice string) ([]string, error) {
	var names []string
	switch slice {
	case "all":
		return nil, nil
	case "fdn", "blb":
		f, err := os.Open(filepath.Join(root, "dirac", "cardset", "pools", slice, "cards.txt"))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if n := strings.TrimSpace(sc.Text()); n != "" && !strings.HasPrefix(n, "#") {
				names = append(names, n)
			}
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	case "decks":
		files, err := filepath.Glob(filepath.Join(root, "testkit", "testutil", "decks", "*.json"))
		if err != nil {
			return nil, err
		}
		for _, p := range files {
			ns, err := deckCardNames(p)
			if err != nil {
				return nil, err
			}
			names = append(names, ns...)
		}
	case "legacy":
		for _, d := range legacyDeckNames {
			ns, err := deckCardNames(filepath.Join(root, "testkit", "testutil", "decks", d+".json"))
			if err != nil {
				return nil, err
			}
			names = append(names, ns...)
		}
	default:
		return nil, fmt.Errorf("mtgish: unknown slice %q (fdn, blb, legacy, decks, all)", slice)
	}
	sort.Strings(names)
	out := names[:0]
	for i, n := range names {
		if i == 0 || n != names[i-1] {
			out = append(out, n)
		}
	}
	return out, nil
}

func deckCardNames(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d struct {
		Cards []struct {
			Name string `json:"name"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	out := make([]string, 0, len(d.Cards))
	for _, c := range d.Cards {
		out = append(out, c.Name)
	}
	return out, nil
}
