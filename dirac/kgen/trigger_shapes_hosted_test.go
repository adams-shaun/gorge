package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The hosted Commander trigger shapes this ticket lowers: Hercules,
// Olympian Hero (DamageDoneOnce FirstTime$), Coveted Jewel
// (AttackerUnblockedOnce), Harsh Mentor (AbilityCast ValidActivatingPlayer$
// Opponent + ValidCard + ValidSA SpellAbility.!ManaAbility) and Folk Hero
// (a once-each-turn trigger on a granted ability).
var triggerShapeCards = []string{
	"Hercules, Olympian Hero", "Coveted Jewel", "Harsh Mentor", "Folk Hero",
}

// triggerShapeFillers are the decks' ordinary cards: attackers and burn for
// Hercules, non-mana activated abilities for Harsh Mentor (Prodigal
// Sorcerer, Shivan Dragon) and creatures for Coveted Jewel's unblocked
// attacks.
var triggerShapeFillers = []string{
	"Raging Goblin", "Prodigal Sorcerer", "Shivan Dragon", "Lightning Bolt", "Shock",
	"Plains", "Mountain", "Island",
}

// triggerShapeDecks is one 40-card deck per seat. A is the shapes' seat; B
// supplies unblocked attackers, burn and activations.
func triggerShapeDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Hercules, Olympian Hero", Count: 4},
			{Name: "Coveted Jewel", Count: 4},
			{Name: "Harsh Mentor", Count: 4},
			{Name: "Folk Hero", Count: 4},
			{Name: "Raging Goblin", Count: 4},
			{Name: "Plains", Count: 10},
			{Name: "Mountain", Count: 10},
		},
		{
			{Name: "Prodigal Sorcerer", Count: 4},
			{Name: "Shivan Dragon", Count: 4},
			{Name: "Lightning Bolt", Count: 4},
			{Name: "Shock", Count: 4},
			{Name: "Island", Count: 12},
			{Name: "Mountain", Count: 12},
		},
	}
}

// TestTriggerShapesHostedLockstep lowers each hosted card and plays the four
// shapes in a real lockstep against gorge. The test asserts the shapes were
// reached (the kernel counters below), not just that the games were quiet.
func TestTriggerShapesHostedLockstep(t *testing.T) {
	cardsDir := ratchetCards(t)

	// Every shape lowers and builds: no refusal from the LIR pass or the
	// generator.
	lw := lowerNamedPool(t, triggerShapeCards)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatalf("build: %v", err)
	}
	seen := map[string]bool{}
	for _, c := range lw.cards {
		for _, want := range triggerShapeCards {
			if c.Name != want {
				continue
			}
			seen[want] = true
			if len(c.Refusals) != 0 {
				t.Errorf("%s refusals = %v, want none", want, c.Refusals)
			}
		}
	}
	for _, want := range triggerShapeCards {
		if !seen[want] {
			t.Errorf("%s was not lowered", want)
		}
	}

	names := append(append([]string{}, triggerShapeCards...), triggerShapeFillers...)
	p, err := loadCardNames(names, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	pd, _, _, err := tableOf(p, "trigger-shapes", nil)
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	decks := triggerShapeDecks()
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range decks[s] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}

	st := &dirac.AbilityCastStats{}
	dirac.DebugAbilityCastStats = st
	defer func() { dirac.DebugAbilityCastStats = nil }()
	var firstLog []dirac.CounterFirstRec
	dirac.CounterFirstLog = &firstLog
	defer func() { dirac.CounterFirstLog = nil }()
	dirac.DebugUnblockedOnceFires = 0
	defer func() { dirac.DebugUnblockedOnceFires = 0 }()

	for seed := uint64(1); seed <= 32; seed++ {
		m, err := oracle.NewMatchup(reg, [2]string{"trigger-shapes-a", "trigger-shapes-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		tbl, _, _, err := set.Matchup(decks[0], decks[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := oracle.Config{}
		if seed%4 == 0 {
			cfg = oracle.Config{GorgeAutopaySurface: true}
		}
		k, err := oracle.SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := sweepCapPolicy(seed)
		if seed%8 == 0 {
			pol = oracle.RandomPolicy(seed)
		}
		res := oracle.Lockstep(m, seed, k, pol, oracle.RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		oracle.CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}

	// The games above must actually reach each shape. Hercules's first-time
	// DamageDone trigger logs its fired instance; Harsh Mentor's
	// non-mana-activated class is its own counter; Coveted Jewel's
	// unblocked-once queue is the third.
	herc := 0
	for _, r := range firstLog {
		if r.Card == "Hercules, Olympian Hero" && r.Fired {
			herc++
		}
	}
	if herc == 0 {
		t.Errorf("the pool never dealt Hercules damage (no FirstTime DamageDone trigger fired)")
	}
	if n := st.Fires[kop.TrigAbCastActivated&7]; n == 0 {
		t.Errorf("the pool never activated a non-mana ability watched by Harsh Mentor")
	}
	if dirac.DebugUnblockedOnceFires == 0 {
		t.Errorf("the pool never attacked Coveted Jewel's controller with an unblocked creature")
	}
	t.Logf("Hercules first-time fires %d, Harsh Mentor activations %d, Coveted Jewel unblocked-once fires %d",
		herc, st.Fires[kop.TrigAbCastActivated&7], dirac.DebugUnblockedOnceFires)
}
