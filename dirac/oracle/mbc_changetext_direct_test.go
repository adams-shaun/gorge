package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The direct SP$ ChangeText spellings (Sleight of Mind, Alter Reality,
// Glamerdye, Magical Hack; Spectral Shift's two DB$ ChangeText Charm modes,
// entwinable) over "target spell or permanent" (ValidTgts$ Card, TgtZone$
// Stack,Battlefield: kop.TgtPermOrSpell) in lockstep against gorge on the
// mbcchangetext fixture pool. Seat 0 holds the spells, seat 1 casts
// creatures they can target on the stack or on the battlefield.
//
// The policy picks only from words no permanent of the decks reads (the
// colour words, Swamp), so a resolution is the kernel's no-op. Magical
// Hack's own text reads Swamp ("swampwalk"), so a Hack aimed at a Hack spell
// stalls StallTextChange (the designed refusal to answer from an unchanged
// face); that stall is tolerated, any other unmodelled stall or a divergence
// is not. On even seeds the policy aims a target ask at a spell on the stack
// when one is offered (changeTextStackPolicy); the counters must show
// resolutions on a spell target.

var mbcChangeTextDirectDecks = [2][]cardset.Entry{
	{
		{Name: "Sleight of Mind", Count: 4}, {Name: "Alter Reality", Count: 4}, {Name: "Glamerdye", Count: 4},
		{Name: "Magical Hack", Count: 4}, {Name: "Spectral Shift", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 16},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Savannah Lions", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Plains", Count: 8},
	},
}

func TestMBCChangeTextDirectLowers(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, mbcChangeTextPool, mbcChangeTextDirectDecks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Fatalf("seat %d: the direct ChangeText deck is not playable in the %s pool", s, mbcChangeTextPool)
		}
	}
	asked0, spells0, reads0 := dirac.ChangeTextStats.Asked.Load(), dirac.ChangeTextStats.Spells.Load(), dirac.ChangeTextStats.Reads.Load()
	allow := func(w uint8) bool { return w < 5 || w == 7 }
	textStall := KernelStallPrefix + dirac.StallName(dirac.StallTextChange)
	seeds, clean := uint64(160), 0
	seedChunks(t, 1, seeds, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		n := 0
		mbcChangeTextPlayWith(t, mbcChangeTextDirectDecks, allow, changeTextStackPolicy, lo, hi, func(seed uint64, res *Result) {
			if d := res.Div; d != nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
			if res.Stall == textStall {
				return
			}
			if err := unmodelledStallError(t.Name(), res); err != nil {
				t.Errorf("seed %d: %v", seed, err)
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
	asked := dirac.ChangeTextStats.Asked.Load() - asked0
	spells := dirac.ChangeTextStats.Spells.Load() - spells0
	reads := dirac.ChangeTextStats.Reads.Load() - reads0
	t.Logf("%d seeds, %d clean; ChangeText resolutions %d (%d on a spell, %d read stalls)", seeds, clean, asked, spells, reads)
	if asked < 10 || spells < 5 {
		t.Fatalf("%d ChangeText resolutions, %d on a spell: the pool never reaches the direct spelling's stack target", asked, spells)
	}
	if clean < int(seeds)/2 {
		t.Fatalf("%d of %d seeds clean", clean, seeds)
	}
}

// changeTextStackPolicy: on an even seed, a target ask that offers a spell
// on gorge's stack picks the first such spell; otherwise base.
func changeTextStackPolicy(seed uint64, base Policy) Policy {
	if seed%2 != 0 {
		return base
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DTarget && c.G != nil {
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActTarget && o.Obj != 0 {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZStack {
						return a
					}
				}
			}
		}
		return base.Pick(c)
	})
}
