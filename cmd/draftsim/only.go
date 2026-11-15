package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// onlyList is an -only file's names. A card matches by its full name or,
// for a split or double-faced card, by its first face.
type onlyList map[string]bool

// allows reports whether the list names card.
func (l onlyList) allows(card string) bool {
	if l[card] {
		return true
	}
	if i := strings.Index(card, " // "); i > 0 {
		return l[card[:i]]
	}
	return false
}

// readOnly reads an -only file: one card name a line. A line
// "name<TAB>true" or "name<TAB>false" (kgen -supported's output) names the
// card only when its second field is true. Blank lines and lines starting
// with # are skipped.
func readOnly(path string) (onlyList, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	l := onlyList{}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, flag, tab := strings.Cut(line, "\t")
		if tab {
			switch strings.TrimSpace(flag) {
			case "true":
			case "false":
				continue
			default:
				return nil, fmt.Errorf("%s:%d: want a name, or name<TAB>true|false, got %q", path, n, line)
			}
		}
		l[strings.TrimSpace(name)] = true
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(l) == 0 {
		return nil, fmt.Errorf("%s names no card", path)
	}
	return l, nil
}

// printFills writes the -only report: the list's size, then one line per
// sheet with its fill rate (the allowed share of its entries and of its
// collation weight) and what was done for a sheet the list could not fill.
func printFills(out io.Writer, path string, only onlyList, fills []setdata.SheetFill) {
	fmt.Fprintf(out, "only: %s, %d names\n", path, len(only))
	for _, f := range fills {
		wpct := 0.0
		if f.Weight > 0 {
			wpct = 100 * float64(f.AllowedWeight) / float64(f.Weight)
		}
		fmt.Fprintf(out, "  sheet %s: %d/%d entries allowed (%.1f%%, %.1f%% of weight), slot %d",
			f.Sheet, f.Allowed, f.Cards, 100*float64(f.Allowed)/float64(max(f.Cards, 1)), wpct, f.Need)
		switch {
		case f.Dead:
			fmt.Fprintf(out, ", empty: %d own + %d borrowed %s entries cannot fill it, its variants open as another", f.Playable, f.Borrowed, f.Rarities)
		case f.Borrowed > 0 || f.Playable < f.Need:
			fmt.Fprintf(out, ", uniform over %d own + %d borrowed %s entries", f.Playable, f.Borrowed, f.Rarities)
		}
		fmt.Fprintln(out)
	}
}
