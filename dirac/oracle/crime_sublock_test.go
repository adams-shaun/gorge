package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// CR 700.13 judges a crime at the frame the target is chosen, and CR
// 601.2c/602.2a record an activated ability's targets — the root's and the
// SubAbility$ chain links' alike — at their answers, before payment. Boom
// Box (AB$ Pump ... SubAbility$ DestroyCreature -> DestroyLand -> DB$
// Destroy) names an opponent's creature on its second link: gorge's
// TargetsChosen fold sets the ledger there, so Slickshot Vault-Buster's
// CheckSVar$ Count$CommittedCrimeThisTurn static turns it 3-power from the
// next decision frame on. A kernel that waits for the post-payment walk
// projects it at 1 in between and the lockstep gate fails the frame.
var boomDecks = [2][]cardset.Entry{{}, {}}

func init() {
	for i := range boomDecks {
		boomDecks[i] = []cardset.Entry{
			{Name: "Boom Box", Count: 4},
			{Name: "Slickshot Vault-Buster", Count: 4},
			{Name: "Omenport Vigilante", Count: 4},
			{Name: "Mountain", Count: 14},
			{Name: "Island", Count: 14},
		}
	}
}

func TestCrimeSubAbilityTargetsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set := boomSetup(t)
	decks := kcastDeck(t, reg, boomDecks)
	tbl, _, _, err := set.Matchup(boomDecks[0], boomDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for seed := uint64(1); seed <= 60; seed++ {
		m, err := NewMatchup(reg, [2]string{"crimea", "crimeb"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if res.Div != nil {
			div++
			lo := max(res.Div.At-12, 0)
			t.Logf("seed %d diverged at %d: %s %s %v\n  last actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, actionStrings(res.Actions[lo:res.Div.At]))
		}
	}
	if div != 0 {
		t.Fatalf("%d/60 seeds diverged", div)
	}
}

func boomSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	return reg, cardsettest.Synth(t, "std-otj")
}
