package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The play-land lockstep: Vault 112: Sadistic Simulation's chapter III pays any
// amount of energy, exiles that many cards from the top of the library and
// offers one free play among them (Play Valid$ Card.IsRemembered, Amount$ 1,
// WithoutManaCost$). A land among the exiled cards is a candidate gorge plays
// as a land (CR 305.2a, 305.3: the active player's land play), a spell is
// cast free. The expected answer at every decision is gorge's.
var kplaylandMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Vault 112: Sadistic Simulation", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Island", Count: 12}, {Name: "Mountain", Count: 14},
	},
	{
		{Name: "Vault 112: Sadistic Simulation", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Island", Count: 12}, {Name: "Mountain", Count: 14},
	},
}}

type kplaylandTally struct {
	games, landsFromExile, spellsFromExile int
}

// kplaylandCheck counts the plays out of exile: a land entering the battlefield
// from exile (the land play) and a spell put on the stack from exile (the
// free cast), by the chapter III's picks.
func kplaylandCheck(tal *kplaylandTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			if ev.From != state.ZExile {
				continue
			}
			switch ev.Kind {
			case events.PutOnStack:
				tal.spellsFromExile++
			case events.LandPlayed:
				tal.landsFromExile++
			}
		}
		return nil
	}
}

func TestPlayLandLockstep(t *testing.T) {
	t.Parallel()
	var tal kplaylandTally
	_, stalled := sonRunStalls(t, "playland", kplaylandMatchups, 200, true, nil, kplaylandCheck(&tal))
	t.Logf("%d games: %d lands played from exile, %d spells cast from exile; %d stalled loud", tal.games, tal.landsFromExile, tal.spellsFromExile, stalled)
	if tal.landsFromExile < 3 || tal.spellsFromExile < 3 {
		t.Errorf("lands played from exile %d, spells cast from exile %d: want both >= 3", tal.landsFromExile, tal.spellsFromExile)
	}
}
