package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Lwaxana Troi, Charming Guest lockstep: the keyword "You may choose not
// to untap CARDNAME during your untap step" (the untap-step election the
// kernel poses as ChGenKeepTapped) and the activated Effect whose
// Panharmonicon static, for as long as she stays tapped, makes the triggered
// abilities of the chosen creature trigger an additional time. The kernel's
// grant records must queue the same placements as gorge's queueTrigger at
// every decision, and the election must offer and apply the same options.

var klwaxanaDecks = [2][]cardset.Entry{
	{
		{Name: "Lwaxana Troi, Charming Guest", Count: 4}, {Name: "Burglar Rat", Count: 6}, {Name: "Crow of Dark Tidings", Count: 6},
		{Name: "Ashmouth Hound", Count: 4}, {Name: "Hill Giant", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Burglar Rat", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
	},
}

// lwaxanaStats are what a run of games reached.
type lwaxanaStats struct {
	games, divs, stalls int
	echoes              int // additional placements the grant demanded
	enterEchoes         int // ... of an enters trigger (Burglar Rat, Crow of Dark Tidings)
	keeps, untaps       int // keep-tapped elections answered each way
}

// runLwaxana plays seeds 1..seeds.
func runLwaxana(t *testing.T, seeds int, mutate bool) (st lwaxanaStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("lwaxana")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugLwaxanaNoGrant
	dirac.DebugLwaxanaNoGrant = mutate
	defer func() { dirac.DebugLwaxanaNoGrant = old }()
	var log []dirac.PanharmRec
	dirac.PanharmLog = &log
	defer func() { dirac.PanharmLog = nil }()
	var decks [2][]*cards.Card
	for s, d := range klwaxanaDecks {
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
	tbl, _, _, err := set.Matchup(klwaxanaDecks[0], klwaxanaDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	keepCode, ok1 := canon.ChooseCode("keep_tapped")
	untapCode, ok2 := canon.ChooseCode("untap")
	if !ok1 || !ok2 {
		t.Fatal("keep_tapped / untap are not in the choose vocabulary")
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"klwaxana-a", "klwaxana-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed) // casts and activates whenever it can
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		log = log[:0]
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		st.games++
		st.echoes += len(log)
		for _, r := range log {
			if !r.Dies {
				st.enterEchoes++
			}
		}
		for _, a := range res.Actions {
			switch a.Kind() {
			case canon.ActChoose + canon.CanonKind(keepCode):
				st.keeps++
			case canon.ActChoose + canon.CanonKind(untapCode):
				st.untaps++
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			if !mutate {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			st.divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return
}

func TestLwaxanaTroiLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runLwaxana(t, 256, false)
	t.Logf("%+v", st)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.echoes < 20 || st.enterEchoes < 10 {
		t.Errorf("only %d echoed placements, %d of enters triggers (want at least 20 and 10)", st.echoes, st.enterEchoes)
	}
	// the untap-step election is posed and answered both ways
	if st.keeps < 20 || st.untaps < 20 {
		t.Errorf("keep-tapped elections: %d kept, %d untapped (want at least 20 each)", st.keeps, st.untaps)
	}
}

// TestLwaxanaTroiMutation drops the Effect-delivered doubler: the games must
// diverge. A green mutation (no divergence) means the lockstep never reaches
// an echo.
func TestLwaxanaTroiMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runLwaxana(t, 32, true)
	if st.divs == 0 {
		t.Fatalf("mutation dropped every grant in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
