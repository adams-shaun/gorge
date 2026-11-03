package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A token that enters beside an entry-counter replacement whose amount counts
// the permanents you already control (Jean-Luc, Mon Capitaine's creature
// types) is not one of them: the replacement is applied before the token is on
// the battlefield (CR 614.12), as it is for a card that enters.
func TestTokenEntryCountExcludesTheEnteringToken(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := searchEngine(t, reg, "Jean-Luc, Mon Capitaine")
	searchMoveByName(t, e, "Jean-Luc, Mon Capitaine", state.ZBattlefield)
	searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "u_1_1_faerie_flying"})
	// Human and Officer from Jean-Luc, Bear from the Grizzly Bears; the
	// Faerie token's own type is not among the creatures already controlled.
	if got := p1p1Of(t, e, "Faerie Token"); got != 3 {
		t.Fatalf("Faerie token entered with %d +1/+1 counters, want 3", got)
	}
}
