package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The enter-time cast provenance lockstep (gorge WasCast on
// the object a resolution puts onto the battlefield): W/B decks over the
// generated wascast pool. Patched Plaything's enters replacement
// (CheckSVar$ Count$wasCastFromYourHandByYou.1.0) puts two -1/-1 counters on it
// only when its controller cast it from their hand. Cloudshift flickers one
// and Dread Return reanimates one, both entering uncast; Murder, Journey to
// Nowhere and Stitcher's Supplier move Plaything to the graveyard and exile.
//
// The Standard trigger carriers (Doomsday Excruciator, Satoru, Twilight
// Diviner, Myojin of Night's Reach) refuse on a second blocker by reading,
// so the replacement read is the lockstep's only carrier.

// The spell cast-provenance lockstep (gorge WasCast,
// WasCastFromHand and WasCastFromExile on a spell on the stack): W/U against
// U/R decks over the generated wascastspell pool. Vega the Watcher, Keeper of Secrets
// and Memory Worm trigger on a spell cast from anywhere but a hand, Kami of
// Celebration on a spell cast from exile (Sentinel of Lost Lore and
// Frolicking Familiar are cast from exile after their adventures), and
// Otterball Antics and Antiquities on the Loose read their own provenance as
// they resolve (a flashback cast earns the counters).
//
// Mana restrictions on a spell's provenance (Interdimensional Web Watch,
// Mm'menon, the Right Hand) are in wascastrestrict_test.go.

var kwascastspellDecks = [2][]cardset.Entry{
	{ // A: W/U
		{Name: "Vega, the Watcher", Count: 3}, {Name: "Otterball Antics", Count: 4}, {Name: "Antiquities on the Loose", Count: 4},
		{Name: "Sentinel of Lost Lore", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Healer's Hawk", Count: 2},
		{Name: "Coral Merfolk", Count: 3}, {Name: "Opt", Count: 2}, {Name: "Plains", Count: 9}, {Name: "Island", Count: 7},
	},
	{ // B: U/R
		{Name: "Keeper of Secrets", Count: 2}, {Name: "Memory Worm", Count: 3}, {Name: "Kami of Celebration", Count: 2},
		{Name: "Otterball Antics", Count: 3}, {Name: "Frolicking Familiar", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Opt", Count: 2}, {Name: "Coral Merfolk", Count: 3}, {Name: "Hill Giant", Count: 2},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

// kwascastspellRun plays seeds 1..seeds and returns whether a game diverged,
// the kernel capacity stall count and the predicate's coverage counters.
func kwascastspellRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st dirac.WasCastSpellStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("wascastspell")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kwascastspellDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kwascastspellDecks[0], kwascastspellDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugWasCastSpellStats = &st
	defer func() { dirac.DebugWasCastSpellStats = nil }()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwascastspell-a", "kwascastspell-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			break
		}
	}
	return
}

func TestWascastspellLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	div, stalls, st := kwascastspellRun(t, 256, true)
	t.Logf("256 games, wascastspell %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.FromHand, 30, "reads of a spell cast from a hand"},
		{st.Elsewhere, 10, "reads of a spell cast from another zone"},
		{st.ResGraveyard, 10, "held reads of a resolving spell cast from a graveyard"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestWascastspellMutation: a read that takes every spell as cast from its
// caster's hand never lets Vega, Keeper of Secrets, Memory Worm or Kami trigger and
// never earns Otterball Antics' counter, which gorge does.
func TestWascastspellMutation(t *testing.T) {
	dirac.DebugWasCastFromHandIgnored = true
	defer func() { dirac.DebugWasCastFromHandIgnored = false }()
	if diverged, _, _ := kwascastspellRun(t, 64, false); !diverged {
		t.Fatal("DebugWasCastFromHandIgnored: no divergence in 64 seeds")
	}
}
