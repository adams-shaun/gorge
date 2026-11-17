package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Fey Steed's and Angelic Cub's lockstep (dirac/kgen/lower_trigger.go's
// BecomesTarget list and FirstTime arms; kernel trig_becomestarget.go's
// btWatchers and trig_counterfirst.go's Tf2FirstTime latch): Fey Steed
// watches a comma list of permanent filters (a creature or planeswalker you
// control) targeted by an opponent's spell or ability and may draw a card;
// Angelic Cub's FirstTime$ True self line puts a +1/+1 counter on it the
// first time it is targeted each turn (a second Lightning Bolt on the same
// Cub in one turn triggers nothing, the next turn's first one does). The
// burn can also aim at a player, which neither form answers.
//
// The other named carriers do not play here: Kira, Great Glass-Spinner's
// Counter body refuses (TriggeredSourceSA outside a SpellCast trigger) and
// Dormant Gomazoa's player target is the pre-existing inert lowering (the
// kernel's targetedOne walk sees object targets only, the ticket's PARTIAL
// player-target item) — either would diverge by design.
func becomestargetlistDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Fey Steed", Count: 4},
		{Name: "Angelic Cub", Count: 4},
		{Name: "Lightning Bolt", Count: 8},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Hill Giant", Count: 4},
		{Name: "Plains", Count: 8},
		{Name: "Mountain", Count: 2},
		{Name: "Forest", Count: 2},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// btcastFirstPolicy casts, activates and plays lands whenever it can (so the
// burn flies and creatures sit on the battlefield as targets), and aims every
// spell target at a battlefield Angelic Cub when one is legal (the Cub's own
// first trigger puts a +1/+1 counter on it, so a second Bolt in the same turn
// finds it still on the battlefield and exercises the once-per-turn latch),
// else random.
func btcastFirstPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DTarget {
			for _, a := range c.D.Options {
				o, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZBattlefield &&
					ob.Face() != nil && ob.Face().Name == "Angelic Cub" {
					return a
				}
			}
		}
		buf := make([]Canon, 0, 8)
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActCastFace, ActActivate, ActPlayLand:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 || len(buf) == len(c.D.Options) {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// Serial, not t.Parallel: the test installs the process-global kernel hooks
// dirac.DebugBtStats and dirac.CounterFirstLog. Other tests' kernel games run
// counterFirstUsed/markCounterFirst concurrently and would race the slice
// header (or panic when the defer nils the pointer between a nil check and a
// deref); e.g. enterprised_test.go is serial for the same reason. Serial
// tests finish before the package's parallel tests resume, so there is no
// overlap.
func TestBecomesTargetListLockstep(t *testing.T) {
	st := &dirac.BtStats{}
	dirac.DebugBtStats = st
	recs := []dirac.CounterFirstRec{}
	dirac.CounterFirstLog = &recs
	defer func() {
		dirac.DebugBtStats = nil
		dirac.CounterFirstLog = nil
	}()
	lockstepRunPolicy(t, "becomestargetlist", becomestargetlistDecks(), 60, true, nil, nil, btcastFirstPolicy)
	if st.Queues == 0 {
		t.Fatal("no BecomesTarget trigger was queued in the games; the lockstep proves nothing")
	}
	var fired, refused int
	for _, r := range recs {
		if r.Card != "Angelic Cub" {
			continue
		}
		if r.Fired {
			fired++
		} else {
			refused++
		}
	}
	if fired == 0 {
		t.Fatal("Angelic Cub's FirstTime line never queued in the games; the lockstep proves nothing")
	}
	if refused == 0 {
		t.Fatal("the FirstTime latch never refused a later targeting in the games; the once-per-turn gate went untested")
	}
	t.Logf("BecomesTarget trigger queues: %d; Angelic Cub latch fired %d, refused %d", st.Queues, fired, refused)
}
