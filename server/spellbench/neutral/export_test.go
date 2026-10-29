package neutral

import (
	"maps"
	"slices"
	"sort"
)

// CopyIDsForTest copies src's protocol-id numbering into dst, so a fresh
// builder can project an unchanged observation with the same object ids as
// the incremental one. A fresh builder otherwise renumbers ids from its own
// first-appearance order, which is history the view necessarily carries.
func CopyIDsForTest(dst, src *IDs) {
	dst.m = maps.Clone(src.m)
	dst.back = slices.Clone(src.back)
}

// CachedObjectIDsForTest returns the view cache's keys, sorted, so an external
// test can assert which objects the previous View left in the cache.
func CachedObjectIDsForTest(b *Builder) []string {
	ids := make([]string, 0, len(b.cards))
	for id := range b.cards {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
