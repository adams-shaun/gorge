package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
)

// acceptanceHeads pins the chain head of the deterministic acceptance game
// at each seat count (R-14). A change here is a change to what the 12 repo
// decks do; the commit that makes it must name the card behaviour that
// moved it. The seed, bot and deck assignment are TestRepoDecksPlayAtEverySeatCount's
// own (rules/acceptance_test.go's playAcceptance), so the two tests always agree.
var acceptanceHeads = map[int]string{
	2: "49bf77aae8335d50",
	4: "26f421e82f3f5f8d",
	6: "6cd5a5255c49a11d",
	8: "2978fc4ef3eb8f31",
}

func TestHeads(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	for _, seats := range []int{2, 4, 6, 8} {
		got := acceptanceHead(t, reg, seats)
		if want := acceptanceHeads[seats]; got != want {
			t.Errorf("%d seats: chain head %s, golden %s — if this move is intended, update acceptanceHeads and name the cause in the commit body", seats, got, want)
		}
	}
}
