package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestMBCGrantedAfterlifeLowers plays Afterlife Insurance (MBC: a PumpAll
// granting afterlife 1 to the creatures you control, then a draw) in
// lockstep with gorge over the mbcafterlife fixture pool. The grant is a
// latent dies trigger (the printed K:Afterlife expansion's shape, from
// scriptfacts.GrantedKeywordTrigger) listed on each creature the PumpAll
// swept until end of turn; a creature that dies that turn makes its 1/1 Spirit
// (CR 702.135a). The grant is read from the pre-departure board (CR 603.10a),
// which gorge's granted walk does in the leaves-the-battlefield pass only.
//
// Preconditions, measured from gorge's side: the Insurance was cast and its
// PumpAll granted afterlife (a __kwAfterlifeGranted:1 GrantTriggerPush), and a
// granted trigger put Spirit tokens onto the battlefield. The mutation drops
// the OverrideAll the lowering emits: the kernel then makes no Spirit and must
// diverge.
func TestMBCGrantedAfterlifeLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, st := runGrantedAfterlife(t, 40, true, nil)
		t.Logf("%d games, %d divergences, %d stalls; Insurance casts %d, granted pushes %d, Spirits %d",
			run.games, run.divs, run.stalls, run.casts["Afterlife Insurance"], st.pushes, st.spirits)
		if run.casts["Afterlife Insurance"] < 8 {
			t.Errorf("Afterlife Insurance was cast %d times, want at least 8", run.casts["Afterlife Insurance"])
		}
		if st.pushes < 4 {
			t.Errorf("a granted afterlife trigger was pushed %d times, want at least 4", st.pushes)
		}
		if st.spirits < 4 {
			t.Errorf("granted afterlife made %d Spirit tokens, want at least 4", st.spirits)
		}
	})
	t.Run("mutation-OverrideAll", func(t *testing.T) {
		run, _ := runGrantedAfterlife(t, 40, false, func(tbl *kop.Table) {
			mutateCard(t, tbl, "Afterlife Insurance", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpOverrideAll {
					*in = kop.Instr{Op: kop.OpNop}
				}
			})
		})
		run.requireDivergence(t, "Afterlife Insurance without its granted dies trigger")
	})
}

var grantedAfterlifeDecks = []fixtureDecks{{
	{{Name: "Afterlife Insurance", Count: 8}, {Name: "Savannah Lions", Count: 8}, {Name: "Glory Seeker", Count: 6}, {Name: "Plains", Count: 9}, {Name: "Swamp", Count: 9}},
	{{Name: "Afterlife Insurance", Count: 8}, {Name: "Gray Ogre", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 9}},
}}

var grantedAfterlifeWatch = map[string]bool{"Afterlife Insurance": true}

// grantedAfterlifeStats are the preconditions measured from gorge's side.
type grantedAfterlifeStats struct {
	pushes, spirits int // granted afterlife triggers pushed; Spirit tokens that entered
}

// runGrantedAfterlife plays seeds 1..seeds of the mbcafterlife matchup, gorge
// leading, alternating the option surface by seed; mutate rewrites the table
// first.
func runGrantedAfterlife(t *testing.T, seeds int, report bool, mutate func(tbl *kop.Table)) (fixtureRun, grantedAfterlifeStats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcafterlife")
	if err != nil {
		t.Fatal(err)
	}
	md := grantedAfterlifeDecks[0]
	var decks [2][]*cards.Card
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
	if mutate != nil {
		mutate(tbl)
	}
	run := fixtureRun{casts: map[string]int{}}
	var st grantedAfterlifeStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"afterlife-a", "afterlife-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, fixtureWatchPolicy(seed, grantedAfterlifeWatch, run.casts), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		for _, ev := range g.E.L.Events {
			if ev.Kind == events.GrantTriggerPush && ev.Counter == "__kwAfterlifeGranted:1" {
				st.pushes++
			}
		}
		for _, o := range g.E.G.Objs {
			if o.IsToken && o.Face() != nil && o.Face().Name == "Spirit Token" {
				st.spirits++
			}
		}
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, st
}
