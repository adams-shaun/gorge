package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The original Fey Steed / Angelic Cub pair, kept separate from the
// player-target and Kira pool: Fey Steed watches the comma list of permanent
// filters targeted by an opponent's spell, and Angelic Cub's FirstTime$ True
// self line queues once per turn (the Tf2FirstTime latch). The larger pool's
// attack-trigger target and reversal shapes are not what this test measures,
// so it keeps the simple burn-only deck it always had.
func becomestargetlistLatchDecks() [2][]cardset.Entry {
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

// btLatchPolicy casts and plays lands whenever it can, and aims every spell
// target at a battlefield Angelic Cub when one is legal (the Cub's own first
// trigger puts a +1/+1 counter on it, so a second Bolt in the same turn finds
// it still on the battlefield and exercises the once-per-turn latch), else
// random.
func btLatchPolicy(seed uint64) Policy {
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

// TestBecomesTargetListLatchLockstep is the FirstTime half of the
// BecomesTarget list arm: Angelic Cub's own latch and Fey Steed's comma-list
// watcher.
func TestBecomesTargetListLatchLockstep(t *testing.T) {
	st := &dirac.BtStats{}
	dirac.DebugBtStats = st
	recs := []dirac.CounterFirstRec{}
	dirac.CounterFirstLog = &recs
	defer func() {
		dirac.DebugBtStats = nil
		dirac.CounterFirstLog = nil
	}()
	lockstepRunPolicy(t, "becomestargetlist", becomestargetlistLatchDecks(), 40, true, nil, nil, btLatchPolicy)
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
