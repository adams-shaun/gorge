package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/draft/limited"
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

// sheetFill is how much of one sheet an -only list keeps: its entries and
// collation weight, the share the list allows, and what restrictSpec did
// when that share could not fill the sheet's slot.
type sheetFill struct {
	sheet                 string
	cards, allowed        int
	weight, allowedWeight int
	// need is the most cards any variant draws from the sheet; playable is
	// the allowed entries the pack can deal (off spec.Unplayable).
	need, playable int
	// borrowed is the same-rarity allowed entries of other sheets added
	// when the sheet's own could not fill its slot; rarities is the
	// sheet's rarities, the ones borrowed from.
	borrowed int
	rarities string
	// dead: even with the borrowed entries the slot cannot fill, so the
	// sheet is empty and a variant drawing from it opens as another
	// (booster's dead-sheet fallback).
	dead bool
}

// restrictSpec is spec with every sheet cut to the entries the list
// allows, in their sheet order and with their collation weights, so a pack
// deals only listed names. A sheet whose allowed, playable entries are
// fewer than its largest slot draws (none, in the usual case) instead
// deals uniformly from those entries plus every allowed, playable entry of
// its rarities on the other sheets (sorted sheet order, one entry a name).
// A sheet still short of its slot is left empty, and booster opens a
// variant that draws from it as the heaviest variant that does not.
// Nothing here is random: the same spec and list give the same sheets.
func restrictSpec(spec setdata.Spec, only onlyList) (setdata.Spec, []sheetFill, error) {
	names := make([]string, 0, len(spec.Config.Sheets))
	for name := range spec.Config.Sheets {
		names = append(names, name)
	}
	sort.Strings(names)
	need := map[string]int{}
	for _, v := range spec.Config.Variants {
		for name, n := range v.Contents {
			need[name] = max(need[name], n)
		}
	}
	playable := func(c setdata.SheetCard) bool { return only.allows(c.Name) && !spec.Unplayable[c.Name] }
	out := spec
	out.Config.Sheets = make(map[string]setdata.Sheet, len(names))
	fills := make([]sheetFill, 0, len(names))
	for _, name := range names {
		sheet := spec.Config.Sheets[name]
		f := sheetFill{sheet: name, cards: len(sheet.Cards), weight: sheet.TotalWeight, need: need[name]}
		kept := make([]setdata.SheetCard, 0, len(sheet.Cards))
		rarities := map[limited.Rarity]bool{}
		have := map[string]bool{}
		for _, c := range sheet.Cards {
			rarities[c.Rarity] = true
			if !only.allows(c.Name) {
				continue
			}
			kept = append(kept, c)
			f.allowed++
			f.allowedWeight += c.Weight
			if playable(c) && !have[c.Name] {
				have[c.Name] = true
				f.playable++
			}
		}
		f.rarities = rarityString(rarities)
		if f.playable < f.need {
			// Borrow: uniform over the sheet's own playable entries and the
			// same-rarity playable entries of the other sheets.
			kept = kept[:0]
			for _, c := range sheet.Cards {
				if playable(c) {
					c.Weight = 1
					kept = append(kept, c)
				}
			}
			for _, other := range names {
				if other == name {
					continue
				}
				for _, c := range spec.Config.Sheets[other].Cards {
					if !rarities[c.Rarity] || !playable(c) || have[c.Name] {
						continue
					}
					have[c.Name] = true
					c.Weight = 1
					kept = append(kept, c)
					f.borrowed++
				}
			}
			if f.playable+f.borrowed < f.need {
				kept, f.dead = nil, true
			}
		}
		sheet.Cards = kept
		sheet.TotalWeight = 0
		for _, c := range kept {
			sheet.TotalWeight += c.Weight
		}
		out.Config.Sheets[name] = sheet
		fills = append(fills, f)
	}
	live := false
	for _, v := range spec.Config.Variants {
		ok := true
		for name := range v.Contents {
			if len(out.Config.Sheets[name].Cards) == 0 {
				ok = false
			}
		}
		live = live || ok
	}
	if !live {
		return setdata.Spec{}, fills, fmt.Errorf("-only: every %s pack variant draws from a sheet the list leaves empty", spec.Code)
	}
	return out, fills, nil
}

// rarityString spells a rarity set in C, U, R, M, L, S order.
func rarityString(rs map[limited.Rarity]bool) string {
	var b strings.Builder
	for _, r := range []limited.Rarity{limited.Common, limited.Uncommon, limited.Rare, limited.Mythic, limited.Land, limited.Special} {
		if rs[r] {
			b.WriteByte(byte(r))
		}
	}
	return b.String()
}

// printFills writes the -only report: the list's size, then one line per
// sheet with its fill rate (the allowed share of its entries and of its
// collation weight) and what was done for a sheet the list could not fill.
func printFills(out io.Writer, path string, only onlyList, fills []sheetFill) {
	fmt.Fprintf(out, "only: %s, %d names\n", path, len(only))
	for _, f := range fills {
		wpct := 0.0
		if f.weight > 0 {
			wpct = 100 * float64(f.allowedWeight) / float64(f.weight)
		}
		fmt.Fprintf(out, "  sheet %s: %d/%d entries allowed (%.1f%%, %.1f%% of weight), slot %d",
			f.sheet, f.allowed, f.cards, 100*float64(f.allowed)/float64(max(f.cards, 1)), wpct, f.need)
		switch {
		case f.dead:
			fmt.Fprintf(out, ", empty: %d own + %d borrowed %s entries cannot fill it, its variants open as another", f.playable, f.borrowed, f.rarities)
		case f.borrowed > 0 || f.playable < f.need:
			fmt.Fprintf(out, ", uniform over %d own + %d borrowed %s entries", f.playable, f.borrowed, f.rarities)
		}
		fmt.Fprintln(out)
	}
}
