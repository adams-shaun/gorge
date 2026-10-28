package gamecfg_test

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
)

// TestShufflePlannerOverride supplies a planner that reverses every library,
// then checks the opening and the London-mulligan redraw both use it instead
// of the secret's library_shuffle stream.
func TestShufflePlannerOverride(t *testing.T) {
	reg := testcorpus.Registry(t)
	burn := deck(t, reg, "Burn")
	reverse := func(ctx rules.ShuffleContext) ([]state.ObjID, error) {
		out := make([]state.ObjID, len(ctx.Library))
		for i, c := range ctx.Library {
			out[len(out)-1-i] = c.ID
		}
		return out, nil
	}
	var calls int
	g, err := gamecfg.New(reg, secret(11), [2][]*cards.Card{burn, burn}, gamecfg.Rules{
		Mulligan: "london",
		ShufflePlanner: func(ctx rules.ShuffleContext) ([]state.ObjID, error) {
			calls++
			return reverse(ctx)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("planner calls after genesis = %d, want one per seat", calls)
	}
	// The genesis shuffle is the whole deck in object order; the deal keeps
	// the shuffled order as hand followed by library.
	next := state.ObjID(1)
	for p := state.PlayerID(0); p < 2; p++ {
		in := make([]state.ObjID, len(burn))
		for i := range in {
			in[i] = next
			next++
		}
		want := slices.Clone(in)
		slices.Reverse(want)
		if slices.Equal(in, want) {
			t.Fatalf("precondition: seat %d deck order is its own reverse", p)
		}
		if got := dealt(g, p); !slices.Equal(got, want) {
			t.Fatalf("seat %d opening order\n got %v\nwant %v", p, got, want)
		}
	}
	// A London mulligan is an in-game shuffle effect: p0's library plus hand
	// is shuffled again, so its reversed opening order must come back.
	in := append(slices.Clone(g.E.G.Zone(state.ZLibrary, 0)), g.E.G.Zone(state.ZHand, 0)...)
	want := slices.Clone(in)
	slices.Reverse(want)
	if slices.Equal(in, want) {
		t.Fatal("precondition: p0's library-plus-hand is its own reverse")
	}
	if err := g.Submit(intent(t, g, 0, "mulligan")); err != nil {
		t.Fatal(err)
	}
	if err := g.Submit(intent(t, g, 1, "keep")); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("planner calls after the mulligan = %d, want genesis plus the redraw", calls)
	}
	if got := dealt(g, 0); !slices.Equal(got, want) {
		t.Fatalf("p0 order after the mulligan\n got %v\nwant %v", got, want)
	}
	if err := g.CheckRandomness(); err != nil {
		t.Fatalf("planner-supplied shuffle counted as unplanned: %v", err)
	}
}

// TestShufflePlannerNilUnchanged pins the default: with no planner, the
// opening libraries still come from each seat's secret library_shuffle stream,
// so the env goldens keep replaying byte for byte.
func TestShufflePlannerNilUnchanged(t *testing.T) {
	reg := testcorpus.Registry(t)
	burn := deck(t, reg, "Burn")
	sec := secret(5)
	g, err := gamecfg.New(reg, sec, [2][]*cards.Card{burn, burn}, gamecfg.Rules{Mulligan: "none"})
	if err != nil {
		t.Fatal(err)
	}
	next := state.ObjID(1)
	for p, owner := range []string{"p0", "p1"} {
		in := make([]state.ObjID, len(burn))
		for i := range in {
			in[i] = next
			next++
		}
		if got, want := dealt(g, state.PlayerID(p)), fisherYates(sec, owner, 0, in); !slices.Equal(got, want) {
			t.Fatalf("%s opening order\n got %v\nwant %v", owner, got, want)
		}
	}
}
