package oracle

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestScratchR2Div runs matchup 3's diverging seeds only and dumps both boards
// and the pending asks at the first divergence. Scratch debug only; not for
// commit.
func TestScratchR2Div(t *testing.T) {
	reg, set := kvsEnv(t)
	md := kvsDecks[3]
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	decks := kvsDecksFor(t, reg, md)
	seed := uint64(1)
	if v := os.Getenv("SCRATCH_SEED"); v != "" {
		n, _ := strconv.ParseUint(v, 10, 64)
		seed = n
	}
	{
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			m, err := NewMatchup(reg, [2]string{"kvs-a", "kvs-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%4 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			defer CloseSide(k)
			o := &kvsObs{counts: map[string]int{}, seed: seed, surf: "window", turn: 1, entry: map[state.ObjID]int{}, used: map[state.ObjID]bool{}, held: map[[2]int]bool{}}
			if os.Getenv("DKTRACE") != "" {
				dirac.DKPlanTrace(func(f string, a ...any) {
					t.Logf("DK "+f, a...)
				})
				defer dirac.DKPlanTrace(nil)
			}
			pol := kvsStaturePolicy(seed, o)
			hooked := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil && c.G.OnInternal == nil {
					o.g = c.G
					c.G.OnInternal = o.hook
				}
				return pol.Pick(c)
			})
			var lastRef *GorgeSide
			check := func(ref *GorgeSide, _ Side) *Divergence {
				lastRef = ref
				o.scan(ref)
				return nil
			}
			res := Lockstep(m, seed, k, hooked, RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 30, Check: check})
			div := res.Div
			if div == nil {
				t.Logf("seed %d: no divergence", seed)
				t.Fatalf("dump")
				return
			}
			t.Logf("seed %d diverged at %d: %s %s %v %s", seed, div.At, div.What, div.Field, div.Diff, div.Detail)
			t.Logf("ref opts %v", div.RefOptions)
			t.Logf("ker opts %v", div.KerOptions)
			if pr, ok := projectAt(t, lastRef, k, []int{57, 58, 59, 65, 66, 67, 68}); ok {
				t.Logf("cards %v", pr)
			}
			acts := actionStrings(res.Actions)
			alo := div.At - 15
			if alo < 0 {
				alo = 0
			}
			ahi := div.At + 3
			if ahi > len(acts) {
				ahi = len(acts)
			}
			t.Logf("actions[%d:%d] %v", alo, ahi, acts[alo:ahi])
			t.Logf("total actions %d", len(acts))
			if lastRef == nil {
				return
			}
			evs := lastRef.E.L.Events
			lo := div.At - 12
			if lo < 0 {
				lo = 0
			}
			for j := lo; j < len(evs) && j < div.At+4; j++ {
				e := evs[j]
				name := ""
				if ob := lastRef.E.G.Obj(e.Obj); ob != nil && ob.Face() != nil {
					name = ob.Face().Name
				}
				t.Logf("ev %d kind=%d p=%d obj=%d %s ctr=%q amt=%d", j, e.Kind, e.Player, e.Obj, name, e.Counter, e.Amount)
			}
			t.Logf("== gorge board at div ==")
			for _, ob := range lastRef.E.G.Objs {
				if ob.Face() == nil || ob.Zone != state.ZBattlefield {
					continue
				}
				t.Logf("gorge obj %d %s ctrl=%d tapped=%v", int(ob.ID), ob.Face().Name, int(ob.Controller), ob.Tapped)
			}
			for pi, pl := range lastRef.E.G.Players {
				t.Logf("gorge p%d pool=%v", pi, pl.Pool)
			}
			if d := lastRef.E.Pending(); d != nil {
				t.Logf("gorge pending kind=%v player=%d prompt=%q", d.Kind, int(d.Player), d.Prompt)
				for _, oo := range d.Options {
					t.Logf("  opt kind=%q idx=%d amount=%d obj=%d", oo.Kind, oo.Index, oo.Amount, int(oo.Obj))
				}
			}
			if real, ok := k.(*realSide); ok {
				var p dirac.Projection
				real.s.Project(&p)
				t.Logf("== kernel board ==")
				for i := dirac.Obj(0); i < p.NObj; i++ {
					po := &p.Objs[i]
					if po.Zone != kop.ZoneBattlefield {
						continue
					}
					name := "?"
					if int(po.Card) < len(real.gname) {
						name = real.gname[po.Card]
					}
					t.Logf("ker obj %d %s flags=%d tapped=%v ctrl=%d", int(i), name, po.Flags, real.s.Tapped(i), po.Ctrl)
				}
				for pi := 0; pi < 2; pi++ {
					t.Logf("ker p%d pool=%v", pi, p.Pool[pi])
				}
			}
			t.Fatalf("dump")
		})
	}
}

func projectAt(t *testing.T, ref *GorgeSide, ker Side, idx []int) ([]string, bool) {
	t.Helper()
	if ref == nil {
		return nil, false
	}
	var pr, pk Projection
	ref.Project(&pr)
	ker.Project(&pk)
	out := []string{}
	for _, i := range idx {
		if i >= int(pr.NInst) || i >= int(pk.NInst) {
			continue
		}
		a, b := &pr.Cards[i], &pk.Cards[i]
		out = append(out, fmt.Sprintf("c%d ref{card=%d zone=%d ctrl=%d pos=%d f=%d} ker{card=%d zone=%d ctrl=%d pos=%d f=%d}",
			i, a.Card, a.Zone, a.Controller, a.Pos, a.Flags, b.Card, b.Zone, b.Controller, b.Pos, b.Flags))
	}
	return out, true
}
