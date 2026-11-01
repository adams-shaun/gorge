package cardset_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestLayerGuardPassesSetPools: the interim layer guard (kgen) refuses no
// matchup of the generated pools today -- one matchup holding every
// supported card fails, if at all, on size, never on the guard.
func TestLayerGuardPassesSetPools(t *testing.T) {
	for _, name := range []string{"fdn", "blb"} {
		s := cardsettest.Set(t, name)
		var all []cardset.Entry
		sup := s.Supported()
		for _, n := range s.Names() {
			if i, ok := s.Lookup(n); ok && sup[i] {
				all = append(all, cardset.Entry{Name: n, Count: 1})
			}
		}
		if _, _, _, err := s.Matchup(all, nil); err != nil && strings.Contains(err.Error(), "layer guard") {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
