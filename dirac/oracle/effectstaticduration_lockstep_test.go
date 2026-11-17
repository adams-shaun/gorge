package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The UntilHostLeavesPlayOrEOT lockstep: a triggered DB$ Effect whose static
// reads the Effect's source (the host) ends at the end of turn OR when that
// host leaves the battlefield, whichever is first (gorge's continuousLive and
// EndOfTurnCleanup; the kernel's WhileHost plus UntilEOT). The carriers are
// the triggered CanAttackDefender / CantBlockBy shapes the lowering accepts
// and the AssignNoCombatDamage statics the kernel zeroes in the combat damage
// step (Ophidian, Carrion Rats, Dwarven Vigilantes; the Rats' switched
// ExileFromGrave unless asks every player). Cloak of Confusion and Laccolith
// Rig are the remembered-attacker AssignNoCombatDamage shape: their Effect
// names RememberObjects$ TriggeredAttackerLKICopy rather than the effect's
// source, so the emblem captures the attacker. Opt and Rancor make Prismari
// Pledgemage's magecraft trigger and Steelclad Spirit's enchantment trigger
// reachable, and Skyclave Squid's landfall fires off the lands.
var keffectStaticDurationDeck = []cardset.Entry{
	{Name: "Ophidian", Count: 3},
	{Name: "Carrion Rats", Count: 3},
	{Name: "Dwarven Vigilantes", Count: 3},
	{Name: "Harvesttide Sentry", Count: 3},
	{Name: "Prismari Pledgemage", Count: 4},
	{Name: "Steelclad Spirit", Count: 4},
	{Name: "Cloak of Confusion", Count: 2},
	{Name: "Laccolith Rig", Count: 2},
	{Name: "Skyclave Squid", Count: 2},
	{Name: "Nivix Cyclops", Count: 2},
	{Name: "Opt", Count: 4},
	{Name: "Rancor", Count: 2},
	{Name: "Forest", Count: 2},
	{Name: "Island", Count: 2},
	{Name: "Mountain", Count: 2},
}

func TestEffectStaticDurationLockstep(t *testing.T) {
	t.Parallel()
	// Resolve the corpus without git: pr-precheck's fresh-clone step runs a
	// git archive with no .git, where corpusDirOrSkip's git rev-parse fails
	// instead of skipping, and a corpus test must SKIP without .cards.
	// printedDelveSet resolves the same directory for its table.
	corpusDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no card corpus: %v", err)
	}
	reg, err := cards.SharedCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range keffectStaticDurationDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	set := printedDelveSet(t, names)
	decks := [2][]cardset.Entry{keffectStaticDurationDeck, keffectStaticDurationDeck}
	const seeds = 80
	clean, casts, div := lockstepSeeds(t, reg, set, "effectstaticduration", decks, gd, 1, seeds, true, nil, nil, castFirstPolicy)
	t.Logf("%d seeds, %d clean, %d divergent; %d casts/activations", seeds, clean, div, casts)
	if div != 0 {
		t.Fatalf("%d divergent games", div)
	}
	if casts == 0 {
		t.Fatal("no cast or activation in the games; the lockstep proves nothing")
	}
}
