package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
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
//     masks moved. Raging Ravine (an Animate activation's word-list Colors$
//     Red,Green) sits in play and activates: its override record reads the
//     colour-word colour frozen at its activation
//     (dirac/static_colorwords.go ovCw), unstalled.

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
		{Name: "Swirl the Mists", Count: 8}, {Name: "Doom Blade", Count: 6}, {Name: "Raging Ravine", Count: 8},
		{Name: "Island", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 5},
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
	colorWordsPlayPol(t, decks, lo, hi, castFirstPolicy, each)
}

// colorWordsPlayPol is colorWordsPlay with the seed's policy.
func colorWordsPlayPol(t *testing.T, decks [2][]cardset.Entry, lo, hi uint64, pol func(uint64) Policy, each func(seed uint64, res *Result)) {
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
		res := Lockstep(m, seed, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
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

// colorWordsActivatePolicy casts and plays lands when offered, else
// activates an ability (the read decks' only one is the Ravine's Animate),
// else random: the game reaches the Ravine's animation without starving its
// casts.
func colorWordsActivatePolicy(seed uint64) Policy {
	rnd := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, acts := range [][]Canon{
			canonOf(c, ActCast, ActCastFace, ActPlayLand),
			canonOf(c, ActActivate),
		} {
			if len(acts) > 0 {
				d := *c.D
				d.Options = acts
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

// canonOf is c.D's options of these action kinds.
func canonOf(c *PickCtx, kinds ...canon.CanonKind) []Canon {
	var buf []Canon
	for _, a := range c.D.Options {
		k := a.Kind()
		for _, want := range kinds {
			if k == want {
				buf = append(buf, a)
				break
			}
		}
	}
	return buf
}

func TestColorWordsLockstepReadsAgree(t *testing.T) {
	t.Parallel()
	before := dirac.ColorWordsStats.Stalls.Load() + dirac.ColorWordsStats.Casts.Load()
	beforeInst := dirac.ColorWordsStats.Installs.Load()
	var mu sync.Mutex
	played, acts := 0, 0
	seedChunks(t, 1, 160, lockstepChunk, func(t *testing.T, lo, hi uint64, _ *sync.Mutex) {
		n, na := 0, 0
		colorWordsPlayPol(t, colorWordsReadDecks, lo, hi, colorWordsActivatePolicy, func(seed uint64, res *Result) {
			if err := unmodelledStallError(t.Name(), res); err != nil {
				t.Errorf("seed %d: %v", seed, err)
			}
			if d := res.Div; d != nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
			n++
			for _, a := range res.Actions {
				if a.Kind() == ActActivate {
					na++ // the Ravine's Animate: the frozen colour-word colour its record reads
				}
			}
		})
		mu.Lock()
		played += n
		acts += na
		mu.Unlock()
	})
	if t.Failed() {
		return
	}
	stalls := dirac.ColorWordsStats.Stalls.Load() + dirac.ColorWordsStats.Casts.Load() - before
	reads := dirac.ColorWordsStats.Reads.Load()
	installs := dirac.ColorWordsStats.Installs.Load() - beforeInst
	t.Logf("%d seeds agree unstalled over the rewritten faces; %d Ravine activations, %d frozen-colour record installs, %d rewritten mask reads, %d stalls", played, acts, installs, reads, stalls)
	if stalls != 0 {
		t.Errorf("%d unmodelled stalls: a deck card reads a rewritten colour word on a surface the model does not rewrite", stalls)
	}
	if reads == 0 {
		t.Fatal("no colour mask read a rewritten face: the decks never reach the model")
	}
	if acts == 0 || installs == 0 {
		t.Fatal("no Ravine activation under a live colour-word colour: the decks never reach the frozen colour-word colour of an Animate Colors$")
	}
}
