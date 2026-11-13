package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Mind Bend (a GenericChoice whose bodies are DB$ ChangeText: a colour word,
// a basic land type; CR 612) in lockstep against gorge on the fixture pool
// dirac/cardset/pools/fixtures/mbcchangetext. The kernel holds no
// per-permanent text, so it models the change where it is a no-op (the
// target's face never reads the from word) and stalls where it is not
// (dirac.StallTextChange). Both halves are asserted:
//
//   - TestMBCChangeTextLowers: the policy picks only from words no card of
//     the decks reads (every colour word, Swamp), so every resolution is the
//     no-op and the games must stay clean and unstalled; the kernel's counter
//     must show the picks were asked.
//   - TestMBCChangeTextReadsStall: the policy picks a basic land type the
//     lands read as the from word, so a Mind Bend on such a land stalls the
//     kernel (unmodelled) instead of leaving the unchanged land standing, and
//     no seed diverges before it.

const mbcChangeTextPool = "mbcchangetext"

var mbcChangeTextDecks = [2][]cardset.Entry{
	{
		{Name: "Mind Bend", Count: 8}, {Name: "Whim of Volrath", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Pacifism", Count: 2},
		{Name: "Island", Count: 14}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Savannah Lions", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Plains", Count: 8},
	},
}

// changeTextFromPolicy removes, from a ChangeText word ask, the "from" words
// allow rejects (by word index: colours 0..4, basic land types 5..9), then
// defers to base.
type changeTextFromPolicy struct {
	base  Policy
	allow func(word uint8) bool
}

func (p *changeTextFromPolicy) Pick(c *PickCtx) Canon {
	from, _ := chooseCode("changetext_from")
	fromKind := ActChoose + CanonKind(from)
	keep := make([]Canon, 0, len(c.D.Options))
	for _, o := range c.D.Options {
		if o.Kind() == fromKind && !p.allow(o.Mode()) {
			continue
		}
		keep = append(keep, o)
	}
	if len(keep) == len(c.D.Options) || len(keep) == 0 {
		return p.base.Pick(c)
	}
	d := *c.D
	d.Options = keep
	return p.base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
}

// mbcChangeTextPlay plays seeds lo..hi of the pool and calls each with the
// game's result.
func mbcChangeTextPlay(t *testing.T, allow func(uint8) bool, lo, hi uint64, each func(seed uint64, res *Result)) {
	t.Helper()
	reg, set, gd := lockstepPool(t, mbcChangeTextPool, mbcChangeTextDecks)
	tbl, _, _, err := set.Matchup(mbcChangeTextDecks[0], mbcChangeTextDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := lo; seed <= hi; seed++ {
		m, err := NewMatchup(reg, [2]string{mbcChangeTextPool + "-a", mbcChangeTextPool + "-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := &changeTextFromPolicy{base: castFirstPolicy(seed), allow: allow}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		each(seed, &res)
	}
}

func TestMBCChangeTextLowers(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, mbcChangeTextPool, mbcChangeTextDecks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Fatalf("seat %d: Mind Bend's deck is not playable in the %s pool", s, mbcChangeTextPool)
		}
	}
	before := dirac.ChangeTextStats.Asked.Load()
	// the colour words and Swamp: no card of the decks reads any of them
	allow := func(w uint8) bool { return w < 5 || w == 7 }
	seeds, clean := uint64(160), 0
	seedChunks(t, 1, seeds, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		n := 0
		mbcChangeTextPlay(t, allow, lo, hi, func(seed uint64, res *Result) {
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
	asked := dirac.ChangeTextStats.Asked.Load() - before
	t.Logf("%d seeds, %d clean; ChangeText resolutions %d", seeds, clean, asked)
	if asked < 10 {
		t.Fatalf("%d ChangeText resolutions (< 10): the pool never reaches the mechanism", asked)
	}
}

func TestMBCChangeTextReadsStall(t *testing.T) {
	t.Parallel()
	before := dirac.ChangeTextStats.Reads.Load()
	// the four basic land types the lands read (Plains, Island, Mountain, Forest)
	allow := func(w uint8) bool { return w >= 5 && w != 7 }
	var mu sync.Mutex
	stalls := 0
	seedChunks(t, 1, 160, lockstepChunk, func(t *testing.T, lo, hi uint64, _ *sync.Mutex) {
		n := 0
		mbcChangeTextPlay(t, allow, lo, hi, func(seed uint64, res *Result) {
			if d := res.Div; d != nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
				return
			}
			if res.Stall == KernelStallPrefix+dirac.StallName(dirac.StallTextChange) {
				if !res.StallUnmodelled {
					t.Errorf("seed %d: the text-change stall is not classed unmodelled", seed)
				}
				n++
			}
		})
		mu.Lock()
		stalls += n
		mu.Unlock()
	})
	if t.Failed() {
		return
	}
	reads := dirac.ChangeTextStats.Reads.Load() - before
	t.Logf("%d games stalled on a read from word; %d reads", stalls, reads)
	if stalls == 0 || reads < int64(stalls) {
		t.Fatalf("%d stalled games, %d reads: no Mind Bend reached a land that reads its from word", stalls, reads)
	}
}
