package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestChooseTypeLandLockstep plays Traveler's Cloak (an as-enters Type$ Land
// ask feeding the static "enchanted creature has landwalk of the chosen type")
// in lockstep with gorge over the choosetypesetter fixture pool. Both decks
// run Cloaks beside vanilla creatures and Swamp / Island / Forest lands, and
// the policy answers the 18-label ask with a basic type three times in four, so
// a Cloak most often names a type the defender controls: the enchanted
// attacker is then unblockable (CR 702.14) on both surfaces. The remaining
// answers are any label, nonbasic ones (no card of the pool prints the type;
// the answer is 255 on the kernel) included. The preconditions, measured from
// gorge's side, are that Cloaks were cast and chose a basic type, and, from the
// kernel's, that an Aura's granted landwalk decided a block
// (DebugLandwalkStats.AttachedEvades).
func TestChooseTypeLandLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run, st, ev := runChooseTypeLand(t, 24)
	t.Logf("%d games, %d divergences, %d stalls; Cloaks entered %d, basic answers %d, other answers %d; evades %+v", run.games, run.divs, run.stalls, st.entered, st.basic, st.other, ev)
	if st.entered < 8 || st.basic < 4 || st.other < 1 {
		t.Errorf("Cloaks entered %d times, basic answers %d, other answers %d: want at least 8, 4 and 1", st.entered, st.basic, st.other)
	}
	if ev.AttachedEvades == 0 {
		t.Error("no game made an attacker unblockable through a Traveler's Cloak's granted landwalk")
	}
}

// chooseTypeLandDecks are the matchup's decks (40 cards each).
var chooseTypeLandDecks = fixtureDecks{
	{{Name: "Traveler's Cloak", Count: 8}, {Name: "Coral Merfolk", Count: 6}, {Name: "Typhoid Rats", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 2}},
	{{Name: "Traveler's Cloak", Count: 6}, {Name: "Typhoid Rats", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Raging Goblin", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 6}, {Name: "Island", Count: 4}},
}

// chooseTypeLandStats are the preconditions measured from gorge's side.
type chooseTypeLandStats struct {
	entered int // Cloaks that entered the battlefield
	basic   int // type answers that named a basic land type
	other   int // type answers that named any other label
}

// chooseTypeLandPolicy plays randomly but answers a land-type ask with a
// Swamp, Island or Forest three times in four.
func chooseTypeLandPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	pick := RandomPolicy(seed ^ 0x6c616e64) // "land"
	return PolicyFunc(func(c *PickCtx) Canon {
		a := rnd.Pick(c)
		if c.G == nil || c.D.Kind != DChoose || len(c.D.Options) < 18 {
			return a
		}
		var basic []Canon
		for _, o := range c.D.Options {
			// a label-valued choice's value rides (target, mode) (optionAction)
			switch v := uint16(o.Tgt())<<8 | uint16(o.Mode()); v {
			case labelValue("Swamp"), labelValue("Island"), labelValue("Forest"):
				basic = append(basic, o)
			}
		}
		if len(basic) == 0 || c.Seq%4 == 0 {
			return a
		}
		d := *c.D
		d.Options = basic
		return pick.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runChooseTypeLand plays seeds 1..seeds of the Cloak matchup, gorge leading,
// alternating the option surface by seed.
func runChooseTypeLand(t *testing.T, seeds int) (fixtureRun, chooseTypeLandStats, dirac.LandwalkStats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("choosetypesetter")
	if err != nil {
		t.Fatal(err)
	}
	md := chooseTypeLandDecks
	decks := [2][]*cards.Card{}
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	ev := &dirac.LandwalkStats{}
	dirac.DebugLandwalkStats = ev
	defer func() { dirac.DebugLandwalkStats = nil }()
	run := fixtureRun{casts: map[string]int{}}
	var st chooseTypeLandStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"chooseland-a", "chooseland-b"}, decks, 1)
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
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				e := evs[seen]
				if e.Kind != events.Choose || e.Counter != "type" {
					continue
				}
				switch e.Text {
				case "Swamp", "Island", "Forest":
					st.basic++
				default:
					st.other++
				}
			}
			return nil
		}
		res := LockstepLed(m, seed, g, k, chooseTypeLandPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			run.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %s %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Detail, d.Diff, d.RefOptions, d.KerOptions)
			if run.divs >= 3 {
				break
			}
		}
	}
	st.entered = st.basic + st.other // each Cloak asks once as it enters
	return run, st, *ev
}
