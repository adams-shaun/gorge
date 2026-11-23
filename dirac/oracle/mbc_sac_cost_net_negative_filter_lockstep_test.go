package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// CR 601.2g-h, 605.3a: Great Hall of Starnheim's {W}{W}{B}, {T}, sacrifice it
// and a creature is payable on a board whose only white comes from Saruli
// Caretaker (tap another creature) and Llanowar Envoy's {1}{G} any-colour
// filter (tail-28 pair 10). The kernel now pays {W}{W}{B} through the
// tap-another colour ask's one-shot prune (dirac/mana_mixgaps.go tapXGap), so
// both engines offer the activation and the recorded game replays clean on the
// raw table of its own cards, serving every scripted action.
func TestMBCSacCostNetNegativeFilterReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "sac_cost_activation_beside_net_negative_filter.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t) // skips without the .cards corpus; printedDelveSet needs it too
	set := cardset.Raw(printedDelveSet(t, r.Cards))
	stallCheck(t, r, replayServedValue(t, reg, set, r), false)
}
