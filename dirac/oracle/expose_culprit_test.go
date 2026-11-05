package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Expose the Culprit's lockstep: the Charm's first mode turns a face-down
// creature face up (kop.ExtTurnFaceUp), its second exiles any number of
// face-up disguise creatures face down and cloaks every one (Defined$
// Remembered, kop.CfCloakRemembered; Shuffle$ is gorge's pile shuffle, no
// library shuffle and no RNG draw).
var kexposeDeck = []cardset.Entry{
	{Name: "Expose the Culprit", Count: 6},
	{Name: "Basilica Stalker", Count: 8},
	{Name: "Vengeful Creeper", Count: 8},
	{Name: "Priest of Gix", Count: 4},
	{Name: "Mountain", Count: 5},
	{Name: "Swamp", Count: 6},
	{Name: "Forest", Count: 3},
}

type kexposeCounts struct {
	games       int
	modeFirst   int // ActMode answers taking mode 0 (the turn-up)
	modeSecond  int // ActMode answers taking mode 1 (the exile and cloak)
	picks       int // hidden_pick answers taken
	pairs       int // second-mode resolutions that cloaked two or more cards
	run         int
	turnUps     int // gorge events.TurnFaceUp (the special action is never taken)
	faceDownCst int // creature casts made face down
	shuffles    int // events.Shuffle after an Expose cast
}

// kexposePolicy casts disguise creatures face down (from Priest of Gix's floating mana) or up, answers
// the Charm's mode ask with the second mode unless a face-down creature of
// the asker's is on the battlefield, takes every hidden_pick before Done, and
// never takes the turn-face-up special action.
func kexposePolicy(seed uint64, n *kexposeCounts) Policy {
	r := rand.New(rand.NewPCG(seed, 0x65787073)) // "exps"
	code, _ := canon.ChooseCode("hidden_pick")
	pickKind := canon.ActChoose + canon.CanonKind(code)
	heu := HeuristicPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		var pick, done *Canon
		var modes [2]*Canon
		var faceCasts []Canon
		for i := range c.D.Options {
			a := &c.D.Options[i]
			switch a.Kind() {
			case pickKind:
				if pick == nil {
					pick = a
				}
			case canon.ActDone:
				done = a
			case canon.ActMode:
				if m := a.Mode(); m < 2 {
					modes[m] = a
				}
			case canon.ActCastFace:
				faceCasts = append(faceCasts, *a)
			}
		}
		if pick != nil {
			n.picks++
			n.run++
			return *pick
		}
		if done != nil && n.run > 0 {
			if n.run >= 2 {
				n.pairs++
			}
			n.run = 0
			return *done
		}
		if n.run >= 2 {
			n.pairs++ // the pick ended without a Done: the pool was exhausted
		}
		n.run = 0
		if modes[0] != nil && modes[1] != nil {
			if r.IntN(2) == 0 {
				n.modeFirst++
				return *modes[0]
			}
			n.modeSecond++
			return *modes[1]
		}
		if len(faceCasts) > 0 && r.IntN(3) != 0 {
			n.faceDownCst++
			return faceCasts[r.IntN(len(faceCasts))]
		}
		return heu.Pick(c)
	})
}

// kexposeFinish counts the turn-ups and flags a library shuffle after an
// Expose the Culprit cast.
func kexposeFinish(n *kexposeCounts) func(g *GorgeSide) {
	return func(g *GorgeSide) {
		cast := false
		for _, ev := range g.E.L.Events {
			switch ev.Kind {
			case events.CastInfo:
				if o := g.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Expose the Culprit" {
					cast = true
				}
			case events.Shuffle:
				if cast {
					n.shuffles++
				}
			case events.TurnFaceUp:
				n.turnUps++
			}
		}
	}
}

func TestExposeTheCulpritLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exposeculprit")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kexposeDeck {
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
	tbl, _, _, err := set.Matchup(kexposeDeck, kexposeDeck)
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(64)
	if testing.Short() {
		seeds = 32
	}
	var n kexposeCounts
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"exp-a", "exp-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		n.run = 0
		res := Lockstep(m, seed, k, kexposePolicy(seed, &n), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30, Finish: kexposeFinish(&n)})
		CloseSide(k)
		n.games++
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%+v", n)
	if n.modeSecond < 1 || n.picks < 2 || n.pairs < 1 {
		t.Errorf("exile mode answered %d times, %d picks, %d resolutions cloaking two or more: want >= 1, >= 2, >= 1", n.modeSecond, n.picks, n.pairs)
	}
	if n.modeFirst < 1 || n.turnUps < 1 {
		t.Errorf("turn-up mode answered %d times, %d turn-ups: want >= 1 each", n.modeFirst, n.turnUps)
	}
	if n.faceDownCst < 1 {
		t.Errorf("no face-down cast")
	}
	if n.shuffles != 0 {
		t.Errorf("%d library shuffles after an Expose the Culprit cast", n.shuffles)
	}
}
