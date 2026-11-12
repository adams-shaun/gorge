package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The AbilityCast lockstep: "whenever you activate an
// exhaust / loyalty ability" (Mode$ AbilityCast, ValidSA$ Activated.Exhaust,
// Activated.Loyalty, Activated.Loyalty+OppCtrl), fired as the activation
// completes. Deck A holds the exhaust carriers (and two Gideons, whose
// trigger watches the opponent's loyalty activations); deck B the loyalty
// carriers (one Vivien and one Kaito: their emblems fill the kernel's four
// emblem slots when a deck repeats them and always activates).

var kabilitycastDecks = [2][]cardset.Entry{
	{
		{Name: "Afterburner Expert", Count: 3}, {Name: "Rangers' Aetherhive", Count: 3}, {Name: "Rangers' Refueler", Count: 3},
		{Name: "Greasewrench Goblin", Count: 3}, {Name: "Keen Buccaneer", Count: 3}, {Name: "Skystreak Engineer", Count: 3},
		{Name: "Stampeding Scurryfoot", Count: 3}, {Name: "Mindspring Merfolk", Count: 3}, {Name: "Gideon the Oathless", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 1}, {Name: "Swamp", Count: 1},
	},
	{
		{Name: "Ajani Unrelenting", Count: 2}, {Name: "Way of the Paradox", Count: 3}, {Name: "Gideon the Oathless", Count: 2},
		{Name: "Chandra, Flameshaper", Count: 3}, {Name: "Vivien Reid", Count: 1}, {Name: "Kaito, Cunning Infiltrator", Count: 1},
		{Name: "Liliana, Dreadhorde General", Count: 3}, {Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 4}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 5},
	},
}

func runAbilitycast(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st dirac.AbilityCastStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("abilitycast")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugAbilityCastIgnoreClass
	dirac.DebugAbilityCastIgnoreClass = mutate
	dirac.DebugAbilityCastStats = &st
	defer func() {
		dirac.DebugAbilityCastIgnoreClass = old
		dirac.DebugAbilityCastStats = nil
	}()
	var decks [2][]*cards.Card
	for s, d := range kabilitycastDecks {
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
	tbl, _, _, err := set.Matchup(kabilitycastDecks[0], kabilitycastDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kabilitycast-a", "kabilitycast-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, activateOnlyPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, st
}

func TestAbilitycastLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runAbilitycast(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, firings exhaust=%d loyalty=%d loyalty+OppCtrl=%d, %d by an opponent's activation, %d from a graveyard",
		games, divs, stalls, st.Fires[kop.TrigAbCastExhaust], st.Fires[kop.TrigAbCastLoyalty], st.Fires[kop.TrigAbCastLoyaltyOpp], st.OppFires, st.GraveFires)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if n := st.Fires[kop.TrigAbCastExhaust]; n < 30 {
		t.Errorf("only %d exhaust-class firings (need 30)", n)
	}
	if n := st.Fires[kop.TrigAbCastLoyalty]; n < 20 {
		t.Errorf("only %d loyalty-class firings (need 20)", n)
	}
	if n := st.Fires[kop.TrigAbCastLoyaltyOpp]; n < 5 {
		t.Errorf("only %d Gideon firings on an opponent's activation (need 5)", n)
	}
}

// TestAbilitycastMutation passes the class test for every activated
// ability: an exhaust watcher then sees loyalty activations (and the reverse),
// so the games must diverge.
func TestAbilitycastMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runAbilitycast(t, 64, true)
	if divs == 0 {
		t.Fatalf("class-ignoring mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
