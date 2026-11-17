package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// turnFaceUpValidCauseDeck: Growing Dread is the Mode$ TurnFaceUp
// ValidCause$ SpellAbility.YouCtrl watcher; Manifest Dread and the
// Disguise/Morph carriers put a permanent onto the battlefield face down, and
// Priest of Urabrask floats the mana the CR 708.6 special action spends (the
// actor ValidCause$ names: the controller taking the action). Dog Walker is
// the disguise carrier, Manifest Dread the manifest-dread one.
var turnFaceUpValidCauseDeck = []cardset.Entry{
	{Name: "Growing Dread", Count: 4},
	{Name: "Manifest Dread", Count: 4},
	{Name: "Dog Walker", Count: 4},
	{Name: "Warbreak Trumpeter", Count: 4},
	{Name: "Priest of Urabrask", Count: 4},
	{Name: "Forest", Count: 6},
	{Name: "Mountain", Count: 6},
	{Name: "Plains", Count: 4},
	{Name: "Island", Count: 4},
}

// turnFaceUpValidCausePolicy casts Growing Dread first, then lets
// FaceDownPolicy take a turn-up special action (the actor ValidCause$ reads)
// or a face-down cast. The base policy fills the rest of the game.
func turnFaceUpValidCausePolicy(seed uint64, upOffers *int) Policy {
	fd := FaceDownPolicy(AffordableTargetPolicy(RandomPolicy(seed)))
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActSpecial && isTurnFaceUp(a) {
				*upOffers++
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			if f := optionFace(c, a); f != nil && f.Name == "Growing Dread" {
				return a
			}
		}
		return fd.Pick(c)
	})
}

// turnFaceUpValidCauseFinish counts the turn-ups and Growing Dread's pushed
// TurnFaceUp triggers, the precondition the lockstep proves.
func turnFaceUpValidCauseFinish(turnUps, pushes *int) func(g *GorgeSide) {
	return func(g *GorgeSide) {
		for _, ev := range g.E.L.Events {
			if ev.Kind == events.TurnFaceUp {
				*turnUps++
				continue
			}
			if ev.Kind != events.TriggerPush {
				continue
			}
			o := g.E.G.Obj(ev.Obj)
			if o == nil || o.Face() == nil || int(ev.Amount) >= len(o.Face().Triggers) {
				continue
			}
			if o.Face().Name == "Growing Dread" && strings.EqualFold(o.Face().Triggers[ev.Amount].Mode, "TurnFaceUp") {
				*pushes++
			}
		}
	}
}

func TestTurnFaceUpValidCauseLockstep(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("turnfaceupvalidcause")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range turnFaceUpValidCauseDeck {
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
	tbl, _, _, err := set.Matchup(turnFaceUpValidCauseDeck, turnFaceUpValidCauseDeck)
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 60
	pushes, turnUps, upOffers := new(int), new(int), new(int)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"gu-a", "gu-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, turnFaceUpValidCausePolicy(seed, upOffers), RunConfig{
			Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Finish: turnFaceUpValidCauseFinish(turnUps, pushes),
		})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: stall %q", seed, res.Stall)
		}
	}
	if *pushes == 0 {
		t.Fatalf("Growing Dread's TurnFaceUp trigger never fired over %d seeds (%d turn-ups, %d up offers): the ValidCause$ gate is not exercised", seeds, *turnUps, *upOffers)
	}
	t.Logf("seeds=%d turn-ups=%d upOffers=%d Growing Dread TurnFaceUp triggers=%d", seeds, *turnUps, *upOffers, *pushes)
}
