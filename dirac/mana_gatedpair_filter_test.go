package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckGatedPairFilterActivation: Stallion of Ashmouth's {1}{B} Delirium pump
// is payable from Whisperer of the Wilds' ferocious {T}: Add {G}{G} and
// Viridian Acolyte's {1},{T}: any colour: one {G} pays the Acolyte's fee, the
// Acolyte's {B} pays the pip, the other {G} pays the {1} (gorge's planner
// prices the gate-on pair's larger alternative, CR 605.1a). Nothing else is
// untapped, so the pump is offered only if the census counts the pair's {G}{G}
// beside the filter.
func CheckGatedPairFilterActivation(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	for _, cfg := range []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}, {GorgeAutopaySurface: true, NoElide: true}} {
		for _, gate := range []bool{true, false} {
			sc := newScenTableLib(t, tbl, id["Forest"], cfg)
			sc.perm(id["Whisperer of the Wilds"], 0)
			sc.perm(id["Viridian Acolyte"], 0)
			stall := sc.perm(id["Stallion of Ashmouth"], 0)
			if gate {
				big := sc.perm(id["Leatherback Baloth"], 0) // power 4: the ferocious gate
				sc.s.perms[sc.slot(big)].flags |= pfTapped
			}
			for _, n := range []string{"Grizzly Bears", "Giant Growth", "Divination", "Forest"} {
				sc.grave(id[n], 0, 1) // creature, instant, sorcery, land: four card types
			}
			if !sc.s.k().gen.anyGatePair {
				t.Fatal("anyGatePair unset: Whisperer is not a gated pair in this table")
			}
			if got := sc.activateOffered(stall, 0); got != gate {
				t.Errorf("cfg %+v, ferocious gate %v: Stallion's pump offered = %v, want %v", cfg, gate, got, gate)
			}
		}
	}
}
