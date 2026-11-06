package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A fight whose fighter's damage event is matched by two ReplDamagePlus
// replacements (two Fated Firepowers) is a pure plus-N competition: the two
// additions commute, so the amount is the same in every order, but gorge still
// collects both matching damage replacements and asks the affected player
// their CR 616.1 order. The batch must pose that ask before landing, then deal
// every hit with the summed amount. Deck B is the fightex pool's deck 2, which
// has no Fated Firepower, so only deck A's fighter's hit competes: exactly one
// event per fight (the one-ask-per-op model).
var kreplPlusBatchDecks = [2][]cardset.Entry{
	{
		{Name: "The Last Agni Kai", Count: 4}, {Name: "Fated Firepower", Count: 2}, {Name: "Hill Giant", Count: 4},
		{Name: "Colossal Dreadmaw", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Mountain", Count: 17}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Elite Vanguard", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Colossal Dreadmaw", Count: 2},
		{Name: "Mind Stone", Count: 3}, {Name: "Short Sword", Count: 3}, {Name: "Soul-Guide Lantern", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
	},
}

// kreplPlusBatchRun plays seeds 1..seeds with deck A in both seat orders and
// both autopay surfaces, and returns the tallies, the first divergence ("" if
// none) and the kernel stalls.
func kreplPlusBatchRun(t *testing.T, seeds uint64) (tal kfightexTally, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("fightex")
	if err != nil {
		t.Fatal(err)
	}
	var deckCards [2][]*cards.Card
	for d, deck := range kreplPlusBatchDecks {
		for _, e := range deck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				deckCards[d] = append(deckCards[d], c)
			}
		}
		if len(deckCards[d]) != 40 {
			t.Fatalf("deck %d: %d cards", d, len(deckCards[d]))
		}
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		order := [2]int{0, 1}
		if seed/2%2 == 1 {
			order = [2]int{1, 0} // both seat orders
		}
		var decks [2][]*cards.Card
		var entries [2][]cardset.Entry
		for i, o := range order {
			decks[i], entries[i] = deckCards[o], kreplPlusBatchDecks[o]
		}
		tbl, _, _, err := set.Matchup(entries[0], entries[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed/4%2 == 1}
		m, err := NewMatchup(reg, [2]string{"krpb-a", "krpb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30, Check: kfightexCheck(&tal)})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tal, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tal, "", stalls
}

// TestReplPlusBatchOrderLockstep: a fight with two Fated Firepowers poses
// gorge's no-op replacement-order ask (the no-divergence at that decision is
// the proof) and the summed damage lands.
func TestReplPlusBatchOrderLockstep(t *testing.T) {
	t.Parallel()
	tal, div, stalls := kreplPlusBatchRun(t, 200)
	t.Logf("%d games; The Last Agni Kai mana adds %d; stalls %v", tal.games, tal.agniMana, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tal.agniMana < 1 {
		t.Errorf("Agni Kai mana adds %d, want at least 1", tal.agniMana)
	}
}
