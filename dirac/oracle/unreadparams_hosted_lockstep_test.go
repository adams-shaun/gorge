package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hosted-Commander unread-param shapes the kernel now lowers (ticket
// cli-20261127T065129Z-e7b92793; the acceptance half is dirac/kgen's
// TestUnreadParamsHostedLockstep): each test plays the card's shape in real
// two-seat games against gorge, castFirstPolicy deciding, and fails on the
// first divergence.

// Nick Valentine, Private Eye: "whenever Nick Valentine or another artifact
// creature you control dies, you may investigate" (DB$ Investigate | Optional$
// True; gorge investigateOptionalWalk's yes / no ask, an OpMayAsk before the
// Clue mint). Juggernauts must attack into the walkers, so artifact creatures
// die.
var nickValentineDeck = []cardset.Entry{
	{Name: "Nick Valentine, Private Eye", Count: 8}, {Name: "Juggernaut", Count: 6},
	{Name: "Phyrexian Walker", Count: 6}, {Name: "Steel Wall", Count: 4}, {Name: "Island", Count: 16},
}

func TestUnreadParamsNickValentineLockstep(t *testing.T) {
	t.Parallel()
	d := nickValentineDeck
	seen := 0
	tally := func(g *state.Game) int {
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := g.Obj(id); o != nil && o.Face() != nil && strings.HasPrefix(o.Face().Name, "Clue") {
					seen = 1
				}
			}
		}
		return seen
	}
	got := cedhPodRun(t, "nickvalentine", [][]cardset.Entry{d, d}, []string{"Nick Valentine, Private Eye"}, 40, tally)
	if got == 0 {
		t.Fatal("no game made a Clue: the may-investigate ask was never answered yes")
	}
	t.Logf("%d games with a Clue", got)
}

// Urza's Bauble: "{T}, sacrifice: look at a card at random in target player's
// hand" (AB$ Reveal | Random$ True | ValidTgts$ Player; gorge draws the seeded
// RNG once over the hand, ExtRevealRandom). Hymn to Tourach's random discards
// after it keep the stream honest: a missed or extra draw desyncs them.
var urzasBaubleDeck = []cardset.Entry{
	{Name: "Urza's Bauble", Count: 10}, {Name: "Hymn to Tourach", Count: 6}, {Name: "Phyrexian Walker", Count: 4},
	{Name: "Swamp", Count: 18},
}

func TestUnreadParamsUrzasBaubleLockstep(t *testing.T) {
	t.Parallel()
	d := urzasBaubleDeck
	tally := func(g *state.Game) int { return cedhZoneCount(g, state.ZGraveyard, "Urza's Bauble") }
	got := cedhPodRun(t, "urzasbauble", [][]cardset.Entry{d, d}, []string{"Urza's Bauble", "Hymn to Tourach"}, 40, tally)
	t.Logf("%d Baubles sacrificed", got)
	if got < 10 {
		t.Fatalf("too little evidence: %d sacrifices (want 10)", got)
	}
}
