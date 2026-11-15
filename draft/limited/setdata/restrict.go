package setdata

import (
	"fmt"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/draft/limited"
)

// SheetFill is how much of one sheet an allow-list keeps: its entries and
// collation weight, the share the list allows, and what Restrict did
// when that share could not fill the sheet's slot.
type SheetFill struct {
	Sheet                 string
	Cards, Allowed        int
	Weight, AllowedWeight int
	// Need is the most cards any variant draws from the sheet; Playable is
	// the allowed entries the pack can deal (off spec.Unplayable).
	Need, Playable int
	// Borrowed is the same-rarity allowed entries of other sheets added
	// when the sheet's own could not fill its slot; Rarities is the
	// sheet's rarities, the ones borrowed from.
	Borrowed int
	Rarities string
	// Dead: even with the borrowed entries the slot cannot fill, so the
	// sheet is empty and a variant drawing from it opens as another
	// (booster's dead-sheet fallback).
	Dead bool
}

// Restrict returns spec with every sheet cut to the entries that allows
// accepts, in their sheet order and with their collation weights, so a pack
// deals only listed names. A sheet whose allowed, playable entries are
// fewer than its largest slot draws (none, in the usual case) instead
// deals uniformly from those entries plus every allowed, playable entry of
// its rarities on the other sheets (sorted sheet order, one entry a name).
// A sheet still short of its slot is left empty, and booster opens a
// variant that draws from it as the heaviest variant that does not.
// Nothing here is random: the same spec and allows give the same sheets.
// spec is not modified; the returned []SheetFill has one row per sheet, in
// sorted sheet-name order, including when Restrict returns an error (every
// variant draws from an emptied sheet).
func Restrict(spec Spec, allows func(name string) bool) (Spec, []SheetFill, error) {
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
	playable := func(c SheetCard) bool { return allows(c.Name) && !spec.Unplayable[c.Name] }
	out := spec
	out.Config.Sheets = make(map[string]Sheet, len(names))
	fills := make([]SheetFill, 0, len(names))
	for _, name := range names {
		sheet := spec.Config.Sheets[name]
		f := SheetFill{Sheet: name, Cards: len(sheet.Cards), Weight: sheet.TotalWeight, Need: need[name]}
		kept := make([]SheetCard, 0, len(sheet.Cards))
		rarities := map[limited.Rarity]bool{}
		have := map[string]bool{}
		for _, c := range sheet.Cards {
			rarities[c.Rarity] = true
			if !allows(c.Name) {
				continue
			}
			kept = append(kept, c)
			f.Allowed++
			f.AllowedWeight += c.Weight
			if playable(c) && !have[c.Name] {
				have[c.Name] = true
				f.Playable++
			}
		}
		f.Rarities = rarityString(rarities)
		if f.Playable < f.Need {
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
					f.Borrowed++
				}
			}
			if f.Playable+f.Borrowed < f.Need {
				kept, f.Dead = nil, true
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
		return Spec{}, fills, fmt.Errorf("-only: every %s pack variant draws from a sheet the list leaves empty", spec.Code)
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
