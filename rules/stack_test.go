package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func castFirst(t *testing.T, e *Engine, kind string) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending decision")
	}
	for _, o := range d.Options {
		if o.Kind == kind {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
				t.Fatalf("submit: %v", err)
			}
			return
		}
	}
	t.Fatalf("no %q option in %+v", kind, d.Options)
}

func TestCastPutsSpellOnStackAndAsksForTargets(t *testing.T) {
	bolt := card(t, "Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	e := handEngine(t, bolt)
	e.G.Players[0].Pool[state.MR] = 1
	e.askPriority(0)

	castFirst(t, e, "cast")

	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the spell", e.G.Stack)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision, got %+v", d)
	}
	if len(d.Options) < 2 {
		t.Fatalf("both players should be legal targets: %+v", d.Options)
	}
	if e.G.Players[0].Pool[state.MR] != 0 {
		t.Error("casting did not spend mana")
	}
}

func TestSpellResolvesAndAppliesItsEffect(t *testing.T) {
	bolt := card(t, "Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	e := handEngine(t, bolt)
	e.G.Players[0].Pool[state.MR] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")

	// Target the opponent.
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("opponent not offered as a target: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatal(err)
	}
	// Everyone passes; the spell resolves.
	for i := 0; i < 4 && len(e.G.Stack) > 0; i++ {
		castFirst(t, e, "pass")
	}
	if e.G.Players[1].Life != 17 {
		t.Fatalf("opponent life = %d, want 17", e.G.Players[1].Life)
	}
	if len(e.G.Stack) != 0 {
		t.Fatal("stack did not empty")
	}
	if len(e.G.Zone(state.ZGraveyard, 0)) != 1 {
		t.Fatal("the instant should be in its owner's graveyard")
	}
}

func TestPermanentResolvesToTheBattlefield(t *testing.T) {
	bear := card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e := handEngine(t, bear)
	e.G.Players[0].Pool[state.MG] = 3
	e.askPriority(0)
	castFirst(t, e, "cast")
	for i := 0; i < 4 && len(e.G.Stack) > 0; i++ {
		castFirst(t, e, "pass")
	}
	bf := e.G.Zone(state.ZBattlefield, 0)
	if len(bf) != 1 {
		t.Fatalf("battlefield = %v", bf)
	}
	if !e.G.Obj(bf[0]).SummonSick {
		t.Error("a creature entering should be summoning sick")
	}
}

func TestLastInFirstOutResolution(t *testing.T) {
	bolt := card(t, "Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	shock := card(t, "Name:Shock\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2\nOracle:x\n")
	e := handEngine(t, bolt, shock)
	e.G.Players[0].Pool[state.MR] = 2
	e.askPriority(0)

	target := func() {
		d := e.Pending()
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player == 1 {
				e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}})
				return
			}
		}
	}
	castFirst(t, e, "cast")
	target()
	castFirst(t, e, "cast")
	target()
	if len(e.G.Stack) != 2 {
		t.Fatalf("stack = %v, want two spells", e.G.Stack)
	}
	top := e.G.Obj(e.G.Stack[1]).Face().Name
	for i := 0; i < 8 && len(e.G.Stack) > 0; i++ {
		castFirst(t, e, "pass")
	}
	gy := e.G.Zone(state.ZGraveyard, 0)
	if len(gy) != 2 {
		t.Fatalf("graveyard = %v", gy)
	}
	if e.G.Obj(gy[0]).Face().Name != top {
		t.Fatalf("resolution order wrong: %s resolved first, stack top was %s",
			e.G.Obj(gy[0]).Face().Name, top)
	}
	if e.G.Players[1].Life != 15 {
		t.Fatalf("life = %d, want 15 after Bolt and Shock", e.G.Players[1].Life)
	}
}

func TestFizzleWhenNoLegalTargets(t *testing.T) {
	// A creature-only removal spell cast into an empty board has no legal
	// target, so it never reaches the stack's resolution step.
	kill := card(t, "Name:Kill\nManaCost:B\nTypes:Instant\nA:SP$ Destroy | ValidTgts$ Creature\nOracle:x\n")
	e := handEngine(t, kill)
	e.G.Players[0].Pool[state.MB] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want empty after a fizzle", e.G.Stack)
	}
	if len(e.G.Zone(state.ZGraveyard, 0)) != 1 {
		t.Fatal("the fizzled spell should be in the graveyard")
	}
}

// --- Task 13 gap-closing coverage -----------------------------------------
//
// legal_test.go and fix1_test.go's Submit-driven tests (TestPlayLandReplay-
// ThroughSubmit, TestActivateManaAbilityReplayThroughSubmit) could not cover
// the "cast" branch or activate's mana-pool effect, because castSpell and
// resolveAbility were still this task's empty stubs when they were written.
// The two tests below close that gap, following the same shape as their
// fix1_test.go siblings: drive the action through Submit only, then replay
// the resulting log alone (no Engine, no rules code involved) and confirm it
// reproduces the live state.
//
// The replay comparison is deliberately scoped to player-indexed scalar
// state (Pool, Life, Passes, Priority) rather than anything object-shaped
// (Stack contents, Object.Targets): genesis -- state.NewGame plus the
// AddObject calls that place cards, whether via Engine.New's deck loop or a
// test's own direct placement -- is never logged (see the rules package
// doc), so a fresh game built from cfg.Names has an empty object arena no
// matter how the live game got its cards. That is the same, already-
// established boundary TestPlayLandReplayThroughSubmit and
// TestActivateManaAbilityReplayThroughSubmit observe for LandsPlayed/Tapped;
// Task 24 (replay) is what recombines a Config with a log to reconstruct
// object-scoped state.

