package mtgish

import (
	"sort"
	"strings"
)

// VariantCount is how many distinct cards use one Enum.Variant, and whether a
// lowering is registered for it.
type VariantCount struct {
	Key        string
	Cards      int
	Registered bool
}

// Census counts, per Enum.Variant key, the distinct cards whose definition
// holds it, sorted by key.
func Census(cards []*Card) []VariantCount {
	counts := map[string]int{}
	for _, c := range cards {
		seen := map[string]bool{}
		walk(c.Shared, func(_ node, enum, variant string) bool {
			k := strings.TrimPrefix(enum, "_") + "." + variant
			if !seen[k] {
				seen[k] = true
				counts[k]++
			}
			return true
		})
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]VariantCount, len(keys))
	for i, k := range keys {
		out[i] = VariantCount{Key: k, Cards: counts[k], Registered: Registered(k)}
	}
	return out
}
