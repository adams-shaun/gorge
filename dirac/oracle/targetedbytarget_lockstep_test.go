package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestTargetedByTargetLockstepMutation: with the nested target count forced to
// zero (dirac.DebugTgtEntryCountOff), Not of This World's games must diverge.
// The pin is what keeps the read honest: an op that never found the targeted
// entry, or a lowering that read the resolving spell's OWN targets, would leave
// these games identical and TestHostedCmdrFilterPropsResidual's clean seeds
// vacuous. The deck is tuned for it: {7} generic behind 16 lands, so the
// {7}-less price the count unlocks is often the only one the pool can pay.
//
// Bane's Contingency, the head's other carrier, is not here: its spec is
// Card.IsCommander, which a duel's seats never carry (gorge's
// wordIsCommander reads state.Game.CommanderOf, empty outside a Commander
// game), so its read is legitimately zero every game and dropping it changes
// nothing. Its evidence is the clean games and the kernel's own
// TestCountTgtEntryMatches.
func TestTargetedByTargetLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	const name = "Not of This World"
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	decks := hostedFilterPropsResidualDecks[name]
	var names [2][]string
	var gd [2][]*cards.Card
	for s := range decks {
		for _, e := range decks[s] {
			names[s] = append(names[s], e.Name)
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("%s deck %d: %d cards", name, s, len(gd[s]))
		}
	}
	set := printedDelveSet(t, names)
	i, ok := set.Lookup(name)
	if !ok {
		t.Fatalf("%s not in the table", name)
	}
	if got := set.Status(i); got != cardset.StatusUnverified {
		t.Fatalf("%s: status %s, want unverified", name, got)
	}
	dirac.DebugTgtEntryCountOff = true
	defer func() { dirac.DebugTgtEntryCountOff = false }()
	clean, casts, div := lockstepSeeds(t, reg, set, "targetedbytarget-mutation", decks, gd, 1, 64, false, nil, nil, hostedFilterPropsPolicy)
	t.Logf("%s: %d clean games, %d casts/activations, %d divergent with the count dropped", name, clean, casts, div)
	if div == 0 {
		t.Errorf("%s: the nested count reads zero and no seed diverged in %d games: the games never ask the read, so the pin proves nothing", name, clean)
	}
}
