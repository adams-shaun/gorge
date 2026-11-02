package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The lockstep: the object-side Attach pick (gorge
// effAttach, Choices$ with no Object$). Zack Fair's mandatory pick (any
// attached Equipment: gorge's Equipment.Attached admits every attached
// Equipment, anyone's, not only Zack Fair's) takes a lone candidate unasked,
// Vow to Erebor and Winter Soldier, Icy Assassin pose an up-to-one ask over
// the Equipment their controller has, and an empty eligible pool (no
// Equipment, or a gone destination) poses nothing. The decks carry the
// three Equipment with equip costs, Dwarves for Vow's condition, removal so
// a target sometimes leaves, discard and self-mill so Winter Soldier
// reaches the graveyard.

var kattachchoicesDecks = [2][]cardset.Entry{
	{
		{Name: "Zack Fair", Count: 4}, {Name: "Vow to Erebor", Count: 4},
		{Name: "Bonesplitter", Count: 2}, {Name: "Short Sword", Count: 1}, {Name: "Leather Armor", Count: 1},
		{Name: "Bastion Enforcer", Count: 4}, {Name: "Axgard Cavalry", Count: 4}, {Name: "Staunch Shieldmate", Count: 1}, {Name: "Dwarven Trader", Count: 1},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Winter Soldier, Icy Assassin", Count: 4}, {Name: "Tome Scour", Count: 4}, {Name: "Mind Rot", Count: 2},
		{Name: "Bonesplitter", Count: 2}, {Name: "Short Sword", Count: 2},
		{Name: "Bastion Enforcer", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Doom Blade", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Plains", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Island", Count: 3},
	},
}

var kattachchoicesCarriers = map[string]bool{"Zack Fair": true, "Vow to Erebor": true, "Winter Soldier, Icy Assassin": true}

// kattachchoicesTally counts, from gorge's decisions and events, the asks
// answered with an Equipment and declined, the mandatory one-candidate
// auto-takes (a carrier resolved, no ask, an Attach) and the resolutions
// with an empty eligible pool (a carrier resolved, no ask, no Attach).
type kattachchoicesTally struct {
	evAt              int
	lastSeq           uint64
	asked             bool
	opts              map[uint32]bool
	answered, decline int
	autoTake, empty   int
}

func (y *kattachchoicesTally) reset() { y.evAt, y.lastSeq, y.asked, y.opts = 0, 0, false, nil }

func (y *kattachchoicesTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < y.evAt {
		y.evAt = 0
	}
	resolved, attached, picked := false, false, false
	for j := y.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Resolve:
			// a carrier's ability (Zack Fair, Winter Soldier) or Vow's spell
			o := g.Obj(e.Obj)
			if o == nil {
				break
			}
			if o.Ability != nil {
				resolved = resolved || kattachchoicesCarriers[chName(g, o.Source)]
			} else {
				resolved = resolved || chName(g, e.Obj) == "Vow to Erebor"
			}
		case events.Attach:
			attached = true
			if y.asked && y.opts[uint32(e.Obj)] {
				picked = true
			}
		}
	}
	y.evAt = len(evs)
	if y.asked { // the previous decision was the ask: this window holds its answer
		if picked {
			y.answered++
		} else {
			y.decline++
		}
		y.asked = false
	}
	if d := ref.Pending(); d != nil && d.ResumeKind == "attach_choice" {
		if uint64(d.Seq) != y.lastSeq {
			y.lastSeq = uint64(d.Seq)
			y.asked = true
			y.opts = map[uint32]bool{}
			for _, o := range d.Options {
				y.opts[uint32(o.Obj)] = true
			}
		}
	} else if resolved {
		if attached {
			y.autoTake++
		} else {
			y.empty++
		}
	}
	return nil
}

func kattachchoicesSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("attachchoices")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kattachchoicesRun plays seeds 1..seeds and returns the first divergence's
// seed (0 none).
func kattachchoicesRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, y *kattachchoicesTally) (games int, divSeed uint64, stalls int) {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range kattachchoicesDecks {
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
	tbl, _, _, err := set.Matchup(kattachchoicesDecks[0], kattachchoicesDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kattachchoices-a", "kattachchoices-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		y.reset()
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: y.check})
		CloseSide(k)
		games++
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			if !dirac.DebugAttachPickAlwaysAsk {
				lo := max(d.At-12, 0)
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
			}
			return games, seed, stalls
		}
	}
	return games, 0, stalls
}

func TestAttachchoicesLockstep(t *testing.T) {
	t.Parallel()
	reg, set := kattachchoicesSetup(t)
	y := &kattachchoicesTally{}
	games, div, stalls := kattachchoicesRun(t, reg, set, 256, y)
	t.Logf("%d games, diverged seed %d, stalls %d; %d asks answered with an Equipment, %d declined, %d mandatory one-candidate auto-takes, %d empty-pool resolutions",
		games, div, stalls, y.answered, y.decline, y.autoTake, y.empty)
	if div != 0 || stalls != 0 {
		return
	}
	if y.answered < 15 {
		t.Errorf("%d asks answered with an Equipment, want at least 15", y.answered)
	}
	if y.decline < 5 {
		t.Errorf("%d asks declined, want at least 5", y.decline)
	}
	if y.autoTake < 5 {
		t.Errorf("%d mandatory one-candidate auto-takes, want at least 5", y.autoTake)
	}
	if y.empty < 5 {
		t.Errorf("%d empty-pool resolutions, want at least 5", y.empty)
	}
}

// TestAttachchoicesMutation: a kernel that asks even a mandatory pick's
// single candidate (gorge takes it unasked) must diverge.
func TestAttachchoicesMutation(t *testing.T) {
	reg, set := kattachchoicesSetup(t)
	dirac.DebugAttachPickAlwaysAsk = true
	defer func() { dirac.DebugAttachPickAlwaysAsk = false }()
	_, div, _ := kattachchoicesRun(t, reg, set, 600, &kattachchoicesTally{})
	if div == 0 {
		t.Fatalf("always-ask: no divergence in 600 seeds")
	}
	t.Logf("diverged at seed %d", div)
}
