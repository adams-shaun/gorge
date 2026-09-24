package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// The RepeatCheckSVar$/RepeatSVarCompare$ between-iteration gate
// (task agent-20260919T192133Z-20ce31d6): a gate-governed Repeat runs its
// body at least once and repeats only while the gate holds, re-evaluated
// per iteration. The synthetic bodies use test-local registered effects --
// the same pattern TestResolveWalksTheSubAbilityChain uses -- because the
// corpus's own gate drivers (StoreSVar accumulators) are not yet
// implemented and would make every gate here unreadable.

func TestRepeatGateRepeatsOnlyWhileTheCheckHolds(t *testing.T) {
	runs := 0
	Register("TestGateTick", func(h Host, c *Ctx, s *cards.SA) {
		runs++
		// Each iteration grows the remembered set, so the gate's count body
		// (Remembered$Amount) moves between iterations: 1, 2, 3.
		c.Remembered = append(c.Remembered, state.Target{Obj: 1})
	})
	t.Cleanup(func() { unregister("TestGateTick") })

	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		"Gate": "Remembered$Amount",
		"Loop": "DB$ TestGateTick",
	}
	// Do-while: run (Remembered 1) < 3 holds -> run (2) holds -> run (3)
	// fails -> stop. Exactly 3 iterations.
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat",
		Params: map[string]string{"RepeatSubAbility": "Loop",
			"RepeatCheckSVar": "Gate", "RepeatSVarCompare": "LT3"}})
	if runs != 3 {
		t.Fatalf("gate-governed repeat ran %d times, want 3", runs)
	}
}

func TestRepeatGateStopsImmediatelyWhenTheCheckFails(t *testing.T) {
	runs := 0
	Register("TestGateTick", func(h Host, c *Ctx, s *cards.SA) { runs++ })
	t.Cleanup(func() { unregister("TestGateTick") })

	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		"Gate": "Remembered$Amount", // the body never remembers: stays 0
		"Loop": "DB$ TestGateTick",
	}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat",
		Params: map[string]string{"RepeatSubAbility": "Loop",
			"RepeatCheckSVar": "Gate", "RepeatSVarCompare": "GE1"}})
	if runs != 1 {
		t.Fatalf("repeat ran %d times, want exactly 1 (gate false after the first body)", runs)
	}
}

func TestRepeatGateUnevaluatedStopsAfterOneIteration(t *testing.T) {
	runs := 0
	Register("TestGateTick", func(h Host, c *Ctx, s *cards.SA) { runs++ })
	t.Cleanup(func() { unregister("TestGateTick") })

	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		// A body whose filter predicate this build does not know: the count
		// would fail closed to 0, which under an EQ0 compare would hold
		// forever and loop to the cap -- so it must be UNevaluated and stop
		// the loop after the iteration just run (the pre-gate behaviour).
		"Gate": "Count$ValidHand Card.absolutelyUnknownPredicateHere",
		"Loop": "DB$ TestGateTick",
	}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat",
		Params: map[string]string{"RepeatSubAbility": "Loop",
			"RepeatCheckSVar": "Gate", "RepeatSVarCompare": "EQ0"}})
	if runs != 1 {
		t.Fatalf("repeat ran %d times, want 1 (unevaluated gate stops the loop)", runs)
	}
}

func TestRepeatGateMaxRepeatCapsAGateThatAlwaysHolds(t *testing.T) {
	runs := 0
	Register("TestGateTick", func(h Host, c *Ctx, s *cards.SA) { runs++ })
	t.Cleanup(func() { unregister("TestGateTick") })

	h, c := fixtureHost(t)
	c.SVars = map[string]string{"Loop": "DB$ TestGateTick"}
	// MaxRepeat$ 3 caps a gate that always holds (GE0 is always true).
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat",
		Params: map[string]string{"RepeatSubAbility": "Loop",
			"RepeatCheckSVar": "Remembered$Amount", "RepeatSVarCompare": "GE0", "MaxRepeat": "3"}})
	if runs != 3 {
		t.Fatalf("MaxRepeat did not cap the gate-governed loop: ran %d times, want 3", runs)
	}
	// And the unbounded default is clamped to the 1000-iteration cap.
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat",
		Params: map[string]string{"RepeatSubAbility": "Loop",
			"RepeatCheckSVar": "Remembered$Amount", "RepeatSVarCompare": "GE0"}})
	if runs-3 != 1000 {
		t.Fatalf("unbounded gate-governed repeat ran %d times, want the 1000 cap", runs-3)
	}
}

func TestRepeatSuspensionDropsTheRemainingIterations(t *testing.T) {
	runs := 0
	Register("TestGateTick", func(h Host, c *Ctx, s *cards.SA) {
		runs++
		// A body ask suspends the resolution: the fake host's Ask always
		// returns false, and the effect marks the double suspended the way
		// the real engine's pending resume point does.
		h.(*fakeHost).suspendAfterAsk = true
	})
	t.Cleanup(func() { unregister("TestGateTick") })

	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		"Gate": "Remembered$Amount", // always holds; MaxRepeat bounds the loop
		"Loop": "DB$ TestGateTick",
	}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat",
		Params: map[string]string{"RepeatSubAbility": "Loop",
			"RepeatCheckSVar": "Gate", "MaxRepeat": "3"}})
	if runs != 1 {
		t.Fatalf("suspended body: loop ran %d times, want 1 (remaining iterations dropped, no dispatch into a suspended engine)", runs)
	}
}

// A repeat-while gate whose SVar the body rewrites through api:StoreSVar
// reads the STORED value, not the printed body (Sword of Dungeons & Dragons:
// RepeatCheck is printed Number$ 1 and the d20's miss branch stores 0). The
// printed read looped every trigger to the 1000-iteration cap -- a thousand
// Dragon tokens per hit, the corpus fuzzer's damage livelock and hang.
func TestRepeatGateReadsTheStoreSVarWrite(t *testing.T) {
	runs := 0
	Register("TestGateTick", func(h Host, c *Ctx, s *cards.SA) {
		runs++
		// Third pass misses: store 0 (earlier passes store 1, the "20").
		v := "1"
		if runs >= 3 {
			v = "0"
		}
		Resolve(h, c, &cards.SA{Kind: "DB", API: "StoreSVar",
			Params: map[string]string{"SVar": "RepeatCheck", "Type": "Number", "Expression": v}})
	})
	t.Cleanup(func() { unregister("TestGateTick") })

	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		"RepeatCheck": "Number$ 1",
		"Loop":        "DB$ TestGateTick",
	}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat",
		Params: map[string]string{"RepeatSubAbility": "Loop",
			"RepeatCheckSVar": "RepeatCheck", "RepeatSVarCompare": "GT0"}})
	if runs != 3 {
		t.Fatalf("repeat ran %d times, want 3 (stops on the stored 0)", runs)
	}
}
