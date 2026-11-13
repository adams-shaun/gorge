package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCCounters3Lowers plays the MBC counters-3 spellings kgen lowers in
// lockstep with gorge over the mbccounters3 fixture pool
// (dirac/cardset/pools/fixtures/mbccounters3), one subtest per spelling:
//
//   - requiem: Vile Requiem's "up to X target nonblack creatures, X the verse
//     counters on CARDNAME" (TargetMax$ X over Count$CardCounters.VERSE), whose
//     bound reads the source's own counters as the slot is announced.
//
// Each subtest's probe counts the effect it exists for and fails when it never
// happened, so a refused card (the pool would not build) or a game that never
// reaches the shape fails as surely as a divergence.
func TestMBCCounters3Lowers(t *testing.T) {
	t.Parallel()
	t.Run("requiem", func(t *testing.T) {
		t.Parallel()
		verse, kills := 0, 0
		pol := func(seed uint64) Policy { return mbcCounters3Probe(mbcRequiemPolicy(seed), &verse, &kills) }
		if div := lockstepRunPolicy(t, "mbccounters3", mbcRequiemDecks, 120, true, nil, nil, pol); div != 0 {
			return
		}
		t.Logf("requiem: %d verse counters, %d destroys", verse, kills)
		if verse < 100 || kills < 20 {
			t.Fatalf("requiem: %d verse counters (want 100), %d destroys (want 20): the pool does not exercise it", verse, kills)
		}
	})
	t.Run("grimdancer", func(t *testing.T) {
		t.Parallel()
		picks, kept := 0, 0
		pol := func(seed uint64) Policy { return mbcGrimdancerProbe(castFirstPolicy(seed), &picks, &kept) }
		if div := lockstepRunPolicy(t, "mbccounters3", mbcGrimdancerDecks, 120, true, nil, nil, pol); div != 0 {
			return
		}
		t.Logf("grimdancer: %d counter-kind asks, %d keyword counters placed", picks, kept)
		if picks < 60 || kept < 120 {
			t.Fatalf("grimdancer: %d counter-kind asks (want 60), %d keyword counters (want 120): the pool does not exercise it", picks, kept)
		}
	})
}

// mbcRequiemPolicy is castFirstPolicy that never sacrifices a Vile Requiem
// holding no verse counter (castFirstPolicy would, the moment it resolves, and
// the up-to-X slot would only ever be announced at X = 0).
func mbcRequiemPolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.D.Kind != DPriority {
			return base.Pick(c)
		}
		d := *c.D
		d.Options = nil
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate && refCardName(c.G, a.Src()) == "Vile Requiem" {
				if o := c.G.E.G.Obj(c.G.id.obj[a.Src()]); o == nil || o.Counter("VERSE") == 0 {
					continue
				}
			}
			d.Options = append(d.Options, a)
		}
		return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// mbcCounters3Probe wraps base and counts, when gorge leads, the verse
// counters placed and the Savannah Lions deaths its log gains (combat
// kills them too, but Vile Requiem is the pool's only way to lose several in
// one turn; the count only proves the games reach creature deaths).
func mbcCounters3Probe(base Policy, verse, kills *int) Policy {
	seen := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		evs := c.G.E.L.Events
		for ; seen < len(evs); seen++ {
			switch e := evs[seen]; {
			case e.Kind == events.CounterChange && strings.EqualFold(e.Counter, "VERSE") && e.Amount > 0:
				*verse++
			case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard:
				if o := c.G.E.G.Obj(e.Obj); o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Savannah Lions" {
					*kills++
				}
			}
		}
		return a
	})
}

// mbcRequiemDecks: Vile Requiem and the mana for it on one side, nonblack
// bodies for it to destroy on the other.
var mbcRequiemDecks = [2][]cardset.Entry{
	{{Name: "Vile Requiem", Count: 12}, {Name: "Swamp", Count: 28}},
	{{Name: "Savannah Lions", Count: 16}, {Name: "Plains", Count: 24}},
}

// mbcGrimdancerProbe wraps base and counts, when gorge leads, the
// "counter_kinds" asks it poses and the keyword counters its log gains.
func mbcGrimdancerProbe(base Policy, picks, kept *int) Policy {
	seen := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		if fr := c.G.fr; fr != nil && fr.d != nil && fr.d.ResumeKind == "counter_kinds" {
			*picks++
		}
		evs := c.G.E.L.Events
		for ; seen < len(evs); seen++ {
			if e := evs[seen]; e.Kind == events.CounterChange && e.Amount > 0 && e.Counter != "VERSE" {
				*kept++
			}
		}
		return a
	})
}

// mbcGrimdancerDecks: Grimdancers to cast on both sides.
var mbcGrimdancerDecks = [2][]cardset.Entry{
	{{Name: "Grimdancer", Count: 16}, {Name: "Swamp", Count: 24}},
	{{Name: "Grimdancer", Count: 10}, {Name: "Savannah Lions", Count: 6}, {Name: "Swamp", Count: 12}, {Name: "Plains", Count: 12}},
}
