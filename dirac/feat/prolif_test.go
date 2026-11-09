package feat

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestProliferatePlayerFeatMatchesPolicynet: gorge poses a proliferate
// resolution's player pick as KChoose "proliferate" with Obj 0 and Player set
// (engine/effects/counters.go), and policynet's isPlayer only reads a
// player-ref Obj or Kind "player", so its option is not a player option. The
// kernel's player-target action must encode the same way.
func TestProliferatePlayerFeatMatchesPolicynet(t *testing.T) {
	d := dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChGenProlif, N: 2}
	d.Options[0] = dirac.MakeAction(dirac.AkChoose, dirac.PlayerTarget0+1, 0, 0)
	d.Options[1] = dirac.MakeAction(dirac.AkDone, 0, 0, 0)

	// Preconditions: the action is the player recipient (not a permanent)
	// and it reaches the proliferate arm.
	if src := d.Options[0].Source(); src < dirac.PlayerTarget0 {
		t.Fatalf("setup: option source %d is not a player target (>= %d)", src, dirac.PlayerTarget0)
	}
	e := &Encoder{}
	o := e.optionOf(&d, d.Options[0])
	if o.kind != okProliferate {
		t.Fatalf("setup: option kind %d, want okProliferate %d", o.kind, okProliferate)
	}
	var k Option
	e.encodeOption(&k, o, 0, 2)

	var v view.View
	g := policynet.EncodeOptionsWith(policynet.FeaturesEntity, &v, 0, decision.KChoose,
		[]decision.Option{{Kind: "proliferate", Player: 1}, {Kind: "done"}}, nil)
	if len(g) != 2 {
		t.Fatalf("reference encoded %d options, want 2", len(g))
	}

	if k.Dense[7] != 0 || g[0].Dense[7] != 0 {
		t.Fatalf("dense[7] (isPlayer) kernel=%v policynet=%v, want 0 and 0", k.Dense[7], g[0].Dense[7])
	}
	if got, want := k.Hashed[:k.NHashed], g[0].Hashed; len(got) != len(want) {
		t.Fatalf("hashed rows kernel=%v policynet=%v, want the same count", got, want)
	} else {
		for i := range got {
			if got[i].Row != want[i].Row || got[i].Value != want[i].Value {
				t.Fatalf("hashed row %d kernel=%v policynet=%v, want identical order", i, got[i], want[i])
			}
		}
	}

	ks := append([]Feature(nil), k.Slots[:k.NSlots]...)
	sort.Slice(ks, func(i, j int) bool { return ks[i].Row < ks[j].Row })
	gs := append([]policynet.Feature(nil), g[0].Slots...)
	sort.Slice(gs, func(i, j int) bool { return gs[i].Row < gs[j].Row })
	if len(ks) != len(gs) {
		t.Fatalf("slots kernel=%v policynet=%v, want the same count", ks, gs)
	}
	for i := range ks {
		if ks[i].Row != gs[i].Row || ks[i].Value != gs[i].Value {
			t.Fatalf("sorted slot %d kernel=%v policynet=%v, want identical", i, ks[i], gs[i])
		}
	}

	for i := 0; i < OptionDenseWidth; i++ {
		if k.Dense[i] != g[0].Dense[i] {
			t.Fatalf("dense[%d] kernel=%v policynet=%v, want identical over the %d compared columns",
				i, k.Dense[i], g[0].Dense[i], OptionDenseWidth)
		}
	}
}
