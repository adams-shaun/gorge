package oracle

import (
	"math/rand/v2"
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestScratchR2Copyab runs copyabAmtSetup's seed (SCRATCH_SEED, default 1) and
// dumps both boards at the first divergence. Scratch debug only; not for
// commit.
func TestScratchR2Copyab(t *testing.T) {
	seed := uint64(1)
	if v := os.Getenv("SCRATCH_SEED"); v != "" {
		n, _ := strconv.ParseUint(v, 10, 64)
		seed = n
	}
	reg, set, tbl, decks := copyabAmtSetup(t)
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(reg, [2]string{"copyab-amt-a", "copyab-amt-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	if os.Getenv("DKTRACE") != "" {
		dirac.DKPlanTrace(func(f string, a ...any) {
			t.Logf("DK "+f, a...)
		})
		defer dirac.DKPlanTrace(nil)
	}
	xcode, _ := chooseCode("x")
	probe := &copyabAmtProbe{st: &copyabAmtStats{}, xcode: xcode, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x61746f6d)),
		copies: map[state.ObjID]int{}, retarg: map[state.ObjID]int{}, aimed: map[state.ObjID]bool{}, pickedAim: map[state.ObjID]bool{}}
	var lastRef *GorgeSide
	res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20,
		Check: func(ref *GorgeSide, _ Side) *Divergence {
			lastRef = ref
			return nil
		}})
	div := res.Div
	if div == nil {
		t.Fatalf("seed %d: no divergence", seed)
	}
	t.Logf("seed %d diverged at %d: %s %s %v %s", seed, div.At, div.What, div.Field, div.Diff, div.Detail)
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
	if lastRef == nil {
		return
	}
	t.Logf("== gorge board ==")
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
}
