package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRaiserTypesReadsACreatureSubtype: a raise whose filter names only a
// creature subtype (Brutal Suppression's Rebel.!token) reaches creatures, so
// the matchup guard does not refuse it beside a land's mana ability; a land
// subtype or an alternative keeps every type.
func TestRaiserTypesReadsACreatureSubtype(t *testing.T) {
	subs := []string{"Rebel", "Forest"}
	filters := []kop.Filter{{}, {Sub: 1}, {Sub: 2}, {Sub: 1, Or: 1}, {Types: kop.TArtifact, Sub: 1}}
	for _, c := range []struct {
		id   kop.FilterID
		want uint16
	}{{1, uint16(kop.TCreature)}, {2, 0}, {3, 0}, {4, uint16(kop.TArtifact)}, {0, 0}} {
		if got := raiserTypes(filters, subs, c.id); got != c.want {
			t.Errorf("filter %d: types %d, want %d", c.id, got, c.want)
		}
	}
}
