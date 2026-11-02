package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Tainted Adversary lockstep: "When it enters, you may pay {2}{B} any
// number of times ... put that many +1/+1 counters on it, then create twice
// that many black 2/2 Zombie creature tokens with decayed." Decayed (CR
// 702.147a): can't block, and when it attacks it is sacrificed at end of
// combat. The expected answer at every decision is gorge's.
var ktaintedMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Tainted Adversary", Count: 12}, {Name: "Swamp", Count: 24}, {Name: "Grizzly Bears", Count: 4},
	},
	{
		{Name: "Tainted Adversary", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Shock", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
}}

type ktaintedTally struct {
	games, tokens, sacrificed int
}

func ktaintedCheck(tal *ktaintedTally) func(*GorgeSide, Side) *Divergence {
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
			if ev.Kind == events.TokenCreate && strings.Contains(ev.Text, "zombie_decayed") {
				tal.tokens++
				continue
			}
			if ev.Kind != events.MoveZone || ev.From != state.ZBattlefield || ev.To != state.ZGraveyard {
				continue
			}
			if o := ref.E.G.Obj(ev.Obj); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Zombie Token" {
				tal.sacrificed++
			}
		}
		return nil
	}
}

func TestTaintedLockstep(t *testing.T) {
	t.Parallel()
	var tal ktaintedTally
	_, stalled := sonRunStalls(t, "tainted", ktaintedMatchups, 600, true, nil, ktaintedCheck(&tal))
	t.Logf("%d games: %d decayed tokens created, %d left the battlefield for the graveyard; %d stalled loud on the 6-program delayed capacity", tal.games, tal.tokens, tal.sacrificed, stalled)
	if stalled > 12 {
		t.Errorf("%d games stalled on StallDProg; want at most 12", stalled)
	}
	if tal.tokens < 40 || tal.sacrificed < 10 {
		t.Errorf("decayed tokens %d (want >= 40), sacrificed %d (want >= 10)", tal.tokens, tal.sacrificed)
	}
}

// TestTaintedLockstepMutation: with the decayed attack trigger dropped from
// the token, some seed diverges.
func TestTaintedLockstepMutation(t *testing.T) {
	t.Parallel()
	n := sonRun(t, "tainted", ktaintedMatchups, 100, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			if strings.Contains(cards[i].Name, "ombie") && len(cards[i].Abilities) > 0 {
				cards[i].Abilities = nil
			}
		}
		tbl.Cards = cards
	}, nil)
	if n == 0 {
		t.Fatal("no seed diverged with the decayed trigger dropped")
	}
}
