package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestScratchStatureOffer probes gorge's offer gate for Stature, Size
// Shifter's power-up in the count scenario: pool {U}, no untapped mana
// source. Scratch debug only; not for commit.
func TestScratchStatureOffer(t *testing.T) {
	stature := corpusAlternativeCard(t, "Stature, Size Shifter")
	for _, tc := range []struct {
		name   string
		lands  int
		tapped bool
		poolU  int
		poolUU int
	}{
		{"pool U, no lands", 0, false, 1, 0},
		{"pool U, tapped land", 1, true, 1, 0},
		{"pool U, untapped land", 1, false, 1, 0},
		{"pool UU, tapped land", 1, true, 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := handEngine(t)
			o := e.G.AddObject(stature, 0)
			o.Zone = state.ZBattlefield
			o.EnteredThisTurn = true
			e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})
			for i := 0; i < tc.lands; i++ {
				m := card(t, "Name:Island\nTypes:Basic Land Island\nOracle:x\n")
				ob := e.G.AddObject(m, 0)
				ob.Zone = state.ZBattlefield
				ob.Tapped = tc.tapped
				e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), ob.ID))
			}
			pool := &e.G.Players[0].Pool
			pool[state.MU] += int32(tc.poolU + tc.poolUU)
			e.priorityRound()
			d := e.Pending()
			if d == nil {
				t.Fatal("no pending decision")
			}
			got := ""
			for _, opt := range d.Options {
				name := ""
				if ob := e.G.Obj(opt.Obj); ob != nil && ob.Face() != nil {
					name = ob.Face().Name
				}
				got += fmt.Sprintf("[%s obj=%d %s] ", opt.Kind, int(opt.Obj), name)
			}
			t.Logf("pool=%v options=%d: %s", pool, len(d.Options), got)
		})
	}
}
