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
// M2d moved all four heads, and owns exactly ONE regeneration -- this one,
// at the milestone merge gate, not once per sub-task. Build-bisected causes
// (FL-76), measured per seat count rather than assumed:
//
//   - M2d-1, the London mulligan (R-M1): moves ALL FOUR. playAcceptance now
//     sets Mulligans: 1, so every acceptance game runs a keep/mulligan round
//     and, for any seat that mulligans, a re-shuffle, a fresh seven and a
//     bottoming round -- all of it emitted, so every head shifts.
//     Intermediate heads after M2d-1 alone: 2 45e0671d07b60d9e,
//     4 dcee545be139ca21, 6 642a239a24d3a1fe, 8 acf4ad4bafda267f.
//
//   - M2d-2, KModes and the mid-resolution ask (R-8): moves 4, 6 and 8 only.
//     It emits the new ModeChosen kind, and counting that kind in each
//     acceptance log gives 2 seats: 0, and 4/6/8 seats: 1 apiece. So the
//     2-seat golden below is M2d-1's value, unchanged by M2d-2.
//
//     An earlier version of this comment explained the 2-seat zero by saying
//     dimir-tempo's Spell Pierce carries UnlessCost$ 2 and is "served through
//     this same kind", just never cast under this seed. That is wrong, and
//     the correction matters more than the head does: effCounter
//     (effects/misc.go) never reads UnlessCost$ at all. Spell Pierce, Mana
//     Leak, Daze and Mausoleum Wanderer counter unconditionally, so casting
//     one would emit no ModeChosen either. M2d-2 closed R-8 for
//     CopySpellAbility only -- see AGENTS.md's Known approximations. The
//     per-seat COUNT above was measured and stands; only the explanation of
//     the zero was wrong.
//
//   - M2d-3, concede (R-M3): moves NOTHING. It only adds an option to every
//     priority decision, and neither bot mirror ever picks it; an offered
//     but untaken option is not an event.
var acceptanceHeads = map[int]string{
	2: "45e0671d07b60d9e",
	4: "795a100313094d6c",
	6: "0311852b655e44d0",
	8: "1216344ec91e5881",
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
