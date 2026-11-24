package oracle

import "testing"

const feedbackRepro = "feedback_aura_cast_offer"

// Rain of Filth gives the tapped lands "Sacrifice this land: Add {B}", and The
// World Tree gives an untapped land "{T}: Add one mana of any color". Feedback
// costs {2}{U}: the World Tree's granted tap for {U}, Shaku for {C} and one
// sacrifice for {B} would pay it (CR 605.3a, 106.1b). gorge's prefix search
// (potentialPrefixScript) finds no script over those grants within its 160-node
// budget (potentialScriptNodes), so the cast is census_incomplete and never
// offered (tail-08-c000, decision 690). The kernel's grant search used to keep
// the grant lists it read at the root through a sacrifice step and priced the
// cast below the sacrificed World Tree; it now re-reads each source's granted
// abilities at the sacrifice steps (mana_grantcost.go sacBranch and
// tapSacBranch), so it withholds the cast like gorge. The recorded game replays
// clean and the card is unrefused.
func TestMBCFeedbackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, feedbackRepro)
}
