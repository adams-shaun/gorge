package booster

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// promoSaltTag sets bits 40 and up of every promo stream's salt, so no promo
// stream shares a pack stream (podSalt is seat<<32|pack, below bit 40 for
// seat < 256).
const promoSaltTag = 0x50524F

// The promo rarity weights: the classic rare sheet prints each rare twice and
// each mythic once; Wizards publishes no prerelease promo odds.
const (
	promoRareWeight   = 2
	promoMythicWeight = 1
)

func promoSalt(seat, k int) uint64 {
	return promoSaltTag<<40 | uint64(uint16(seat))<<16 | uint64(uint16(k))
}

// OpenPromo draws one prerelease promo: a rare or mythic of the set, weighted
// promoRareWeight:promoMythicWeight, its CardRef ID from ids.
func OpenPromo(spec setdata.Spec, rng *rand.Rand, ids IDSource) (limited.CardRef, error) {
	universe, err := promoUniverse(spec)
	if err != nil {
		return limited.CardRef{}, err
	}
	total := 0
	for _, sc := range universe {
		total += promoWeight(sc)
	}
	pick := rng.IntN(total)
	for _, sc := range universe {
		pick -= promoWeight(sc)
		if pick < 0 {
			return ref(ids, sc), nil
		}
	}
	return limited.CardRef{}, fmt.Errorf("booster: set %s promo draw ran off its universe", spec.Code)
}

func promoWeight(sc setdata.SheetCard) int {
	if sc.Rarity == limited.Mythic {
		return promoMythicWeight
	}
	return promoRareWeight
}

// OpenPrereleasePod opens a sealed pod with promos: out[seat] is OpenPod's
// packs boosters (identical, IDs included) followed by promos one-card promo
// packs, each from rand.NewPCG(seed, promoSalt(seat, k)), their IDs drawn
// after every booster's, in seat order. promos 0 is exactly OpenPod.
func OpenPrereleasePod(spec setdata.Spec, seed uint64, seats, packs, promos int, ids IDSource, colourOf ColourOf) ([][][]limited.CardRef, error) {
	if promos < 0 {
		return nil, fmt.Errorf("booster: OpenPrereleasePod needs promos >= 0, got %d", promos)
	}
	out, err := OpenPod(spec, seed, seats, packs, ids, colourOf)
	if err != nil {
		return nil, err
	}
	for seat := range out {
		for k := 0; k < promos; k++ {
			rng := rand.New(rand.NewPCG(seed, promoSalt(seat, k)))
			c, err := OpenPromo(spec, rng, ids)
			if err != nil {
				return nil, fmt.Errorf("booster: pod seat %d promo %d: %w", seat, k, err)
			}
			out[seat] = append(out[seat], []limited.CardRef{c})
		}
	}
	return out, nil
}

// promoUniverse is the cards a promo can be: the playable (not
// spec.Unplayable) rare and mythic sheet cards of the spec's own set (every rare and mythic when none carries the set
// code), one printing per name (the lowest collector number), sorted by name.
func promoUniverse(spec setdata.Spec) ([]setdata.SheetCard, error) {
	names := make([]string, 0, len(spec.Config.Sheets))
	for n := range spec.Config.Sheets {
		names = append(names, n)
	}
	sort.Strings(names)
	var own, all []setdata.SheetCard
	for _, n := range names {
		for _, sc := range spec.Config.Sheets[n].Cards {
			if sc.Rarity != limited.Rare && sc.Rarity != limited.Mythic {
				continue
			}
			if spec.Unplayable[sc.Name] {
				continue // a promo has no sheet to substitute from
			}
			all = append(all, sc)
			if strings.EqualFold(sc.Set, spec.Code) {
				own = append(own, sc)
			}
		}
	}
	cands := own
	if len(cands) == 0 {
		cands = all
	}
	best := map[string]setdata.SheetCard{}
	for _, sc := range cands {
		if cur, ok := best[sc.Name]; !ok || numberLess(sc.Number, cur.Number) {
			best[sc.Name] = sc
		}
	}
	if len(best) == 0 {
		return nil, fmt.Errorf("booster: set %s has no rare or mythic sheet card for a promo", spec.Code)
	}
	out := make([]setdata.SheetCard, 0, len(best))
	for _, sc := range best {
		out = append(out, sc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// numberLess orders collector numbers: the leading decimal digits as
// integers (no leading digits sorts after any number), then the whole string.
func numberLess(a, b string) bool {
	na, oka := leadingNumber(a)
	nb, okb := leadingNumber(b)
	switch {
	case oka && okb && na != nb:
		return na < nb
	case oka != okb:
		return oka
	}
	return a < b
}

func leadingNumber(s string) (int, bool) {
	n, i := 0, 0
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9' && n < 1<<24; i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n, i > 0
}
