package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// ChangeZone's unconditional DestinationAlternative$ (no DestAltSVar$, no
// AlternativeDecider$; Forge handleAltDest asks the activator): the
// controller's top or bottom of a library -- Dream Cache's two hand cards
// after its draw (asked once after the move), Not Forgotten's graveyard
// target, Arashin Sovereign's optional dies trigger -- and Dina's
// Guidance's hand or graveyard after the pick, before the move. gorge
// (engine/effects zone_positionchoice.go, zone_search.go) leads; the kernel
// asks the same decisions (dirac/zone_destalt.go, kop.LibChoiceCtrlGrave).

// destaltCase is one matchup: only its card poses its kind of answer, so
// the counts show each card's ask was reached and answered both ways.
type destaltCase struct {
	name  string
	decks [2][]cardset.Entry
	dest  bool // Dina's Guidance's hand/graveyard, else top/bottom
}

var destaltCases = []destaltCase{
	{"DreamCache", [2][]cardset.Entry{
		{{Name: "Dream Cache", Count: 12}, {Name: "Grizzly Bears", Count: 8}, {Name: "Island", Count: 12}, {Name: "Forest", Count: 8}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Llanowar Elves", Count: 8}, {Name: "Forest", Count: 20}},
	}, false},
	{"NotForgotten", [2][]cardset.Entry{
		{{Name: "Not Forgotten", Count: 10}, {Name: "Murder", Count: 6}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 3}},
		{{Name: "Grizzly Bears", Count: 12}, {Name: "Llanowar Elves", Count: 8}, {Name: "Forest", Count: 20}},
	}, false},
	{"ArashinSovereign", [2][]cardset.Entry{
		{{Name: "Murder", Count: 12}, {Name: "Grizzly Bears", Count: 8}, {Name: "Swamp", Count: 12}, {Name: "Forest", Count: 8}},
		{{Name: "Arashin Sovereign", Count: 10}, {Name: "Llanowar Elves", Count: 8}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 14}},
	}, false},
	{"DinasGuidance", [2][]cardset.Entry{
		{{Name: "Dina's Guidance", Count: 12}, {Name: "Grizzly Bears", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 12}},
		{{Name: "Dina's Guidance", Count: 8}, {Name: "Llanowar Elves", Count: 12}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 12}},
	}, true},
}

func TestDestAltLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("destalt")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range destaltCases {
		t.Run(c.name, func(t *testing.T) { destaltRun(t, reg, set, c) })
	}
}

func destaltRun(t *testing.T, reg *cards.Registry, set cardset.Set, c destaltCase) {
	decks := kincrementDecksOf(t, reg, c.decks)
	tbl, _, _, err := set.Matchup(c.decks[0], c.decks[1])
	if err != nil {
		t.Fatal(err)
	}
	primary, _ := canon.ChooseCode("primary")
	bottom, _ := canon.ChooseCode("bottom")
	dest, _ := canon.ChooseCode("destination")
	var n [2]int // primary (top / hand), alternative (bottom / graveyard)
	stalls := 0
	for seed := uint64(1); seed <= 60; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"destalt-a", "destalt-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		for _, a := range res.Actions {
			switch k := a.Kind(); {
			case !c.dest && k == canon.ActChoose+canon.CanonKind(primary):
				n[0]++
			case !c.dest && k == canon.ActChoose+canon.CanonKind(bottom):
				n[1]++
			case c.dest && k == canon.ActChoose+canon.CanonKind(dest):
				n[a.Mode()&1]++
			}
		}
	}
	t.Logf("primary %d, alternative %d, kernel stalls %d", n[0], n[1], stalls)
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if n[0] < 5 || n[1] < 5 {
		t.Errorf("answers reached primary %d / alternative %d, want at least 5 of each", n[0], n[1])
	}
}
