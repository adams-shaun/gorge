package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// becomestargetlistCombinedDecks is the combined BecomesTarget pool: the old
// Fey Steed / Angelic Cub latch pair in the same decks as the player/Kira
// pool (Loki, Rayne, Dormant Gomazoa, Kira). Each arm plays correctly in its
// own simpler pool (TestBecomesTargetListLatchLockstep,
// TestBecomesTargetListLockstep); the interference between the Fey Steed /
// Angelic Cub attack-trigger target shapes and the player/Kira reversal
// shapes only shows in the larger deck.
func becomestargetlistCombinedDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Fey Steed", Count: 4},
		{Name: "Angelic Cub", Count: 4},
		{Name: "Loki, God of Mischief", Count: 2},
		{Name: "Rayne, Academy Chancellor", Count: 2},
		{Name: "Dormant Gomazoa", Count: 2},
		{Name: "Kira, Great Glass-Spinner", Count: 2},
		{Name: "Lightning Bolt", Count: 8},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 4},
		{Name: "Island", Count: 5},
		{Name: "Mountain", Count: 2},
		{Name: "Forest", Count: 1},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// TestBecomesTargetListCombinedLockstep runs both BecomesTarget arms in the
// same decks: the Fey Steed / Angelic Cub FirstTime and activation-limit
// latches next to the Loki / Rayne / Dormant Gomazoa / Kira player-target and
// reversal shapes.
func TestBecomesTargetListCombinedLockstep(t *testing.T) {
	st := &dirac.BtStats{}
	dirac.DebugBtStats = st
	recs := []dirac.CounterFirstRec{}
	dirac.CounterFirstLog = &recs
	defer func() {
		dirac.DebugBtStats = nil
		dirac.CounterFirstLog = nil
	}()
	lockstepRunPolicy(t, "becomestargetlist", becomestargetlistCombinedDecks(), 300, true, nil, nil, btcastFirstPolicy)
	if st.Queues == 0 {
		t.Fatal("no BecomesTarget trigger was queued in the games; the lockstep proves nothing")
	}
}
