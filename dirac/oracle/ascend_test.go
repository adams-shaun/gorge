package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Ascend lockstep (CR 702.131): every Ascend carrier the blessing
// lowering lifted, in two matchups of the ascend pool. Boards grow to ten
// permanents, so the permanent form latches (a Dusk Charger, a Spire Winder
// on the battlefield) and, in the deck that holds Ascend only on its instants
// and sorceries, the spell form latches as Secrets of the Golden City or
// Expel from Orazca resolves. The readers (Count$Blessing.<yes>.<no>, the
// static Condition$ Blessing, Wayward Swordtooth's CheckSVar$) read both
// arms. The expected answer at every decision is gorge's.

var kascendMatchups = [2][2][]cardset.Entry{
	{ // the permanent form against the spell form
		{ // A: W/U/B Ascend permanents
			{Name: "Skymarcher Aspirant", Count: 3}, {Name: "Snubhorn Sentry", Count: 3}, {Name: "Spire Winder", Count: 2},
			{Name: "Dusk Charger", Count: 3}, {Name: "Kumena's Awakening", Count: 2}, {Name: "Radiant Destiny", Count: 2},
			{Name: "Andúril, Narsil Reforged", Count: 1}, {Name: "Savannah Lions", Count: 3}, {Name: "Walking Corpse", Count: 3},
			{Name: "Plains", Count: 7}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 5},
		},
		{ // B: U/B/W, Ascend on its instants and sorceries only
			{Name: "Secrets of the Golden City", Count: 3}, {Name: "Expel from Orazca", Count: 3}, {Name: "Golden Demise", Count: 2},
			{Name: "Vona's Hunger", Count: 2}, {Name: "Pride of Conquerors", Count: 3}, {Name: "Elite Vanguard", Count: 3},
			{Name: "Wind Drake", Count: 3}, {Name: "Walking Corpse", Count: 3},
			{Name: "Plains", Count: 6}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 6},
		},
	},
	{ // tokens and the rest of the lifted cards
		{ // C: G/W/R tokens
			{Name: "Tendershoot Dryad", Count: 3}, {Name: "Illustrious Wanderglyph", Count: 2}, {Name: "Wayward Swordtooth", Count: 3},
			{Name: "Storm Fleet Swashbuckler", Count: 3}, {Name: "Pride of Conquerors", Count: 2}, {Name: "Andúril, Narsil Reforged", Count: 1},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Goblin Piker", Count: 2},
			{Name: "Forest", Count: 7}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 5},
		},
		{ // D: U/B permanents and spells
			{Name: "Dusk Charger", Count: 2}, {Name: "Spire Winder", Count: 2}, {Name: "Secrets of the Golden City", Count: 2},
			{Name: "Expel from Orazca", Count: 2}, {Name: "Golden Demise", Count: 2}, {Name: "Vona's Hunger", Count: 2},
			{Name: "Kumena's Awakening", Count: 1}, {Name: "Wind Drake", Count: 3}, {Name: "Walking Corpse", Count: 4},
			{Name: "Island", Count: 10}, {Name: "Swamp", Count: 10},
		},
	},
}

// kascendLifted is every Ascend carrier the lowering lifted that the pool
// plays.
var kascendLifted = []string{
	"Andúril, Narsil Reforged", "Dusk Charger", "Expel from Orazca", "Golden Demise", "Illustrious Wanderglyph",
	"Kumena's Awakening", "Pride of Conquerors", "Radiant Destiny", "Secrets of the Golden City", "Skymarcher Aspirant",
	"Snubhorn Sentry", "Spire Winder", "Storm Fleet Swashbuckler", "Tendershoot Dryad", "Vona's Hunger", "Wayward Swordtooth",
}

// kascendProbe records, on gorge's side, the cards seen on the battlefield or
// on the stack, and the seats gorge has blessed.
type kascendProbe struct {
	played  map[string]int
	blessed int // decisions at which some seat had the blessing
}

func (p *kascendProbe) observe(gs *GorgeSide) {
	g := gs.E.G
	for s := range g.Players {
		if g.Players[s].Blessing {
			p.blessed++
		}
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(s)) {
			if o := g.Obj(id); o != nil && o.Face() != nil {
				p.played[o.Face().Name]++
			}
		}
	}
	for _, id := range g.Stack {
		if o := g.Obj(id); o != nil && o.Card != nil && o.Face() != nil {
			p.played[o.Face().Name]++
		}
	}
}

// TestAscendBlessingLowers: every decision of every seed matches gorge in
// both matchups; both forms of the latch granted the blessing, every reader
// read both its arms, and every lifted card was played. With the latch
// disabled (dirac.DebugBlessingOff) some seed diverges.
func TestAscendBlessingLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	st := &dirac.BlessStats{}
	dirac.DebugBlessStats = st
	defer func() { dirac.DebugBlessStats = nil }()
	probe := &kascendProbe{played: map[string]int{}}
	policy := func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				probe.observe(c.G)
			}
			return base.Pick(c)
		})
	}
	const seeds = 40
	for i, decks := range kascendMatchups {
		if div := lockstepRunPolicy(t, "ascend", decks, seeds, true, nil, nil, policy); div != 0 {
			t.Errorf("matchup %d: %d of %d seeds diverged", i, div, seeds)
		}
	}
	t.Logf("kernel: %+v; gorge blessed at %d seat-decisions", *st, probe.blessed)
	if probe.blessed == 0 {
		t.Error("gorge never granted the blessing: the pool never reached ten permanents")
	}
	for _, c := range []struct {
		n    int
		what string
	}{
		{st.Permanent, "permanent-form grants (CR 702.131b)"}, {st.Spell, "spell-form grants (CR 702.131a)"},
		{st.ExYes, "Count$Blessing reads that held"}, {st.ExNo, "Count$Blessing reads that failed"},
		{st.CondYes, "Condition$ Blessing reads that held"}, {st.CondNo, "Condition$ Blessing reads that failed"},
	} {
		if c.n == 0 {
			t.Errorf("%s: none", c.what)
		}
	}
	for _, n := range kascendLifted {
		if probe.played[n] == 0 {
			t.Errorf("%s: never played", n)
		}
	}
	if t.Failed() {
		return
	}
	// the mutation: with the latch off, the readers must expose it
	dirac.DebugBlessingOff = true
	defer func() { dirac.DebugBlessingOff = false }()
	if !lockstepDiverges(t, "ascend", kascendMatchups[0], seeds) {
		t.Error("no seed diverged with the blessing latch disabled: the pool never reads a granted blessing")
	}
}
