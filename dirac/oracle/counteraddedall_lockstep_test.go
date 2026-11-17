package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Bioessence Hydra's "whenever one or more loyalty counters are put on
// planeswalkers you control, put that many +1/+1 counters on CARDNAME" and
// Cloaked Cadet's "whenever one or more +1/+1 counters are put on one or more
// Humans you control, draw a card; only once each turn" are Mode$
// CounterAddedAll watchers (dirac/trig_counteraddedall.go). The deck plays
// Chandra's loyalty ability (the planeswalker batch), Luminarch Aspirant, a
// Human that puts +1/+1 counters on a Human every combat, and Cathars'
// Crusade, whose PutCounterAll is one multi-recipient batch.
var kcounteraddedallDecks = [2][]cardset.Entry{
	{
		{Name: "Bioessence Hydra", Count: 4}, {Name: "Cloaked Cadet", Count: 4},
		{Name: "Luminarch Aspirant", Count: 4}, {Name: "Chandra, Fire Artisan", Count: 4},
		{Name: "Cathars' Crusade", Count: 4},
		{Name: "Forest", Count: 9}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 3}, {Name: "Mountain", Count: 3},
	},
	{
		{Name: "Bioessence Hydra", Count: 4}, {Name: "Cloaked Cadet", Count: 4},
		{Name: "Luminarch Aspirant", Count: 4}, {Name: "Chandra, Fire Artisan", Count: 4},
		{Name: "Cathars' Crusade", Count: 4},
		{Name: "Forest", Count: 9}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 3}, {Name: "Mountain", Count: 3},
	},
}

// kcounteraddedallPolicy casts the watchers and their enablers and plays
// lands whenever it can, else random; it counts, from gorge's options, the
// watcher activations (a Hydra or Cadet on the battlefield is enough for a
// cast to have resolved).
func kcounteraddedallPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		var pref []Canon
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActCastFace, ActActivate, ActPlayLand:
			default:
				continue
			}
			o, ok := c.G.GorgeOption(a)
			if !ok {
				continue
			}
			name := ""
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
				name = ob.Face().Name
			}
			switch name {
			case "Bioessence Hydra", "Cloaked Cadet", "Luminarch Aspirant", "Chandra, Fire Artisan", "Cathars' Crusade":
				pref = append(pref, a)
			default:
				if a.Kind() == ActPlayLand {
					pref = append(pref, a)
				}
			}
		}
		if len(pref) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = pref
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestCounterAddedAllLockstep(t *testing.T) {
	t.Parallel()
	oldFires, oldJoins := dirac.DebugCtrAllFires, dirac.DebugCtrAllJoins
	dirac.DebugCtrAllFires, dirac.DebugCtrAllJoins = 0, 0
	defer func() { dirac.DebugCtrAllFires, dirac.DebugCtrAllJoins = oldFires, oldJoins }()
	lockstepRunPolicy(t, "ctraddedall", kcounteraddedallDecks, 60, true, nil, nil, kcounteraddedallPolicy)
	if dirac.DebugCtrAllFires == 0 {
		t.Fatal("no CounterAddedAll trigger fired in the games; the lockstep proves nothing")
	}
	t.Logf("CounterAddedAll watcher events: %d (batch joins: %d)", dirac.DebugCtrAllFires, dirac.DebugCtrAllJoins)
}
