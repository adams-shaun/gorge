package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Nog, Ambitious Cadet's Officer step grants "You may play lands from your
// graveyard" and "You may play an additional land on each of your turns" (CR
// 305.2, 611.3a). "You" is the controller of the permanent, so when Rules of
// Acquisition moves Nog to the other player the permission moves with him: the
// new controller may play its own lands from its own graveyard, and the old
// controller may not. Both decks run Nog, Rules of Acquisition and self-mill,
// so the control change happens in both directions; the policy plays lands
// first so the graveyard offer is reached.
type nogOfficerStats struct {
	officer, control, grave int
}

// nogOfficerPolicy plays a land first, then a cast or activation, else
// random, and counts the scenario's preconditions: an Officer Nog on the
// battlefield, a control change of Nog, and a land played from a graveyard.
func nogOfficerPolicy(seed uint64, st *nogOfficerStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	lastCtrl := -1
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			found := false
			for i := range g.Objs {
				o := &g.Objs[i]
				if o.Zone != state.ZBattlefield || o.Face() == nil || o.Face().Name != "Nog, Ambitious Cadet" {
					continue
				}
				found = true
				for _, ty := range c.G.E.Chars(o.ID).Types {
					if ty == "Officer" {
						st.officer++
						break
					}
				}
				if lastCtrl >= 0 && int(o.Controller) != lastCtrl {
					st.control++
				}
				lastCtrl = int(o.Controller)
				break
			}
			if !found {
				lastCtrl = -1
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate {
					buf = append(buf, a)
				}
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActPlayLand && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZGraveyard {
					st.grave++
				}
			}
		}
		return a
	})
}

func TestNogOfficerLandPlaysLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-trk")
	a := []cardset.Entry{
		{Name: "Nog, Ambitious Cadet", Count: 8},
		{Name: "Rules of Acquisition", Count: 4},
		{Name: "Cybernetic Specialist", Count: 4},
		{Name: "First Contact", Count: 4},
		{Name: "Forest", Count: 10},
		{Name: "Swamp", Count: 10},
	}
	b := []cardset.Entry{
		{Name: "Rules of Acquisition", Count: 4},
		{Name: "Cybernetic Specialist", Count: 4},
		{Name: "First Contact", Count: 4},
		{Name: "Island", Count: 14},
		{Name: "Forest", Count: 14},
	}
	var cs [2][]*cards.Card
	for s, d := range [2][]cardset.Entry{a, b} {
		n := 0
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d has %d cards", s, n)
		}
	}
	tbl, _, _, err := set.Matchup(a, b)
	if err != nil {
		t.Fatal(err)
	}
	var st nogOfficerStats
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"nog-officer-a", "nog-officer-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, nogOfficerPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if st.officer == 0 || st.control == 0 || st.grave == 0 {
		t.Fatalf("the games barely reached the grant: %d Officer decisions, %d Nog control changes, %d graveyard land plays", st.officer, st.control, st.grave)
	}
	t.Logf("%d Officer decisions, %d Nog control changes, %d graveyard land plays", st.officer, st.control, st.grave)
}
