package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestMBCFilters2Lowers: the Mystery Booster Commander filters-2 spelling
// family in lockstep, one subtest per card over the mbcfilters2 fixture pool.
// Both are CopyPermanent Populate$ True (CR 701.36a), whose "no target and no
// Defined$" refusal this change lowers onto the pick (OpChooseCard
// choosePopulatePick) and ExtCopyPicked:
//
//   - rootborn: Rootborn Defenses' populate and its indestructible grant for
//     the creatures you control;
//   - judgment: Trostani's Judgment's exile of a target creature, then populate.
//
// The creature tokens come from Raise the Alarm (Soldiers) and Dragon Fodder
// (Goblins), so a populate meets none, one or several kinds of token: gorge
// copies nothing for none, copies a lone token unasked and asks the controller
// to pick among several. Each subtest's probe counts the populate copies and
// the several-token asks and fails when either is missing, so a refused card
// (the pool would not build) or a game that never populates fails as surely as
// a divergence.
func TestMBCFilters2Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		decks [2][]cardset.Entry
		seeds uint64
		// copies and asks are the least populate copies and several-token
		// asks the games must reach
		copies, asks int
	}{
		{"rootborn", mbcRootbornDecks, 120, 100, 50},
		{"judgment", mbcJudgmentDecks, 120, 100, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			copies, asks := 0, 0
			pol := func(seed uint64) Policy { return mbcPopulateProbe(castFirstPolicy(seed), &copies, &asks) }
			if div := lockstepRunPolicy(t, "mbcfilters2", tc.decks, tc.seeds, true, nil, nil, pol); div != 0 {
				return // each divergence is already a test error
			}
			t.Logf("%s: %d populate copies, %d several-token asks", tc.name, copies, asks)
			if copies < tc.copies || asks < tc.asks {
				t.Fatalf("%s: %d populate copies (want %d), %d several-token asks (want %d) over %d seeds: the pool does not exercise it",
					tc.name, copies, tc.copies, asks, tc.asks, tc.seeds)
			}
		})
	}
}

// mbcPopulateProbe wraps base and counts, when gorge leads, the CopyToken
// events its log gains (the pool's only copy effect is populate) and the
// gorge's copypermanent_choice asks over several options (the populate pick).
func mbcPopulateProbe(base Policy, copies, asks *int) Policy {
	seen := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		evs := c.G.E.L.Events
		for ; seen < len(evs); seen++ {
			if evs[seen].Kind == events.CopyToken {
				*copies++
			}
		}
		// the token pick: gorge's copypermanent_choice ask over several tokens
		if fr := c.G.fr; fr != nil && fr.d != nil && fr.d.ResumeKind == "copypermanent_choice" && len(fr.d.Options) > 1 {
			*asks++
		}
		return a
	})
}

// mbcRootbornDecks: token makers to populate, Rootborn Defenses to do it,
// bodies for the other side to attack with.
var mbcRootbornDecks = [2][]cardset.Entry{
	{
		{Name: "Rootborn Defenses", Count: 8}, {Name: "Raise the Alarm", Count: 5}, {Name: "Dragon Fodder", Count: 5},
		{Name: "Plains", Count: 11}, {Name: "Mountain", Count: 11},
	},
	{
		{Name: "Rootborn Defenses", Count: 4}, {Name: "Raise the Alarm", Count: 4}, {Name: "Dragon Fodder", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

// mbcJudgmentDecks: Trostani's Judgment needs a creature to exile, so the
// other side plays bodies, and six mana to cast it.
var mbcJudgmentDecks = [2][]cardset.Entry{
	{
		{Name: "Trostani's Judgment", Count: 8}, {Name: "Raise the Alarm", Count: 5}, {Name: "Dragon Fodder", Count: 5},
		{Name: "Plains", Count: 14}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Trostani's Judgment", Count: 4}, {Name: "Raise the Alarm", Count: 4}, {Name: "Dragon Fodder", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Plains", Count: 14}, {Name: "Mountain", Count: 6},
	},
}

// TestMBCFilters2PopulateMutation: with the populate pick asked even for a
// lone token (gorge copies it unasked) some game diverges, so the pool
// reaches a populate with exactly one token and the lockstep compares the
// ask.
func TestMBCFilters2PopulateMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPopulateAlwaysAsk = true
	defer func() { dirac.DebugPopulateAlwaysAsk = false }()
	if lockstepDiverges(t, "mbcfilters2", mbcRootbornDecks, 60) {
		return
	}
	t.Fatal("no seed diverged with the lone-token populate ask forced")
}
