package oracle

import (
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
	"testing"
)

// TestScratchStatureDiv runs matchup 3 (Stature/Mind Spring/Island) seed 1 and
// dumps both engines' boards at the first divergence. Scratch debug only; not
// for commit.
func TestScratchStatureDiv(t *testing.T) {
	reg, set := kvsEnv(t)
	md := kvsDecks[3]
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	decks := kvsDecksFor(t, reg, md)
	for seed := uint64(1); seed <= uint64(256); seed++ {
		m, err := NewMatchup(reg, [2]string{"kvs-a", "kvs-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		o := &kvsObs{counts: map[string]int{}, seed: seed, surf: "window", turn: 1, entry: map[state.ObjID]int{}, used: map[state.ObjID]bool{}, held: map[[2]int]bool{}}
		pol := kvsStaturePolicy(seed, o)
		hooked := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.G.OnInternal == nil {
				o.g = c.G
				c.G.OnInternal = o.hook
			}
			if os.Getenv("KVS_POL") != "" && c.G != nil {
				g := c.G.E.G
				acts := []string{}
				for _, a := range c.D.Options {
					if a.Kind() == ActActivate {
						if ob := g.Obj(state.ObjID(a.Src())); ob != nil && ob.Face() != nil {
							acts = append(acts, fmt.Sprintf("%d:%s", int(a.Src()), ob.Face().Name))
						}
					}
				}
				var xs []string
				pend := c.G.Pending()
				var srcName string
				if pend != nil {
					if src := g.Obj(pend.Source); src != nil && src.Face() != nil {
						srcName = src.Face().Name
					}
				}
				if c.D.Kind == DChoose && srcName == "Stature, Size Shifter" {
					for _, a := range c.D.Options {
						xs = append(xs, fmt.Sprint(a.Mode()))
					}
				}
				pool := g.Players[c.D.Player].Pool
				total := 0
				for _, v := range pool {
					total += int(v)
				}
				if os.Getenv("KVS_POL2") != "" && c.D.Kind == DPriority {
					sts := []string{}
					usedAny := false
					for _, ob := range g.Objs {
						if ob.Face() != nil && ob.Face().Name == "Stature, Size Shifter" && ob.Zone == state.ZBattlefield && ob.Controller == state.PlayerID(c.D.Player) {
							sts = append(sts, fmt.Sprintf("%d:e%d:t%v", int(ob.ID), o.entry[ob.ID], o.used[ob.ID]))
							usedAny = usedAny || o.used[ob.ID]
						}
					}
					offStature := false
					for _, a := range acts {
						offStature = offStature || len(a) > 0
					}
					if os.Getenv("KVS_FLOAT") != "" && total > 0 && int(pool[1]) == total {
						t.Logf("FLOATSTATE p%d poolU=%d total=%d o.turn=%d statures=%v", c.D.Player, int(pool[1]), total, o.turn, sts)
					}
					if (pool[1] == 1 && total == 1 && len(sts) > 0) || (total >= 2 && pool[1] >= 2 && len(sts) > 0 && usedAny) {
						t.Logf("COUNTSTATE pick %s: acts=%v poolU=%d total=%d o.turn=%d statures=%v", c.D.Kind, acts, pool[1], total, o.turn, sts)
					}
				}
				if len(acts) > 0 || len(xs) > 0 {
					t.Logf("pick %s: acts=%v xs(src=%s)=%v poolU=%d total=%d", c.D.Kind, acts, srcName, xs, pool[1], total)
				}
			}
			return pol.Pick(c)
		})
		var lastRef *GorgeSide
		dumped := false
		n := 0
		prevTapped := map[int]bool{}
		prevG := map[int]bool{}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			lastRef = ref
			o.scan(ref)
			n++
			real, ok := k.(*realSide)
			if real != nil && ok {
				for i := 0; i < real.s.NumObjs(); i++ {
					ob := dirac.Obj(i)
					tp := real.s.Tapped(ob)
					if tp && !prevTapped[i] {
						t.Logf("check %d: ker obj %d tapped", n, i)
					} else if !tp && prevTapped[i] {
						t.Logf("check %d: ker obj %d untapped", n, i)
					}
					prevTapped[i] = tp
				}
			}
			for _, ob := range ref.E.G.Objs {
				if ob.Face() == nil {
					continue
				}
				i := int(ob.ID)
				tp := ob.Tapped
				if tp && !prevG[i] {
					t.Logf("check %d: gorge obj %d (%s) tapped", n, i, ob.Face().Name)
				} else if !tp && prevG[i] {
					t.Logf("check %d: gorge obj %d (%s) untapped", n, i, ob.Face().Name)
				}
				prevG[i] = tp
			}
			if os.Getenv("KVS_OFFERS") != "" {
				acts := []string{}
				for _, a := range ref.E.Pending().Options {
					if a.Kind == "activate" {
						if ob := ref.E.G.Obj(state.ObjID(a.Obj)); ob != nil && ob.Face() != nil {
							acts = append(acts, fmt.Sprintf("%d:%s", int(a.Obj), ob.Face().Name))
						}
					}
				}
				if len(acts) > 0 {
					t.Logf("check %d: gorge offers activate %v pool=%v", n, acts, ref.E.G.Players[ref.E.Pending().Player].Pool)
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, hooked, RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 30, Check: check})
		if res.Div != nil && !dumped {
			dumped = true
			t.Logf("divergence at %d: %s %s %v %s", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
			dumpAtDiv(t, lastRef, k)
		}
		_ = dumped
	}
}

func dumpAtDiv(t *testing.T, ref *GorgeSide, ker Side) {
	t.Helper()
	t.Logf("== gorge board ==")
	for _, ob := range ref.E.G.Objs {
		if ob.Face() == nil || ob.Zone != state.ZBattlefield {
			continue
		}
		t.Logf("gorge obj %d %s ctrl=%d tapped=%v", int(ob.ID), ob.Face().Name, int(ob.Controller), ob.Tapped)
	}
	for pi, pl := range ref.E.G.Players {
		t.Logf("gorge p%d pool=%v life=%d", pi, pl.Pool, pl.Life)
	}
	real, _ := ker.(*realSide)
	var p dirac.Projection
	if real != nil {
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
			t.Logf("ker obj %d %s flags=%d ctrl=%d", int(i), name, po.Flags, po.Ctrl)
		}
		for pi := 0; pi < 2; pi++ {
			t.Logf("ker p%d pool=%v life=%d", pi, p.Pool[pi], p.Life[pi])
		}
	}
	if d := ref.E.Pending(); d != nil {
		t.Logf("gorge pending kind=%v player=%d prompt=%q resume=%q", d.Kind, int(d.Player), d.Prompt, d.ResumeKind)
		for _, o := range d.Options {
			t.Logf("  opt kind=%q idx=%d amount=%d obj=%d", o.Kind, o.Index, o.Amount, int(o.Obj))
		}
	}
}
