package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Six Wastes beside Baxter Building pay a {1}{G} spell: {4} from the lands
// floats the fee and Baxter's "{4}, {T}: four mana in any combination" makes
// the {G}. The plain prefix search taps Baxter for its own {C} first and
// spends its 160 nodes under that branch; gorge's fee-order fallback run
// floats the fee first (potentialPrefixScript, scriptSearch.feeOrder), so the
// kernel prices the cast through it too (mana_feeorder.go).
func TestBaxterComboFilterPaysAGreenSpell(t *testing.T) {
	sc, id := manasecondScen(t, []cardset.Entry{
		{Name: "Wastes", Count: 6}, {Name: "Baxter Building", Count: 1}, {Name: "Grizzly Bears", Count: 1},
	})
	sc.perm(id["Baxter Building"], 0)
	sc.lands(0, id["Wastes"], 6)
	bears := sc.hand(id["Grizzly Bears"], 0)
	if !sc.castOffered(0, bears) {
		t.Errorf("Grizzly Bears ({1}{G}) is not offered over six Wastes and Baxter Building's {4},{T}: four mana of any colour")
	}
}
