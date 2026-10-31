// Package booster opens seeded packs from a draft/limited/setdata Spec's MTGJSON
// collation config (draft-events spec §4.3, E5-a1): it picks a pack variant
// by weight, fills each contents sheet by the cards' collation weights,
// never puts the same card name twice in one pack, honours a sheet's
// allow_duplicates and fixed flags, and colour-balances the sheets marked
// balance_colors.
//
// The package is pure: its only randomness is the *rand.Rand the caller
// seeds, its only inputs besides that are the spec and a colour lookup the
// caller supplies, and it imports nothing but the standard library and its
// draft/limited/ siblings.
package booster

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// ColourOf reports a card's colours as W/U/B/R/G bytes, in that order. A
// mono-coloured card returns exactly one byte, a colourless card an empty
// (or nil) slice, a multicoloured card one byte per colour. The lookup is a
// function so the package never has to touch the card corpus itself.
type ColourOf func(name string) []byte

// IDSource hands out the card IDs a pack's CardRefs carry. The caller
// supplies one allocator per event so IDs stay unique across every pack it
// opens.
type IDSource interface {
	Next() string
}

// IDs is the simple sequential allocator the event layer uses: zero value
// ready, first Next is "c1".
type IDs struct{ next int }

// Next returns the next sequential ID ("c1", "c2", ...).
func (ids *IDs) Next() string {
	ids.next++
	return fmt.Sprintf("c%d", ids.next)
}

// Count reports how many IDs the allocator has handed out.
func (ids *IDs) Count() int { return ids.next }

// wubrg spells the five colours in the order colourOf's bytes and every
// deterministic tie-break in this package use.
const wubrg = "WUBRG"

// Open fills one pack from spec. rng drives every draw; ids assigns the
// CardRef IDs in fill order; colourOf backs the colour balance of the
// sheets whose BalanceColors is set and whose slot draws at least 5 cards
// (Open refuses to run when such a sheet exists and colourOf is nil).
//
// The variant is picked by weight first, then the variant's contents fill
// in sorted sheet-name order. A draw resamples any name already in the
// pack, so a pack never carries the same name twice; a sheet draws without
// replacement within the pack unless its AllowDuplicates is set. A slot
// that cannot be filled under those rules fails Open with an error.
func Open(spec setdata.Spec, rng *rand.Rand, ids IDSource, colourOf ColourOf) ([]limited.CardRef, error) {
	pack, _, err := openPack(spec, rng, ids, colourOf)
	return pack, err
}

// openPack is Open with the picked variant index returned, so the stats
// tests can measure the variant distribution the public API only implies.
func openPack(spec setdata.Spec, rng *rand.Rand, ids IDSource, colourOf ColourOf) ([]limited.CardRef, int, error) {
	if rng == nil {
		return nil, -1, errors.New("booster: Open needs a seeded *rand.Rand")
	}
	if ids == nil {
		return nil, -1, errors.New("booster: Open needs a non-nil ID allocator")
	}
	if colourOf == nil {
		// Sorted: the error text must not depend on the map's iteration
		// order when several sheets balance colours.
		balancing := make([]string, 0, len(spec.Config.Sheets))
		for name, sheet := range spec.Config.Sheets {
			if sheet.BalanceColors {
				balancing = append(balancing, name)
			}
		}
		sort.Strings(balancing)
		for _, name := range balancing {
			return nil, -1, fmt.Errorf("booster: sheet %q balances colours but no ColourOf was supplied", name)
		}
	}
	v := pickVariant(spec.Config, rng)
	variant := spec.Config.Variants[v]
	pack := make([]limited.CardRef, 0, spec.PackSize)
	st := &drawState{inPack: map[string]bool{}, drawn: map[string]bool{}}
	for _, name := range sortedSheetNames(variant.Contents) {
		count := variant.Contents[name]
		sheet, ok := spec.Config.Sheets[name]
		if !ok {
			return nil, -1, fmt.Errorf("booster: %s: variant names missing sheet %q (spec is unvalidated?)", spec.Code, name)
		}
		start := len(pack)
		if sheet.Fixed {
			fixed, err := drawFixed(sheet, count, st)
			if err != nil {
				return nil, -1, fmt.Errorf("booster: %s sheet %q: %w", spec.Code, name, err)
			}
			for _, sc := range fixed {
				pack = append(pack, ref(ids, sc))
				st.take(sc)
			}
		} else {
			for n := 0; n < count; n++ {
				sc, err := drawWeighted(rng, sheet, st)
				if err != nil {
					return nil, -1, fmt.Errorf("booster: %s sheet %q: %w", spec.Code, name, err)
				}
				pack = append(pack, ref(ids, sc))
				st.take(sc)
			}
		}
		if sheet.BalanceColors && count >= 5 {
			balanceColours(rng, pack[start:], sheet.Cards, colourOf, st.inPack)
		}
	}
	return pack, v, nil
}

