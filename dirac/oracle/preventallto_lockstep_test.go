package oracle

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kpreventalltoCorpusOrSkip skips when this tree has no corpus, including a
// tree that is no git repository at all (pr-precheck's fresh-clone run
// extracts HEAD with no .git, where corpusDirOrSkip fails rather than skips).
// It is each test's first statement, ahead of t.Parallel and isolate.
func kpreventalltoCorpusOrSkip(t *testing.T) {
	t.Helper()
	dir, ok := testutil.SharedCorpusDir(".")
	if !ok {
		t.Skip("no git repository, so no .cards corpus")
	}
	if _, err := os.Stat(cards.CorpusDir(dir)); err != nil {
		t.Skip("no .cards corpus present")
	}
}

// The unbounded prevent-all shield lockstep (Ext PreventNext C&2, kgen
// lower_preventallto.go; kernel repl_prevnext.go): "prevent all damage that
// would be dealt to target creature this turn" (Shielded Passage,
// Indestructible Aura) against burn and combat. Both seats play the same
// red-white deck, so a shield meets a Lightning Bolt or Shock aimed at the
// creature, a block and an attack. The expected answer at every decision is
// gorge's.
func kpreventalltoDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Shielded Passage", Count: 4}, {Name: "Indestructible Aura", Count: 4},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Savannah Lions", Count: 4}, {Name: "Raging Goblin", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// kpreventalltoShield reports whether gorge's option a casts a prevent-all
// spell.
func kpreventalltoShield(c *PickCtx, a Canon) bool {
	if a.Kind() != ActCast || c.G == nil {
		return false
	}
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return false
	}
	obj := c.G.E.G.Obj(o.Obj)
	if obj == nil || obj.Face() == nil {
		return false
	}
	n := obj.Face().Name
	return n == "Shielded Passage" || n == "Indestructible Aura"
}

// kpreventalltoPolicy is castFirstPolicy with at most one prevent-all spell
// cast a turn (so the games exercise one shield's prevention at a time),
// counting the damage events gorge logged as prevented by a replacement
// effect (the Note a Prevent$ True line leaves).
func kpreventalltoPolicy(prevented *atomic.Int64) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		var ref *GorgeSide
		seen := 0
		shieldTurn := int32(-1)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G == nil {
				return inner.Pick(c)
			}
			if c.G != ref {
				ref, seen, shieldTurn = c.G, 0, -1
			}
			evs := c.G.E.L.Events
			for ; seen < len(evs); seen++ {
				if e := &evs[seen]; e.Kind == events.Note && e.Text == "damage prevented by replacement effect" {
					prevented.Add(1)
				}
			}
			turn := c.G.E.G.Turn
			if c.D.Kind != DPriority {
				return inner.Pick(c)
			}
			d := *c.D
			d.Options = nil
			for _, a := range c.D.Options {
				if shieldTurn == turn && kpreventalltoShield(c, a) {
					continue
				}
				d.Options = append(d.Options, a)
			}
			pc := &PickCtx{D: &d, G: c.G, Seq: c.Seq}
			pick := inner.Pick(pc)
			if kpreventalltoShield(c, pick) {
				shieldTurn = turn
			}
			return pick
		})
	}
}

func TestPreventAllToLockstep(t *testing.T) {
	kpreventalltoCorpusOrSkip(t)
	t.Parallel()
	var prevented atomic.Int64
	reg, set, gd := lockstepPool(t, "preventallto", kpreventalltoDecks())
	seedChunks(t, 1, 300, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		lockstepSeeds(t, reg, set, "preventallto", kpreventalltoDecks(), gd, lo, hi, true, nil, nil, kpreventalltoPolicy(&prevented))
	})
	if prevented.Load() == 0 {
		t.Fatal("gorge never prevented a damage event in the games; the lockstep proves nothing")
	}
	t.Logf("damage events gorge logged as prevented: %d", prevented.Load())
}

// TestPreventAllToMutation: with the shields installed but preventing nothing
// (dirac.DebugNoPrevNext), some seed diverges.
func TestPreventAllToMutation(t *testing.T) {
	kpreventalltoCorpusOrSkip(t)
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevNext = true
	defer func() { dirac.DebugNoPrevNext = false }()
	var prevented atomic.Int64
	if lockstepRunPolicy(t, "preventallto", kpreventalltoDecks(), 200, false, nil, nil, kpreventalltoPolicy(&prevented)) == 0 {
		t.Fatal("no seed diverged with the shields dropped: the pool never has damage prevented by a prevent-all shield")
	}
}
