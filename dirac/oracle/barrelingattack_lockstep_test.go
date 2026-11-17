package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Barreling Attack's lockstep (dirac/kgen/lower_zone.go emblem over
// Mode$ AttackerBlocked; kernel trig_effectuntileot.go fireEmblemsBlocked): the
// targeted creature gets trample, and when it becomes blocked this turn it
// gets +1/+1 for each creature blocking it. The deck is vanilla 2/2s and 3/3s
// that block (several at once), burn that removes the targeted creature before
// blockers are declared (its capture lapses, as gorge forgets the object).
func barrelingAttackDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Barreling Attack", Count: 6}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Lightning Bolt", Count: 8},
		{Name: "Mountain", Count: 14},
	}
	return [2][]cardset.Entry{a, append([]cardset.Entry(nil), a...)}
}

// barrelingPolicy casts whenever it can (castFirstPolicy), and once a Barreling
// Attack's emblem is live it casts burn first and aims every target ask at the
// emblem's creature, so the creature often dies before blockers are declared.
func barrelingPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		var live state.ObjID
		if c.G != nil {
			for _, dt := range c.G.E.G.Delayed {
				if dt.EffectRepeat && dt.EventMode == "AttackerBlocked" && len(dt.Remembered) == 1 {
					live = dt.Remembered[0].Obj
				}
			}
		}
		buf := make([]Canon, 0, 8)
		for _, a := range c.D.Options {
			switch {
			case live != 0 && c.D.Kind == DTarget:
				if o, ok := c.G.GorgeOption(a); ok && o.Obj == live {
					return a
				}
			case c.D.Kind == DPriority && live != 0:
				if o, ok := c.G.GorgeOption(a); ok && o.Kind == "cast" {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Lightning Bolt" {
						buf = append(buf, a)
					}
				}
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestBarrelingAttackLockstep(t *testing.T) {
	st := &dirac.EffEOTTrigStats{}
	dirac.DebugEffEOTTrigStats = st
	defer func() { dirac.DebugEffEOTTrigStats = nil }()
	lockstepRunPolicy(t, "barrelingattack", barrelingAttackDecks(), 300, true, nil, nil, barrelingPolicy)
	if st.BlockedFires == 0 {
		t.Fatal("no become-blocked emblem trigger was queued in the games; the lockstep proves nothing")
	}
	if st.BlockedMulti == 0 {
		t.Error("no emblem trigger was queued for an attacker blocked by several creatures")
	}
	if st.BlockedLapsed == 0 {
		t.Error("no game passed over an attacker with a lapsed capture (the targeted creature left before blockers)")
	}
	t.Logf("become-blocked emblem fires %d (multi-blocked %d), lapsed captures %d", st.BlockedFires, st.BlockedMulti, st.BlockedLapsed)
}

// TestBarrelingAttackLockstepMutation: with the become-blocked emblem hook
// off, some seed diverges (gorge pumps the attacker; the kernel would not).
func TestBarrelingAttackLockstepMutation(t *testing.T) {
	dirac.DebugIgnoreBlockedEmblem = true
	defer func() { dirac.DebugIgnoreBlockedEmblem = false }()
	if lockstepRunPolicy(t, "barrelingattack", barrelingAttackDecks(), 300, false, nil, nil, barrelingPolicy) == 0 {
		t.Fatal("ignoring the become-blocked emblem left the lockstep clean")
	}
}