// ref builds the pack entry one sheet card becomes. Set and number are the
// printing's own (a special-guest card carries its SPG set), while the
// CardRef's event ID comes from the caller's allocator.
func ref(ids IDSource, sc setdata.SheetCard) limited.CardRef {
	return limited.CardRef{
		ID:     ids.Next(),
		Name:   sc.Name,
		Set:    sc.Set,
		Number: sc.Number,
		Rarity: sc.Rarity,
	}
}

// drawState tracks what one pack already holds: the card names (never
// repeated, whatever the sheets say) and the card UUIDs (not repeated
// unless the sheet allows duplicates).
type drawState struct {
	inPack map[string]bool
	drawn  map[string]bool
}

// take records one card as packed.
func (st *drawState) take(sc setdata.SheetCard) {
	st.inPack[sc.Name] = true
	st.drawn[sc.UUID] = true
}

// packed reports whether the card can still be drawn for this pack: its
// name is not in the pack, and — unless the sheet allows duplicates — its
// UUID is not either.
func (st *drawState) packed(sc setdata.SheetCard, allowDuplicates bool) bool {
	return st.inPack[sc.Name] || (!allowDuplicates && st.drawn[sc.UUID])
}

// pickVariant picks a pack variant by weight: one IntN over the config's
// total weight, walked across the variants in their listed order.
func pickVariant(cfg setdata.Config, rng *rand.Rand) int {
	n := rng.IntN(cfg.TotalWeight)
	for i := range cfg.Variants {
		if n < cfg.Variants[i].Weight {
			return i
		}
		n -= cfg.Variants[i].Weight
	}
	// Unreachable for a validated config (the weights sum to TotalWeight);
	// the last variant keeps the function total anyway.
	return len(cfg.Variants) - 1
}

