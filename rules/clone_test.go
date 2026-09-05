package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// drive answers n decisions with the package's own testBot and returns the
// intents it submitted, so the same choices can be replayed elsewhere.
func drive(t *testing.T, e *Engine, b *testBot, n int) []decision.Intent {
	t.Helper()
	var out []decision.Intent
	for i := 0; i < n && !e.G.Over && e.Pending() != nil; i++ {
		d := e.Pending()
		in := b.answer(e.G.Step.IsMain(), d)
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d: %v", i, err)
		}
		out = append(out, in)
	}
	return out
}

// seedInternalQueues populates every Engine-internal collection Clone must
// copy -- continuous effects, the pending-trigger queue, and both
// trigger-bookkeeping maps -- so a Clone that aliases, rebinds, or simply
// drops one of them shows up as a real assertion failure instead of two
// tests comparing empty collections to each other (a fixture driven only a
// few dozen decisions has none of these populated on its own).
//
// The pending trigger's Source is a battlefield permanent with no T: lines
// of its own (Idx 0 is therefore always out of range for it), so draining it
// through the real trigger pipeline -- which TestCloneStaysIndependentAndReplaysInLockstep's
// further Submit calls do reach, via putTriggersOnStack -- is a guaranteed,
// side-effect-free no-op (events.Apply's TriggerPush case degrades an
// out-of-range index to nothing) rather than resolving an arbitrary card's
// script against fabricated targets.
func seedInternalQueues(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	var src state.ObjID
	for p := state.PlayerID(0); src == 0 && int(p) < len(e.G.Players); p++ {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if o := e.G.Obj(id); o != nil {
				if f := o.Face(); f != nil && len(f.Triggers) == 0 {
					src = id
					break
				}
			}
		}
	}
	if src == 0 {
		t.Fatal("fixture has no trigger-free battlefield permanent to seed internal state with")
	}

	e.AddContinuous(ContinuousEffect{Source: src, Timestamp: 1, Layer: LAbilities,
		Affects: "Card.Self", AddKeywords: []string{"Flying"}, AddTypes: []string{"Bird"}, UntilEOT: true})
	e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
		Source: src, Idx: 0,
		Ctx: effects.Ctx{
			Targets:    []state.Target{{Obj: src}},
			Remembered: []state.Target{{Obj: src}},
			SVars:      map[string]string{"k": "v"},
		},
	})
	e.triggerFireCount = map[triggerKey]int32{{Source: src, Idx: 0}: 1}
	e.damageOnceFired = map[triggerKey]int32{{Source: 1, Idx: 0}: e.G.Turn}

	// A pending cast (Task 9), with its own non-empty slices -- delve/sacs
	// and the cost's own Sac/SubCounter parts -- so a Clone that omits,
	// aliases, or only shallow-copies e.cast shows up as a real assertion
	// failure the same way an omitted continuous/pendingTriggers would.
	e.cast = &pendingCast{
		player:  0,
		card:    src,
		from:    state.ZHand,
		mode:    "kicked",
		ability: -1,
		cost:    Cost{Generic: 1, Sac: []CostPart{{N: 1, Spec: "Creature"}}},
		x:       1,
		delve:   []state.ObjID{src},
		sacs:    []state.ObjID{src},
	}

	if len(e.continuous) == 0 || len(e.pendingTriggers) == 0 ||
		len(e.triggerFireCount) == 0 || len(e.damageOnceFired) == 0 || e.cast == nil {
		t.Fatal("fixture seeding left an internal collection empty")
	}
	return src
}

func TestCloneStaysIndependentAndReplaysInLockstep(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 4)
	e := New(Config{Seed: 7, Names: names, Decks: decks})
	e.Advance()
	bot := newTestBot(7)
	drive(t, e, bot, 40)
	seedInternalQueues(t, e)

	c := e.Clone()
	// Checked immediately, before either side is driven further: a Clone
	// that omits continuous/pendingTriggers/the trigger maps entirely would
	// leave c's copies at zero while e's (seeded above) are not, catching
	// the omission here rather than after both sides have independently
	// drained the same seeded queue back down to empty.
	if len(c.continuous) != len(e.continuous) || len(c.pendingTriggers) != len(e.pendingTriggers) ||
		len(c.triggerFireCount) != len(e.triggerFireCount) || len(c.damageOnceFired) != len(e.damageOnceFired) ||
		c.cast == nil {
		t.Fatalf("clone did not copy the seeded internal state: continuous %d/%d, triggers %d/%d, fireCount %d/%d, onceFired %d/%d, cast nil=%v",
			len(c.continuous), len(e.continuous), len(c.pendingTriggers), len(e.pendingTriggers),
			len(c.triggerFireCount), len(e.triggerFireCount), len(c.damageOnceFired), len(e.damageOnceFired), c.cast == nil)
	}
	headBefore, drawsBefore, eventsBefore := e.L.Head(), e.RNGDraws(), len(e.L.Events)
	if got := diffGames(e.G, c.G); got != "" {
		t.Fatalf("clone differs from original at the boundary: %s", got)
	}

	// Diverge the clone by 60 more decisions; the original must not move.
	recorded := drive(t, c, bot, 60)
	if len(recorded) == 0 {
		t.Fatal("clone accepted no intents")
	}
	if e.L.Head() != headBefore || e.RNGDraws() != drawsBefore || len(e.L.Events) != eventsBefore {
		t.Fatal("driving the clone changed the original")
	}

	// Feed the original the very same intents: identical events, chain head
	// and RNG position mean the clone copied every piece of engine state.
	for i, in := range recorded {
		if err := e.Submit(in); err != nil {
			t.Fatalf("original rejected recorded intent %d: %v", i, err)
		}
	}
	if e.L.Head() != c.L.Head() {
		t.Fatalf("chain heads differ after lockstep: %s vs %s", e.L.Head(), c.L.Head())
	}
	if e.RNGDraws() != c.RNGDraws() {
		t.Fatalf("RNG draws differ: %d vs %d", e.RNGDraws(), c.RNGDraws())
	}
	// The RNGDraws equality above is vacuous by itself: this fixture's decks
	// never draw from the engine's RNG past the opening shuffle (both sides
	// hold steady at the same count for the whole test), so a clone.rng that
	// re-seeded from scratch instead of copying the PCG's position would
	// still pass it. Drawing one more value from each side's RNG and
	// requiring them to agree only holds if Clone copied the exact position.
	if got, want := c.Rand(100), e.Rand(100); got != want {
		t.Fatalf("RNG positions differ after lockstep: clone drew %d, original drew %d", got, want)
	}
	if got := diffGames(e.G, c.G); got != "" {
		t.Fatalf("games differ after lockstep: %s", got)
	}
	if len(e.pendingTriggers) != len(c.pendingTriggers) || len(e.continuous) != len(c.continuous) {
		t.Fatalf("engine-internal queues differ: triggers %d/%d, continuous %d/%d",
			len(e.pendingTriggers), len(c.pendingTriggers), len(e.continuous), len(c.continuous))
	}
}

