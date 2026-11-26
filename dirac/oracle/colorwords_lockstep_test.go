package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Swirl the Mists (a Continuous ChangeColorWordsTo$ static over the spells
// and permanents, CR 612.5; kop.StatColorWords) in lockstep against gorge on
// the fixture pool dirac/cardset/pools/fixtures/colorwords. The kernel holds
// no per-object text, so it rewrites the colour words where the faces are
// read (dirac/static_colorwords.go: a target spec's filter masks and a
// printed Protection keyword of an affected spell or permanent) and stalls
// (dirac.StallTextChange) only where a rewritten word also sits on a surface
// the model does not read. Both halves are asserted:
//
//   - TestColorWordsLockstepNoOp: no card of the decks names a colour word,
//     so Swirl resolves, records a colour and stays live while the games run
//     clean and unstalled and the kernel agrees with gorge decision for
//     decision; the live counter proves the static was read.
//   - TestColorWordsLockstepReadsAgree: Doom Blade (target nonblack creature)
//     and Northern Paladin (target black permanent) name black, so a game that chose another colour reads their rewritten
//     faces (nonred, target red permanent) and still agrees with gorge
//     decision for decision, unstalled, with the rewrite counter proving the
//     masks moved.

const colorWordsPool = "colorwords"

var colorWordsNoOpDecks = [2][]cardset.Entry{
	{
		{Name: "Swirl the Mists", Count: 10}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Island", Count: 14}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Mountain", Count: 14}, {Name: "Plains", Count: 10},
	},
}

var colorWordsReadDecks = [2][]cardset.Entry{
	{
		{Name: "Swirl the Mists", Count: 8}, {Name: "Doom Blade", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Island", Count: 10}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Northern Paladin", Count: 8}, {Name: "Doom Blade", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Swamp", Count: 10},
	},
}

// colorWordsPlay plays seeds lo..hi of the pool and calls each with the
// game's result.
func colorWordsPlay(t *testing.T, decks [2][]cardset.Entry, lo, hi uint64, each func(seed uint64, res *Result)) {
	t.Helper()
	reg, set, gd := lockstepPool(t, colorWordsPool, decks)
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := lo; seed <= hi; seed++ {
		m, err := NewMatchup(reg, [2]string{colorWordsPool + "-a", colorWordsPool + "-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		each(seed, &res)
	}
}

func TestColorWordsLockstepNoOp(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, colorWordsPool, colorWordsNoOpDecks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Fatalf("seat %d: Swirl the Mists' deck is not playable in the %s pool", s, colorWordsPool)
		}
	}
	before := dirac.ColorWordsStats.Live.Load()
	seeds, clean := uint64(160), 0
	seedChunks(t, 1, seeds, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		n := 0
		colorWordsPlay(t, colorWordsNoOpDecks, lo, hi, func(seed uint64, res *Result) {
			if err := unmodelledStallError(t.Name(), res); err != nil {
				t.Errorf("seed %d: %v", seed, err)
			}
			if d := res.Div; d != nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
			n++
		})
		mu.Lock()
		clean += n
		mu.Unlock()
	})
	if t.Failed() {
		return
	}
	live := dirac.ColorWordsStats.Live.Load() - before
	t.Logf("%d seeds, %d clean; priority grants under a live colour-word static %d", seeds, clean, live)
	if live == 0 {
		t.Fatal("no priority grant found a live colour-word static with a recorded colour: the pool never reaches the mechanism")
	}
}

func TestColorWordsLockstepReadsAgree(t *testing.T) {
	t.Parallel()
	before := dirac.ColorWordsStats.Stalls.Load() + dirac.ColorWordsStats.Casts.Load()
	var mu sync.Mutex
	played := 0
	seedChunks(t, 1, 160, lockstepChunk, func(t *testing.T, lo, hi uint64, _ *sync.Mutex) {
		n := 0
		colorWordsPlay(t, colorWordsReadDecks, lo, hi, func(seed uint64, res *Result) {
			if err := unmodelledStallError(t.Name(), res); err != nil {
				t.Errorf("seed %d: %v", seed, err)
			}
			if d := res.Div; d != nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
			n++
		})
		mu.Lock()
		played += n
		mu.Unlock()
	})
	if t.Failed() {
		return
	}
	stalls := dirac.ColorWordsStats.Stalls.Load() + dirac.ColorWordsStats.Casts.Load() - before
	reads := dirac.ColorWordsStats.Reads.Load()
	t.Logf("%d seeds agree unstalled over the rewritten faces; %d rewritten mask reads, %d stalls", played, reads, stalls)
	if stalls != 0 {
		t.Errorf("%d unmodelled stalls: a deck card reads a rewritten colour word on a surface the model does not rewrite", stalls)
	}
	if reads == 0 {
		t.Fatal("no colour mask read a rewritten face: the decks never reach the model")
	}
}