// sortedSheetNames lists a variant's contents keys, sorted: no map
// iteration order may reach the pack.
func sortedSheetNames(contents map[string]int) []string {
	names := make([]string, 0, len(contents))
	for name := range contents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// drawFixed takes a fixed sheet's cards as listed, in order, skipping any
// name already in the pack until the slot's count is met. A fixed sheet
// with fewer unpacked cards than the slot needs is a spec defect and fails.
func drawFixed(sheet setdata.Sheet, count int, st *drawState) ([]setdata.SheetCard, error) {
	out := make([]setdata.SheetCard, 0, count)
	for _, sc := range sheet.Cards {
		if len(out) == count {
			break
		}
		if st.inPack[sc.Name] {
			continue
		}
		out = append(out, sc)
	}
	if len(out) < count {
		return nil, errors.New("fixed sheet has fewer unpacked cards than the slot draws")
	}
	return out, nil
}

// drawWeighted draws one card from a sheet by its collation weight, over
// the cards the pack can still take (drawState.packed). It fails only when
// no drawable card is left.
func drawWeighted(rng *rand.Rand, sheet setdata.Sheet, st *drawState) (setdata.SheetCard, error) {
	total := 0
	for _, sc := range sheet.Cards {
		if !st.packed(sc, sheet.AllowDuplicates) {
			total += sc.Weight
		}
	}
	if total == 0 {
		return setdata.SheetCard{}, errors.New("no drawable card is left")
	}
	n := rng.IntN(total)
	for _, sc := range sheet.Cards {
		if st.packed(sc, sheet.AllowDuplicates) {
			continue
		}
		if n < sc.Weight {
			return sc, nil
		}
		n -= sc.Weight
	}
	return setdata.SheetCard{}, errors.New("weighted draw fell off the sheet (impossible for a validated sheet)")
}

// balanceColours tops up the colour coverage of one balance_colors sheet's
// freshly drawn slot (spec §4.3): while some W/U/B/R/G colour has no
// mono-coloured card in the slot, it replaces one card with an unpacked mono
// card of the missing colour, up to five tries. The replaced card is a
// colourless or multicoloured one when the slot has any (swapping a mono
// card for a mono card of another colour can never reach five covered
// colours, and dropping the victim costs no coverage, so each try strictly
// gains a colour); when the whole slot is mono, it is a card of the
// most-represented colour as the spec spells it — with every colour covered
// but one and seven mono cards in play, that colour holds at least two, so
// the trade empties nothing. Every choice is deterministic (missing colour,
// most-represented colour and replaced position all break ties in
// WUBRG/slot order); only the replacement card is random. A colour the
// sheet has no unpacked mono card of stops the balancing quietly — the pack
// stays as drawn.
func balanceColours(rng *rand.Rand, slot []limited.CardRef, sheet []setdata.SheetCard, colourOf ColourOf, inPack map[string]bool) {
	for tries := 0; tries < 5; tries++ {
		mono := [5]int{}
		for i := range slot {
			if c := monoColour(colourOf(slot[i].Name)); c >= 0 {
				mono[c]++
			}
		}
		missing, best := -1, 0
		for c := 0; c < 5; c++ {
			if mono[c] == 0 && missing < 0 {
				missing = c
			}
			if mono[c] > mono[best] {
				best = c
			}
		}
		if missing < 0 {
			return
		}
		// Candidates: unpacked mono cards of the missing colour, in sheet order.
		var cand []int
		for i, sc := range sheet {
			if !inPack[sc.Name] && monoColour(colourOf(sc.Name)) == missing {
				cand = append(cand, i)
			}
		}
		if len(cand) == 0 {
			return
		}
		// Victim: the first non-mono card — losing it costs no colour
		// coverage and each try then strictly gains a colour. When the whole
		// slot is mono, the spec's most-represented colour, which with six
		// mono cards left after one trade holds at least two copies.
		pos := -1
		for i := range slot {
			if monoColour(colourOf(slot[i].Name)) < 0 {
				pos = i
				break
			}
		}
		if pos < 0 {
			if mono[best] < 2 {
				return // nothing to trade away without emptying a colour
			}
			for i := range slot {
				if monoColour(colourOf(slot[i].Name)) == best {
					pos = i
					break
				}
			}
		}
		if pos < 0 {
			return
		}
		sc := sheet[cand[rng.IntN(len(cand))]]
		delete(inPack, slot[pos].Name)
		slot[pos] = limited.CardRef{ID: slot[pos].ID, Name: sc.Name, Set: sc.Set, Number: sc.Number, Rarity: sc.Rarity}
		inPack[sc.Name] = true
	}
}

// monoColour returns the WUBRG index of cs's single colour, or -1 when cs is
// not exactly one of the five colours.
func monoColour(cs []byte) int {
	if len(cs) != 1 {
		return -1
	}
	for i := 0; i < len(wubrg); i++ {
		if wubrg[i] == cs[0] {
			return i
		}
	}
	return -1
}
