package draftlab

import (
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
)

// loadSpec loads the set's booster spec at the collation. An empty
// collation means setdata's default (E5-a1's play-arena default, because
// the pick ratings are 17Lands Arena data).
func (l *Lab) loadSpec(set string, c setdata.Collation) (setdata.Spec, error) {
	if c == "" {
		c = setdata.DefaultCollation
	}
	spec, err := setdata.LoadCollation(set, c)
	if err != nil {
		return setdata.Spec{}, fmt.Errorf("draftlab: set %q collation %q: %w", set, c, err)
	}
	return spec, nil
}

// OpenPacks opens n packs of set as one seat's supply: out[i] is pack i's
// CardRefs, each pack from its own seeded booster stream. An empty
// collation means setdata's default.
func (l *Lab) OpenPacks(set string, c setdata.Collation, seed uint64, n int) ([][]limited.CardRef, error) {
	spec, err := l.loadSpec(set, c)
	if err != nil {
		return nil, err
	}
	pod, err := booster.OpenPod(spec, seed, 1, n, &booster.IDs{}, colourOfRegistry(l.reg))
	if err != nil {
		return nil, fmt.Errorf("draftlab: OpenPacks %s: %w", set, err)
	}
	return pod[0], nil
}

// OpenPod opens a draft pod's packs the way cmd/draftsim does:
// out[seat][round] is that seat's copy of that round's pack, every pack
// from its own seeded stream (booster's seat<<32|pack salt), IDs assigned
// once across the whole pod in seat-major order. Callers feed the result
// to Draft unchanged — packs from separate OpenPacks calls duplicate IDs
// and the draft refuses them.
func (l *Lab) OpenPod(set string, c setdata.Collation, seed uint64, seats, rounds int) ([][][]limited.CardRef, error) {
	spec, err := l.loadSpec(set, c)
	if err != nil {
		return nil, err
	}
	pod, err := booster.OpenPod(spec, seed, seats, rounds, &booster.IDs{}, colourOfRegistry(l.reg))
	if err != nil {
		return nil, fmt.Errorf("draftlab: OpenPod %s: %w", set, err)
	}
	return pod, nil
}

// SealedPool opens packs packs of set as one sealed pool: the packs
// flattened in pack order (pack i first), IDs unique across the pool.
func (l *Lab) SealedPool(set string, c setdata.Collation, seed uint64, packs int) ([]limited.CardRef, error) {
	opened, err := l.OpenPacks(set, c, seed, packs)
	if err != nil {
		return nil, err
	}
	total := 0
	for _, pack := range opened {
		total += len(pack)
	}
	pool := make([]limited.CardRef, 0, total)
	for _, pack := range opened {
		pool = append(pool, pack...)
	}
	return pool, nil
}

// CardNames lists the set's sheet-card universe at the collation — every
// distinct card name the booster sheets can put in a pack, sorted unique.
// An empty collation means setdata's default. These are the CSV columns
// (E5-c4) and the pack/pool shape a training program reports against.
func (l *Lab) CardNames(set string, c setdata.Collation) ([]string, error) {
	spec, err := l.loadSpec(set, c)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, sheet := range spec.Config.Sheets {
		for _, sc := range sheet.Cards {
			seen[sc.Name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// colourOfRegistry adapts the card corpus to booster's ColourOf: the
// card's colour identity as W/U/B/R/G bytes, in that order. A name the
// corpus does not carry, or a colourless card, has no colours. (The same
// adapter cmd/draftsim's event driver builds; the lab keeps its own copy
// because a cmd package is never imported.)
func colourOfRegistry(reg *cards.Registry) booster.ColourOf {
	const wubrg = "WUBRG"
	return func(name string) []byte {
		c, ok := reg.Lookup(name)
		if !ok {
			return nil
		}
		mask := c.ColourIdentity()
		if mask == 0 {
			return nil
		}
		out := make([]byte, 0, 5)
		for i := 0; i < len(wubrg); i++ {
			if mask&(1<<i) != 0 {
				out = append(out, wubrg[i])
			}
		}
		return out
	}
}
