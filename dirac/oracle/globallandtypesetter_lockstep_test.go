package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGlobalLandTypeSetterLockstep plays the global CR 305.7 land-type
// setters kgen lowers (Blood Moon, Magus of the Moon, Harbinger of the Seas, Conversion:
// RemoveLandTypes$ True with AddType$ Mountain / Island over Land.nonBasic;
// dirac/kgen/lower_landtypesetter.go) in lockstep with gorge over the
// globallandtypesetter fixture pool. The decks hold original duals, a pain
// land and basics: under a setter every nonbasic land is only the new basic
// type, its own mana abilities (and the pain land's damage) gone and the new
// type's intrinsic one in its place, while basics keep theirs. The
// precondition, measured from gorge's side, is that a setter was on the
// battlefield while a nonbasic land of another type was tapped for mana.
//
// The mutation keeps the land's own mana abilities beside the intrinsic one
// (dirac.DebugLandSetOwnMana): a Bayou under Blood Moon then taps for {B}
// and {G} too, and the kernel must diverge.
func TestGlobalLandTypeSetterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, taps := runGlobalLandSetLockstep(t, 24, true)
		t.Logf("%d games, %d divergences, %d stalls; nonbasic off-type taps under a setter: %d", run.games, run.divs, run.stalls, taps)
		if taps < 6 {
			t.Errorf("nonbasic lands without the setter's type tapped under a setter %d times: want at least 6", taps)
		}
		for _, n := range []string{"Blood Moon", "Magus of the Moon", "Harbinger of the Seas", "Conversion"} {
			if run.casts[n] == 0 {
				t.Errorf("%s was never cast", n)
			}
		}
	})
	t.Run("mutation-own-mana", func(t *testing.T) {
		dirac.DebugLandSetOwnMana = true
		defer func() { dirac.DebugLandSetOwnMana = false }()
		run, _ := runGlobalLandSetLockstep(t, 24, false)
		run.requireDivergence(t, "a global land-type setter's land keeping its own mana abilities")
	})
}

var globalLandSetDecks = []fixtureDecks{{
	{{Name: "Blood Moon", Count: 4}, {Name: "Magus of the Moon", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 3}, {Name: "Bayou", Count: 4}, {Name: "Savannah", Count: 4}, {Name: "Underground River", Count: 4}, {Name: "Wastes", Count: 3}},
	{{Name: "Harbinger of the Seas", Count: 4}, {Name: "Coral Merfolk", Count: 6}, {Name: "Typhoid Rats", Count: 6}, {Name: "Savannah Lions", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 4}, {Name: "Tropical Island", Count: 4}, {Name: "Bayou", Count: 3}, {Name: "Underground River", Count: 3}},
}, {
	{{Name: "Blood Moon", Count: 3}, {Name: "Harbinger of the Seas", Count: 4}, {Name: "Coral Merfolk", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Typhoid Rats", Count: 3},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 2}, {Name: "Tropical Island", Count: 4}, {Name: "Underground River", Count: 4}, {Name: "Bayou", Count: 4}},
	{{Name: "Magus of the Moon", Count: 4}, {Name: "Blood Moon", Count: 3}, {Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 7},
		{Name: "Forest", Count: 5}, {Name: "Plains", Count: 5}, {Name: "Savannah", Count: 5}, {Name: "Bayou", Count: 3}, {Name: "Wastes", Count: 2}},
}, {
	// Conversion ("All Mountains are Plains": the filter reads the Mountain
	// subtype, so basic Mountains are set too) against a Mountain deck
	{{Name: "Conversion", Count: 4}, {Name: "Savannah Lions", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Savannah", Count: 6}, {Name: "Forest", Count: 4}, {Name: "Wastes", Count: 4}},
	{{Name: "Raging Goblin", Count: 12}, {Name: "Typhoid Rats", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 4}, {Name: "Bayou", Count: 4}, {Name: "Underground River", Count: 4}},
}}

var globalLandSetWatch = map[string]bool{"Blood Moon": true, "Magus of the Moon": true, "Harbinger of the Seas": true, "Conversion": true}

// globalLandSetAdds is the basic land type each watched setter makes the
// nonbasic lands.
var globalLandSetAdds = map[string]string{"Blood Moon": "Mountain", "Magus of the Moon": "Mountain", "Harbinger of the Seas": "Island"}

// globalLandSetTap reports whether the Tap event e taps a nonbasic land whose
// printed types lack the type of a setter on the battlefield.
func globalLandSetTap(g *state.Game, e events.Event) bool {
	land := g.Obj(e.Obj)
	if land == nil || land.Face() == nil || !land.Face().HasTypeWord("Land") {
		return false
	}
	conv := land.Face().HasTypeWord("Mountain") // Conversion's "All Mountains are Plains"
	if land.Face().HasTypeWord("Basic") && !conv {
		return false
	}
	for _, q := range g.AliveFrom(0) {
		for _, a := range g.Zone(state.ZBattlefield, q) {
			o := g.Obj(a)
			if o == nil || o.Face() == nil {
				continue
			}
			if o.Face().Name == "Conversion" && conv {
				return true
			}
			if w, ok := globalLandSetAdds[o.Face().Name]; ok && !land.Face().HasTypeWord("Basic") && !land.Face().HasTypeWord(w) {
				return true
			}
		}
	}
	return false
}

// runGlobalLandSetLockstep plays seeds 1..seeds of the matchups, gorge
// leading, and returns the run and the measured precondition.
func runGlobalLandSetLockstep(t *testing.T, seeds int, report bool) (fixtureRun, int) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("globallandtypesetter")
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(globalLandSetDecks))
	tbls := make([]*kop.Table, len(globalLandSetDecks))
	for mi, md := range globalLandSetDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for range e.Count {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		if tbls[mi], _, _, err = set.Matchup(md[0], md[1]); err != nil {
			t.Fatal(err)
		}
	}
	run := fixtureRun{casts: map[string]int{}}
	taps := 0
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % len(globalLandSetDecks)
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"globallandset-a", "globallandset-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
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
				if evs[seen].Kind == events.Tap && globalLandSetTap(ref.E.G, evs[seen]) {
					taps++
				}
			}
			return nil
		}
		res := LockstepLed(m, seed, g, k, fixtureWatchPolicy(seed, globalLandSetWatch, run.casts), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
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
	return run, taps
}
