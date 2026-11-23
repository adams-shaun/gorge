package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const feedbackRepro = "feedback_aura_cast_offer"

// Rain of Filth gives the tapped lands "Sacrifice this land: Add {B}", and The
// World Tree gives an untapped land "{T}: Add one mana of any color". Feedback
// costs {2}{U}: the World Tree's granted tap for {U}, Shaku for {C} and one
// sacrifice for {B} pays it (CR 605.3a, 106.1b). gorge's prefix search
// (potentialPrefixScript) finds no script over those grants within its 160-node
// budget (potentialScriptNodes), so the cast is census_incomplete and never
// offered, while the kernel prices the cast and offers it (tail-08-c000,
// decision 690). Matching the search's node accounting over a dynamic grant
// list is far more than the kernel's script mirror holds, so the card is
// refused by name as Iron Fist, Hero for Hire is, and the recorded game still
// shows the divergence with it implicated.
func TestMBCFeedbackReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Feedback" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != feedbackRepro {
			t.Fatalf("Feedback's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, feedbackRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Feedback has no dirac/support/refused entry: gorge's prefix search finds no script over the granted sacrifices and the granted any-colour tap and the kernel offers the cast")
}
