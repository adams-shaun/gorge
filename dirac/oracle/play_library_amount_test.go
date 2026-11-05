package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The play-library lockstep: Cosmic Cube's attack trigger peeks at the top
// six cards, offers a free cast of one of them with mana value at most the
// greatest power among the attackers (Play ValidZone$ Library, Amount$ 1,
// ForgetPlayed$) and puts the rest on the bottom in a random order (gorge
// ChangeZoneAll RandomOrder$ through the engine rng). Doom Reigns Supreme's
// fifth plan counter sacrifices it; the reflexive trigger exiles the target
// opponent's top five and offers up to two free casts among them (Play
// Amount$ 2). The expected answer at every decision is gorge's.
var kplaylibMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Cosmic Cube", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Mountain", Count: 14}, {Name: "Forest", Count: 14},
	},
}, {
	{
		{Name: "Doom Reigns Supreme", Count: 4}, {Name: "Merciless Enforcers", Count: 6}, {Name: "Kingpin's Enforcers", Count: 6},
		{Name: "Glamorous Grapplers", Count: 4}, {Name: "Swamp", Count: 20},
	},
	{
		{Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}}

type kplaylibTally struct {
	games, fromLib, fromExile, exilePairs int
}

// kplaylibCheck counts the free casts each card reaches: a spell put on the
// stack from a library (Cosmic Cube's only route there) and a spell put on
// the stack from exile (Doom Reigns Supreme's), two in a row being one
// resolution's pair.
func kplaylibCheck(tal *kplaylibTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen, run := 0, 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen, run = ref, 0, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			if ev.Kind != events.PutOnStack {
				continue
			}
			switch ev.From {
			case state.ZLibrary:
				tal.fromLib++
				run = 0
			case state.ZExile:
				tal.fromExile++
				if run++; run == 2 {
					tal.exilePairs++
				}
			default:
				run = 0
			}
		}
		return nil
	}
}

func TestPlayLibraryAmountLockstep(t *testing.T) {
	t.Parallel()
	var tal kplaylibTally
	_, stalled := sonRunStalls(t, "playlib", kplaylibMatchups, 100, true, nil, kplaylibCheck(&tal))
	t.Logf("%d games: %d casts from a library, %d from exile (%d pairs); %d stalled loud", tal.games, tal.fromLib, tal.fromExile, tal.exilePairs, stalled)
	if tal.fromLib < 5 || tal.fromExile < 5 || tal.exilePairs < 2 {
		t.Errorf("casts from a library %d (want >= 5), from exile %d (want >= 5), exile pairs %d (want >= 2)", tal.fromLib, tal.fromExile, tal.exilePairs)
	}
}
