package oracle

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// definedRefsSlice4Carriers are the slice's lowered cards this fixture can
// cast: Loki's single-target arm (RefTriggerSpellTarget), Storm's set arm
// (RefTriggerSpellTargets), and Aura Barbs' multi-source damage
// (DmgFromEachCtrl / DmgFromEachAttached). Lightning Bolt and Giant Growth are
// the cheap spells that trigger the SpellCast lines; the watch policy casts
// every carrier whenever it can.
var definedRefsSlice4Carriers = []string{"Loki, God of Lies", "Storm, Windrider", "Aura Barbs", "Pacifism", "Lightning Bolt", "Giant Growth"}

// definedRefsSlice4Decks are two 40-card constructed decks: deck A casts the
// carriers and the cheap spells (five-colour mana), deck B supplies the
// creatures they target and Counterspell, which can remove a cast spell while
// its trigger is still on the stack (gorge clears the spell's targets as it
// leaves the stack, so the trigger gains nothing; the kernel reads the spell's
// stack entry and must agree).
var definedRefsSlice4Decks = [2][]cardset.Entry{
	{
		{Name: "Loki, God of Lies", Count: 2}, {Name: "Storm, Windrider", Count: 2},
		{Name: "Aura Barbs", Count: 4}, {Name: "Pacifism", Count: 5}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Giant Growth", Count: 2}, {Name: "Mana Confluence", Count: 23},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Wind Drake", Count: 4},
		{Name: "Counterspell", Count: 4}, {Name: "Mana Confluence", Count: 20},
	},
}

// TestDefinedRefsSlice4Lockstep plays the carriers and the spells that trigger
// them in constructed lockstep games, half on each of gorge's autopay
// surfaces. The cards are not in a standard synthetic pool, so the table is
// built from the decks' own card list. Every carrier must be cast at least
// once (the precondition), and no game may diverge or stall.
func TestDefinedRefsSlice4Lockstep(t *testing.T) {
	t.Parallel()
	seeds := 25 // per surface; 50 games total
	if v := os.Getenv("KDEFINEDREFS_SEEDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			seeds = n
		}
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	for s, d := range definedRefsSlice4Decks {
		for _, e := range d {
			names[s] = append(names[s], e.Name)
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(definedRefsSlice4Decks[0], definedRefsSlice4Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range definedRefsSlice4Decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards, want 40", s, len(gd[s]))
		}
	}
	watch := make(map[string]bool, len(definedRefsSlice4Carriers))
	for _, n := range definedRefsSlice4Carriers {
		watch[n] = true
	}
	casts := map[string]int{}
	for _, surface := range []bool{false, true} {
		cfg := Config{GorgeAutopaySurface: surface}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			m, err := NewMatchup(reg, [2]string{"definedrefs4-a", "definedrefs4-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := NewGorgeSide(m, seed, cfg, nil); err != nil {
				CloseSide(k)
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, fixtureWatchPolicy(seed, watch, casts), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			if res.Div != nil {
				t.Fatalf("autopay=%v seed %d: diverged at decision %d: %s %s %v\nref %v\nker %v",
					surface, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("autopay=%v seed %d: kernel stall %q", surface, seed, res.Stall)
			}
		}
	}
	for _, n := range definedRefsSlice4Carriers {
		if casts[n] == 0 {
			t.Errorf("%s was never cast across the games: the fixture does not exercise it", n)
		}
	}
	t.Logf("defined refs slice 4: %d games per surface; casts %v", seeds, casts)
}
