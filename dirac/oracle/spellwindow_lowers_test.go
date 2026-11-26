package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestSpellWindowLowers: a spell's own timing restriction in lockstep over the
// spellwindow fixture pool. gorge's spellTimingPre reads activationPhasesOK on
// the spell's SA at the cast offer (CR 601.3): ActivationPhases$ ("Cast this
// spell only during combat", "... before blockers are declared", "... during
// your end step"), PlayerTurn$ True and OpponentTurn$ True. kgen lowers each
// onto the spell ability's cast restriction (lower_actwindow.go spellWindow),
// which the kernel's castForbidden withholds the cast by
// (cast_spellcondition.go). The finish hook counts each pool spell's casts
// off gorge's event log and fails when one never reached the stack, so the
// lockstep compares the restricted offers, not just their absence.
var kspellwinDecks = [2][]cardset.Entry{
	{
		{Name: "Savage Beating", Count: 2}, {Name: "Panic", Count: 2}, {Name: "Gorilla War Cry", Count: 2},
		{Name: "Disharmony", Count: 2}, {Name: "Berserk", Count: 2}, {Name: "Blood Frenzy", Count: 2}, {Name: "Seedtime", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Take the Bait", Count: 2}, {Name: "Teleport", Count: 2}, {Name: "Necrologia", Count: 2},
		{Name: "Angelic Favor", Count: 2}, {Name: "Surprise Deployment", Count: 2},
		{Name: "Savannah Lions", Count: 4}, {Name: "Serra Angel", Count: 2}, {Name: "Wind Drake", Count: 3}, {Name: "Memnite", Count: 3},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 3},
	},
}

var kspellwinSpells = []string{"Savage Beating", "Panic", "Gorilla War Cry", "Disharmony", "Berserk", "Blood Frenzy", "Seedtime",
	"Take the Bait", "Teleport", "Necrologia", "Angelic Favor", "Surprise Deployment"}

func kspellwinFinish(casts map[string]int) func(*GorgeSide) {
	want := map[string]bool{}
	for _, n := range kspellwinSpells {
		want[n] = true
	}
	return func(g *GorgeSide) {
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind != events.PutOnStack {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && want[o.Face().Name] {
				casts[o.Face().Name]++
			}
		}
	}
}

func TestSpellWindowLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("spellwindow")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, kspellwinDecks)
	tbl, _, _, err := set.Matchup(kspellwinDecks[0], kspellwinDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	casts := map[string]int{}
	for _, pol := range []func(uint64) Policy{castFirstPolicy, ExplorePolicy} {
		for seed := uint64(1); seed <= 48; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ksw-a", "ksw-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 20, Finish: kspellwinFinish(casts)})
			CloseSide(k)
			if s := res.Stall; len(s) >= len(KernelStallPrefix) && s[:len(KernelStallPrefix)] == KernelStallPrefix {
				t.Fatalf("seed %d: kernel stall %s", seed, s)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	for _, n := range kspellwinSpells {
		t.Logf("%s cast %d times on gorge's side", n, casts[n])
		if casts[n] == 0 {
			t.Errorf("%s was never cast: the pool does not reach its window", n)
		}
	}
}
