package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMayAskSeesGrantedRiot: Rhythm of the Wild's Continuous AddKeyword$
// grants Riot, so a vanilla creature spell poses the CR 702.136a
// counter-or-haste as-enters ask even though its own text is ask-free. The
// ask-free predicate used to read the card's printed keywords only and
// exempted the resolution, then panicked at the ask ("the ask-free predicate
// missed an ask: choose/etb"). The board gate makes the resolving spell
// checkpoint; the control without the grant stays exempt.
func TestMayAskSeesGrantedRiot(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	rhythm := mustCorpusCard(t, reg, "Rhythm of the Wild")
	bears := mustCorpusCard(t, reg, "Grizzly Bears")

	for _, granted := range []bool{false, true} {
		e, cfg := tokenReplGame(t, 7555083076110424585, rhythm, bears)
		moveSeededCard(t, e, 0, bears, state.ZHand)
		if granted {
			moveSeededCard(t, e, 0, rhythm, state.ZBattlefield)
		}
		addMana(t, e, 0, "GG")
		castSpellOption(t, e, bears.Faces[0].Name)
		if got := tapeMayAsk(e); got != granted {
			t.Fatalf("granted=%v: tapeMayAsk = %v; a granted Riot must checkpoint the entry and an ungranted vanilla creature must stay ask-free", granted, got)
		}
		replayCheck(t, e, cfg)
	}
}