// TestCastReplayThroughSubmit drives a full cast -> target -> resolve cycle
// through Submit and checks both the live mutations castSpell/resolveAbility
// make (mana spent, the spell on the stack, its target recorded, the target
// player's life reduced) and that the scalar half of that state replays.
func TestCastReplayThroughSubmit(t *testing.T) {
	names := []string{"a", "b"}
	cfg := Config{Seed: 11, Names: names,
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	bolt := card(t, "Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	o := e.G.AddObject(bolt, 0)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	// Setup only (not the action under test): fund the cast. Emitted, not a
	// direct Pool write, so the replay comparison below -- which reconstructs
	// the pool from the log alone -- has a starting balance to reconstruct;
	// a direct write here would be invisible to replay and desync fresh's
	// pool from the live one the moment castSpell spends against it.
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 1})

	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected player 0's priority, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == o.ID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the hand spell: %+v", d.Options)
	}

	// The action under test: a cast intent through Submit.
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}

	if len(e.G.Stack) != 1 || e.G.Stack[0] != o.ID {
		t.Fatalf("stack = %v, want [%d]", e.G.Stack, o.ID)
	}
	if e.G.Players[0].Pool[state.MR] != 0 {
		t.Fatalf("pool = %v, want the R spent", e.G.Players[0].Pool)
	}

	target := e.Pending()
	if target == nil || target.Kind != decision.KTarget {
		t.Fatalf("expected a target decision, got %+v", target)
	}
	tIdx := -1
	for _, opt := range target.Options {
		if opt.Kind == "player" && opt.Player == 1 {
			tIdx = opt.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("opponent not offered as a target: %+v", target.Options)
	}
	// Also through Submit: exercises handleTarget's TargetsChosen emit.
	if err := e.Submit(decision.Intent{Seq: target.Seq, Player: 0, Choices: []int{tIdx}}); err != nil {
		t.Fatalf("submit target: %v", err)
	}
	if got := e.G.Obj(o.ID).Targets; len(got) != 1 || !got[0].IsPlayer || got[0].Player != 1 {
		t.Fatalf("targets = %+v, want a single player-1 target", got)
	}

	// Both players pass; the spell resolves.
	for i := 0; i < 4 && len(e.G.Stack) > 0; i++ {
		if passAll(t, e, 1) == 0 {
			t.Fatal("expected priority decisions until the spell resolves")
		}
	}
	if e.G.Players[1].Life != 17 {
		t.Fatalf("life = %d, want 17 after Bolt resolves", e.G.Players[1].Life)
	}

	fresh := state.NewGame(cfg.Names)
	for _, ev := range e.L.Events {
		events.Apply(fresh, ev)
	}
	if fresh.Players[0].Pool != e.G.Players[0].Pool {
		t.Errorf("replayed pool = %v, want %v (live)", fresh.Players[0].Pool, e.G.Players[0].Pool)
	}
	if fresh.Players[1].Life != e.G.Players[1].Life {
		t.Errorf("replayed life = %d, want %d (live)", fresh.Players[1].Life, e.G.Players[1].Life)
	}
	if fresh.Passes != e.G.Passes {
		t.Errorf("replayed Passes = %d, want %d (live)", fresh.Passes, e.G.Passes)
	}
	if fresh.Priority != e.G.Priority {
		t.Errorf("replayed Priority = %d, want %d (live)", fresh.Priority, e.G.Priority)
	}
}

// TestActivateManaAbilityProducesMana is
// TestActivateManaAbilityReplayThroughSubmit's sibling: same setup, but
// carried through to the assertion that fix1_test.go's version explicitly
// deferred -- that activating actually adds mana to the pool now that
// resolveAbility is no longer a stub -- plus the replay comparison for that
// pool.
func TestActivateManaAbilityProducesMana(t *testing.T) {
	names := []string{"a", "b"}
	cfg := Config{Seed: 12, Names: names,
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	mtn := card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	o := e.G.AddObject(mtn, 0)
	o.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1

	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected player 0's priority, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "activate" && opt.Obj == o.ID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no activate option for the battlefield land: %+v", d.Options)
	}

	// The action under test: an activate intent through Submit.
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit activate: %v", err)
	}

	if !o.Tapped {
		t.Fatal("activating the mana ability should tap the land")
	}
	if e.G.Players[0].Pool[state.MR] != 1 {
		t.Fatalf("pool = %v, want 1 red mana from the Mountain", e.G.Players[0].Pool)
	}

	fresh := state.NewGame(cfg.Names)
	for _, ev := range e.L.Events {
		events.Apply(fresh, ev)
	}
	if fresh.Players[0].Pool != e.G.Players[0].Pool {
		t.Errorf("replayed pool = %v, want %v (live)", fresh.Players[0].Pool, e.G.Players[0].Pool)
	}
}