func TestCloneSharesNoMutableStateWithTheOriginal(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	e := New(Config{Seed: 3, Names: names, Decks: decks})
	e.Advance()
	drive(t, e, newTestBot(3), 30)
	src := seedInternalQueues(t, e)
	// Nonzero, non-default values for the two bookkeeping scalars: neither
	// test drives the engine again after this point, so leaving them set
	// (rather than the transient save/restore every production caller uses)
	// has no further effect and gives Clone something real to copy.
	e.orderedTriggers = 1
	e.applyingReplacement = true
	c := e.Clone()

	if c.orderedTriggers != e.orderedTriggers || c.applyingReplacement != e.applyingReplacement {
		t.Fatalf("clone did not copy the engine scalars: orderedTriggers %d/%d, applyingReplacement %v/%v",
			c.orderedTriggers, e.orderedTriggers, c.applyingReplacement, e.applyingReplacement)
	}

	// Mutate every cloned collection IN PLACE (index/key writes, never
	// append past a slice's length): an append that exceeds capacity
	// allocates a fresh backing array regardless of whether the clone
	// aliased the original, silently passing even a Clone that aliased
	// everything. An in-place write only leaves the original untouched if
	// Clone actually copied the backing storage.
	c.G.Players[0].Life = -100
	c.L.Events[0].Kind = 200
	if c.pending != nil && len(c.pending.Options) > 0 {
		c.pending.Options[0].Label = "mutated"
	}
	c.continuous[0].AddKeywords[0] = "Mutated"
	c.pendingTriggers[0].Ctx.Targets[0].Obj = 9999
	c.pendingTriggers[0].Ctx.SVars["k"] = "mutated"
	c.triggerFireCount[triggerKey{Source: src, Idx: 0}] = 99
	c.damageOnceFired[triggerKey{Source: 1, Idx: 0}] = 99
	c.cast.delve[0] = 9999
	c.cast.sacs[0] = 9999
	c.cast.cost.Sac[0].N = 99

	if e.G.Players[0].Life == -100 || e.L.Events[0].Kind == 200 {
		t.Fatal("clone shares Game or Log storage with the original")
	}
	if e.pending != nil && len(e.pending.Options) > 0 && e.pending.Options[0].Label == "mutated" {
		t.Fatal("clone shares the pending decision's Options")
	}
	for _, ce := range e.continuous {
		for _, k := range ce.AddKeywords {
			if k == "Mutated" {
				t.Fatal("clone shares a continuous effect's AddKeywords")
			}
		}
	}
	for _, pt := range e.pendingTriggers {
		for _, tgt := range pt.Ctx.Targets {
			if tgt.Obj == 9999 {
				t.Fatal("clone shares a pending trigger's Targets")
			}
		}
		if pt.Ctx.SVars["k"] == "mutated" {
			t.Fatal("clone shares a pending trigger's SVars map")
		}
	}
	if e.triggerFireCount[triggerKey{Source: src, Idx: 0}] == 99 {
		t.Fatal("clone shares triggerFireCount")
	}
	if e.damageOnceFired[triggerKey{Source: 1, Idx: 0}] == 99 {
		t.Fatal("clone shares damageOnceFired")
	}
	if e.cast.delve[0] == 9999 {
		t.Fatal("clone shares a pending cast's delve slice")
	}
	if e.cast.sacs[0] == 9999 {
		t.Fatal("clone shares a pending cast's sacs slice")
	}
	if e.cast.cost.Sac[0].N == 99 {
		t.Fatal("clone shares a pending cast's cost.Sac slice")
	}
}

func TestCloneOfAFinishedGameIsFinished(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	e := New(Config{Seed: 11, Names: names, Decks: decks})
	e.Advance()
	drive(t, e, newTestBot(11), 400000)
	if !e.G.Over {
		t.Fatal("fixture game did not finish")
	}
	c := e.Clone()
	if !c.G.Over || c.L.Head() != e.L.Head() || c.Pending() != nil {
		t.Fatal("clone of a finished game is not finished with the same head")
	}
	if err := c.Submit(decision.Intent{}); err == nil {
		t.Fatal("clone of a finished game accepted an intent")
	}
}
