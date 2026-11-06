package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// kreplPlusMultiDecks is the measured 60-card deck with n Fated Firepowers
// (the measured table). The 2/2 row puts two on BOTH battlefields, so a fight
// with The Last Agni Kai is a two-hit dealHits batch in which either hit (or
// both) can be matched by two pure plus-N replacements.
func kreplPlusMultiDecks(n int) [2][]cardset.Entry {
	mk := func() []cardset.Entry {
		return []cardset.Entry{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Colossal Dreadmaw", Count: 3},
			{Name: "The Last Agni Kai", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Fated Firepower", Count: n},
			{Name: "Mountain", Count: 32 - n}, {Name: "Forest", Count: 8},
		}
	}
	return [2][]cardset.Entry{mk(), mk()}
}

// kreplPlusMultiRun plays seeds 1..seeds with the same deck in both seat
// orders and both autopay surfaces (the measured castFirst/random mix) and
// returns the tallies, the first divergence ("" if none) and the kernel
// stalls by name.
func kreplPlusMultiRun(t *testing.T, n int, seeds uint64) (tal kfightexTally, div string, stalls map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("fightex")
	if err != nil {
		t.Fatal(err)
	}
	entriesIn := kreplPlusMultiDecks(n)
	var deckCards [2][]*cards.Card
	for d := range entriesIn {
		for _, e := range entriesIn[d] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				deckCards[d] = append(deckCards[d], c)
			}
		}
		if len(deckCards[d]) != 60 {
			t.Fatalf("deck %d: %d cards, want 60", d, len(deckCards[d]))
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
			decks[i], entries[i] = deckCards[o], entriesIn[o]
		}
		tbl, _, _, err := set.Matchup(entries[0], entries[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed/4%2 == 1}
		m, err := NewMatchup(reg, [2]string{"krpm-a", "krpm-b"}, decks, 1)
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

// TestReplPlusBatchMultiEventLockstep: a two-hit fight whose plus-N
// competition is on a later hit (or on both) is not a batch the one-parked-
// event op-resume shape can pose. It must stop at the named capacity stall
// before any hit lands, never diverge and never lose the earlier hit; the
// 0/0 control must stay divergence-free and stall-free.
func TestReplPlusBatchMultiEventLockstep(t *testing.T) {
	t.Parallel()
	const seeds = 200
	wantStall := KernelStallPrefix + dirac.StallName(dirac.StallReplPlusBatch)

	tal, div, stalls := kreplPlusMultiRun(t, 2, seeds)
	t.Logf("2/2: %d games; The Last Agni Kai mana adds %d; stalls %v", tal.games, tal.agniMana, stalls)
	if div != "" {
		t.Fatalf("2/2 diverged: %s", div)
	}
	for why, n := range stalls {
		if why != wantStall {
			t.Errorf("2/2 stalled at %q (%d games), want only the named capacity stall %q", why, n, wantStall)
		}
	}
	if stalls[wantStall] == 0 {
		t.Errorf("2/2 never reached the named capacity stall %q; the multi-event batch was not exercised (stalls %v)", wantStall, stalls)
	}
	if tal.agniMana < 1 {
		t.Errorf("The Last Agni Kai mana adds %d, want at least 1", tal.agniMana)
	}

	ctl, cdiv, cstalls := kreplPlusMultiRun(t, 0, seeds)
	t.Logf("0/0: %d games; The Last Agni Kai mana adds %d; stalls %v", ctl.games, ctl.agniMana, cstalls)
	if cdiv != "" {
		t.Fatalf("0/0 control diverged: %s", cdiv)
	}
	if len(cstalls) != 0 {
		t.Errorf("0/0 control kernel capacity stalls %v", cstalls)
	}
	if ctl.agniMana < 1 {
		t.Errorf("0/0 control The Last Agni Kai mana adds %d, want at least 1", ctl.agniMana)
	}
}
