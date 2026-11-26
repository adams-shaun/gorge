package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Clone forms beyond the battlefield (dirac/kgen lower_clonecard.go,
// kernel copy_clonecard.go): Saheeli, Sublime Artificer (AddTypes$ on an
// until-end-of-turn copy), Curie, Emergent Intelligence (Defined$ Exiled, the
// card her Exile<> cost took; AddTriggers$ her damage trigger, whose draw is
// Count$CardBasePower) and Shifting Woodland (TgtZone$ Graveyard, delirium).
// The pool is dirac/cardset/pools/fixtures/democloneadd. The stat counters
// prove each shape fired, so a green game is not one where none did.
const demoCloneAddPool = "democloneadd"

var demoCloneAddDecks = [2][]cardset.Entry{
	{
		{Name: "Curie, Emergent Intelligence", Count: 4}, {Name: "Saheeli, Sublime Artificer", Count: 3}, {Name: "Juggernaut", Count: 3},
		{Name: "Ornithopter", Count: 4}, {Name: "Phyrexian Walker", Count: 2}, {Name: "Steel Overseer", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Glorious Anthem", Count: 2}, {Name: "Giant Growth", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Shifting Woodland", Count: 4}, {Name: "Mulch", Count: 3}, {Name: "Doom Blade", Count: 3}, {Name: "Deadly Dispute", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 2}, {Name: "Ornithopter", Count: 3}, {Name: "Darksteel Relic", Count: 1},
		{Name: "Unholy Strength", Count: 2}, {Name: "Mind Rot", Count: 1}, {Name: "Pacifism", Count: 1}, {Name: "Liliana of the Veil", Count: 3},
		{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 4},
	},
}

// demoCloneAddGames plays the seeds and returns the clean games, the
// divergent ones and what the Clone ops did.
func demoCloneAddGames(t *testing.T, hi uint64, mode lockstepMode) (clean int, divs int, cs dirac.CloneCopyStats, cc dirac.CloneCardStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoCloneAddPool, demoCloneAddDecks)
	dirac.DebugCloneCopyStats, dirac.DebugCloneCardStats = &cs, &cc
	defer func() { dirac.DebugCloneCopyStats, dirac.DebugCloneCardStats = nil, nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, demoCloneAddPool, demoCloneAddDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, len(d), cs, cc
}

// TestDemoCloneAddLockstep: no divergence and no kernel stall over 96 seeds,
// and across them each new shape fired.
func TestDemoCloneAddLockstep(t *testing.T) {
	clean, divs, cs, cc := demoCloneAddGames(t, 96, lockstepFailEach)
	if divs > 0 {
		t.Errorf("%d divergent games", divs)
	}
	if cc.Grave == 0 {
		t.Error("no game copied a graveyard card (Shifting Woodland)")
	}
	if cc.Cost == 0 {
		t.Error("no game copied the card a cost exiled (Curie)")
	}
	if cc.Types == 0 {
		t.Error("no game made a copy with an AddTypes$ exception (Saheeli)")
	}
	if cc.Grant == 0 || cc.Base == 0 {
		t.Errorf("no game drew through Curie's granted trigger: %d grants, %d base power reads", cc.Grant, cc.Base)
	}
	if cs.EOT == 0 || cs.Perm == 0 {
		t.Errorf("no until-end-of-turn or no permanent copy was made: %+v", cs)
	}
	t.Logf("%d clean games; clone %+v, card templates %+v", clean, cs, cc)
}

// TestDemoCloneAddMutation: with an off-battlefield template naming no card,
// and with Count$CardBasePower reading 0, some game diverges from gorge.
func TestDemoCloneAddMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for name, sw := range map[string]*bool{"DebugCloneCardTmplNone": &dirac.DebugCloneCardTmplNone, "DebugNoCardBasePower": &dirac.DebugNoCardBasePower} {
		*sw = true
		_, divs, _, _ := demoCloneAddGames(t, 96, lockstepQuiet)
		*sw = false
		if divs == 0 {
			t.Errorf("%s: no divergence in 96 games", name)
		}
		t.Logf("%s diverged in %d games", name, divs)
	}
}

// TestFourSeatDemoCloneAddLockstep: four seats, gorge leading.
func TestFourSeatDemoCloneAddLockstep(t *testing.T) {
	a := pod4Deck(deckOf(4, "Curie, Emergent Intelligence"), deckOf(3, "Saheeli, Sublime Artificer"), deckOf(3, "Juggernaut"), deckOf(4, "Ornithopter"),
		deckOf(2, "Phyrexian Walker"), deckOf(2, "Steel Overseer"), deckOf(3, "Grizzly Bears"), deckOf(2, "Glorious Anthem"), deckOf(2, "Giant Growth"),
		deckOf(9, "Island"), deckOf(6, "Mountain"))
	b := pod4Deck(deckOf(4, "Shifting Woodland"), deckOf(3, "Mulch"), deckOf(3, "Doom Blade"), deckOf(2, "Deadly Dispute"), deckOf(4, "Grizzly Bears"),
		deckOf(2, "Llanowar Elves"), deckOf(3, "Ornithopter"), deckOf(1, "Darksteel Relic"), deckOf(2, "Unholy Strength"), deckOf(1, "Mind Rot"),
		deckOf(1, "Pacifism"), deckOf(3, "Liliana of the Veil"), deckOf(7, "Forest"), deckOf(4, "Swamp"))
	if n := pod4FixtureLockstep(t, demoCloneAddPool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
