package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Pump Duration$ UntilEndOfCombat lockstep: the three corpus cards kgen
// lowers through kop.Override.EndCombat. Murk Dwellers gets +2/+0 when it
// attacks unblocked, Flummoxed Cyclops and Forgestoker Dragon's target can't
// block. gorge ends each record on the transition that leaves the
// end-of-combat step (CR 511.2), and at cleanup when combat was skipped; the
// kernel must copy that, or the pump outlives its combat and a later main
// phase or turn diverges. Battering Ram's pump grants Banding, which kgen
// refuses before the duration (KwMask has no free bit), so it cannot join the
// lockstep until that keyword has a manifest-keyword grant; the pool keeps it
// and the test asserts that measured refusal.
var kPumpEndCombatDeck = []cardset.Entry{
	{Name: "Murk Dwellers", Count: 4},
	{Name: "Flummoxed Cyclops", Count: 4},
	{Name: "Forgestoker Dragon", Count: 4},
	{Name: "Mountain", Count: 14},
	{Name: "Swamp", Count: 14},
}

// pumpEndCombatCards are the carriers whose UntilEndOfCombat registration the
// run counts. It is the precondition the lockstep needs: a game in which one
// never resolves cannot prove its expiry.
var pumpEndCombatCards = []string{
	"Murk Dwellers", "Flummoxed Cyclops", "Forgestoker Dragon",
}

func TestPumpEndCombatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("pumpendcombat")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kPumpEndCombatDeck {
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
	// Battering Ram is the ticket's fourth card, but its pump grants Banding,
	// which kgen refuses before it reaches the Duration arm. KwMask is full,
	// so the card needs a manifest-keyword grant before it can be lowered.
	if i, ok := set.Lookup("Battering Ram"); !ok {
		t.Fatal("Battering Ram not in the fixture pool")
	} else if st := set.Status(i); st != cardset.StatusNotLowered {
		t.Fatalf("Battering Ram status %s, want not lowered", st)
	} else if why := set.Reason(i); !strings.Contains(why, `keyword "Banding"`) {
		t.Fatalf("Battering Ram refusal %q, want the Banding keyword", why)
	}
	tbl, _, _, err := set.Matchup(kPumpEndCombatDeck, kPumpEndCombatDeck)
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[string]bool, len(pumpEndCombatCards))
	for _, name := range pumpEndCombatCards {
		want[name] = true
	}
	pumps := make(map[string]int, len(pumpEndCombatCards))
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kpump-a", "kpump-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{
			Config: cfg, MaxDecisions: 2500, MaxTurns: 25,
			Finish: func(g *GorgeSide) {
				// RegisteredContinuous keeps every registration, live or
				// expired, so a pump that resolved in any turn is visible.
				for _, ce := range g.E.RegisteredContinuous() {
					if !strings.EqualFold(ce.Duration, "UntilEndOfCombat") {
						continue
					}
					if ob := g.E.G.Obj(ce.Source); ob != nil && ob.Face() != nil && want[ob.Face().Name] {
						pumps[ob.Face().Name]++
					}
				}
			},
		})
		CloseSide(k)
		if res.StallUnmodelled {
			t.Errorf("seed %d: unmodelled stall: %s", seed, res.Stall)
			return
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	t.Logf("256 games, UntilEndOfCombat registrations %v", pumps)
	for _, name := range pumpEndCombatCards {
		if pumps[name] == 0 {
			t.Errorf("%s: no UntilEndOfCombat pump resolved in 256 games", name)
		}
	}
}
