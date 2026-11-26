package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The printed conspire (CR 702.78a): spells that print K:Conspire, the cast
// that taps two creatures sharing a colour with them, and the copy their own
// SpellCast trigger makes when Count$Conspired reads the paid count
// (dirac/cost_paidcount.go). The deck and policy are the granted-conspire test's
// (stackkw_conspire_test.go), with the keyword printed instead of granted.

var conspirePrintedDeck = []cardset.Entry{
	{Name: "Burn Trail", Count: 4}, {Name: "Disturbing Plot", Count: 2}, {Name: "Rally the Galadhrim", Count: 2},
	{Name: "Ghastly Discovery", Count: 2}, {Name: "Gleeful Sabotage", Count: 2}, {Name: "Aethertow", Count: 2},
	{Name: "Barkshell Blessing", Count: 2},
	{Name: "Burning-Tree Emissary", Count: 2}, {Name: "Iridescent Tiger", Count: 3},
	{Name: "Grizzly Bears", Count: 3}, {Name: "Merfolk of the Pearl Trident", Count: 3},
	{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 3}, {Name: "Island", Count: 3}, {Name: "Swamp", Count: 2},
}

func runConspirePrinted(t *testing.T, seeds int, report bool) (games, divs, stalls int, cnt map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("conspireprinted")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range conspirePrintedDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck has %d cards", len(deck))
	}
	tbl, _, _, err := set.Matchup(conspirePrintedDeck, conspirePrintedDeck)
	if err != nil {
		t.Fatal(err)
	}
	decks := [2][]*cards.Card{deck, deck}
	cnt = map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kconspire-printed-a", "kconspire-printed-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kconspirePolicy(seed, cnt), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if !report || divs >= 3 {
				break
			}
		}
	}
	return games, divs, stalls, cnt
}

// TestConspirePrintedLockstep: printed conspire plays the same in the kernel as
// in gorge over 256 seeds: the conspired cast, the tap election and the copies.
func TestConspirePrintedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, cnt := runConspirePrinted(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls, plays %v", games, divs, stalls, cnt)
	for _, w := range []string{"conspired", "copy"} {
		if cnt[w] < 5 {
			t.Errorf("only %d printed %s plays (want >= 5)", cnt[w], w)
		}
	}
}

// TestConspirePrintedMutation: a conspired cast that stores no paid count
// leaves the printed trigger's Count$Conspired at 0, so its copies vanish from
// the kernel's stack and the lockstep must diverge.
func TestConspirePrintedMutation(t *testing.T) {
	old := dirac.DebugPaidConspireOff
	dirac.DebugPaidConspireOff = true
	defer func() { dirac.DebugPaidConspireOff = old }()
	games, divs, _, _ := runConspirePrinted(t, 64, false)
	if divs == 0 {
		t.Fatalf("no paid count stored in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
