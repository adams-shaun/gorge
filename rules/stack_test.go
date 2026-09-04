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

// --- CR 608.2b: target legality is rechecked at resolution, not just at
// cast time (TestFizzleWhenNoLegalTargets above). These two tests build the
// stack object directly rather than through castSpell/askTarget, because
// M1's askTarget only ever asks for a single target (Min/Max 1) -- there is
// no way to reach a spell with two chosen targets, one legal and one not,
// through the public casting path the way real Magic's TargetMin/TargetMax
// would allow. Constructing the scenario directly and calling resolveTop is
// the same white-box style layers_test.go already uses for Derived/
// AddContinuous.

// TestSpellFizzlesWhenAllTargetsBecomeIllegalBeforeResolution: the spell's
// only target dies (to anything -- combat, another spell) after it was
// chosen but before this spell resolves. The whole spell must do nothing --
// not just skip the now-gone target, but also never run the SubAbility
// chained onto it, which names no target of its own and so would otherwise
// still fire even though CR 608.2b says the spell never resolves at all.
func TestSpellFizzlesWhenAllTargetsBecomeIllegalBeforeResolution(t *testing.T) {
	e := layerEngine(t)
	victim := onBoard(t, e, 1, "Name:Goat\nManaCost:1 G\nTypes:Creature Goat\nPT:2/2\nOracle:x\n")
	helix := card(t, "Name:Helix\nManaCost:R W\nTypes:Instant\n"+
		"A:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 3 | SubAbility$ DBLife\n"+
		"SVar:DBLife:DB$ GainLife | Defined$ You | LifeAmount$ 3\nOracle:x\n")
	o := e.G.AddObject(helix, 0)
	o.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, []state.ObjID{o.ID})
	o.Targets = []state.Target{{Obj: victim}}

	// The only target dies before the spell resolves.
	e.emit(events.Event{Kind: events.MoveZone, Obj: victim, From: state.ZBattlefield, To: state.ZGraveyard})
	beforeLife := e.G.Players[0].Life

	e.resolveTop()

	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want empty", e.G.Stack)
	}
	if got := e.G.Obj(o.ID).Zone; got != state.ZGraveyard {
		t.Fatalf("fizzled spell zone = %s, want graveyard", got)
	}
	if e.G.Players[0].Life != beforeLife {
		t.Fatalf("caster life = %d, want unchanged %d: the chained SubAbility must not run once every "+
			"target is illegal (CR 608.2b)", e.G.Players[0].Life, beforeLife)
	}
}

// TestResolveDoesAsMuchAsItCanWhenSomeTargetsAreStillLegal: two targets were
// chosen (an object target and a player target); the object target dies
// before resolution but the player target does not. CR 608.2b: the spell
// still resolves, applying its effect to the target that is still legal.
func TestResolveDoesAsMuchAsItCanWhenSomeTargetsAreStillLegal(t *testing.T) {
	e := layerEngine(t)
	dead := onBoard(t, e, 1, "Name:Goat\nManaCost:1 G\nTypes:Creature Goat\nPT:2/2\nOracle:x\n")
	thunder := card(t, "Name:Thunder\nManaCost:2 R\nTypes:Sorcery\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	o := e.G.AddObject(thunder, 0)
	o.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, []state.ObjID{o.ID})
	o.Targets = []state.Target{{Obj: dead}, {Player: 1, IsPlayer: true}}

	// One of the two chosen targets dies before the spell resolves; the
	// player target does not.
	e.emit(events.Event{Kind: events.MoveZone, Obj: dead, From: state.ZBattlefield, To: state.ZGraveyard})

	e.resolveTop()

	if e.G.Players[1].Life != 17 {
		t.Fatalf("opponent life = %d, want 17: the still-legal player target must still take damage", e.G.Players[1].Life)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want empty", e.G.Stack)
	}
	if got := e.G.Obj(o.ID).Zone; got != state.ZGraveyard {
		t.Fatalf("spell zone = %s, want graveyard", got)
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

// --- Fix round 1: Ruling T14-e -- the caster keeps priority ---------------
//
// CR 117.3c: the player who cast a spell (or chose its target) keeps
// priority. All three Priority emits Ruling T14-a moved behind events used
// e.G.Active as the value, which is right only when the caster happens to
// be the active player -- every test above casts as the active player
// (seat 0), so none of them could have caught this. These two tests build a
// non-active caster (seat 1, priority passed to it by seat 0) and drive a
// cast through Submit, once with no target and once with one, asserting
// priority ends with the caster rather than snapping back to seat 0.

// passToPlayerOne has player 0 pass priority once, without resolving
// anything, so player 1 (not the active player) is the one asked next --
// setup shared by the three tests below, not itself the action under test.
func passToPlayerOne(t *testing.T, e *Engine) {
	t.Helper()
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected player 0's priority, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "pass" {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no pass option: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit pass: %v", err)
	}
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 1 {
		t.Fatalf("expected player 1's priority after the pass (active stays 0), got %+v", d)
	}
}

// TestNonActiveCasterKeepsPriorityNoTarget: player 1 casts a no-target
// instant. Before Ruling T14-e, castSpell's trailing Priority emit used
// e.G.Active (0), clobbering the correct value legal.go's "cast" case had
// just emitted.
func TestNonActiveCasterKeepsPriorityNoTarget(t *testing.T) {
	names := []string{"a", "b"}
	cfg := Config{Seed: 21, Names: names,
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	// A vanilla instant with no ability at all: SpellAbility() is nil, so
	// castSpell never asks for a target.
	quiet := card(t, "Name:Quiet Instant\nManaCost:R\nTypes:Instant\nOracle:x\n")
	o := e.G.AddObject(quiet, 1)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{o.ID})
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	e.emit(events.Event{Kind: events.ManaAdd, Player: 1, Counter: "R", Amount: 1})

	passToPlayerOne(t, e)
	d := e.Pending()
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == o.ID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for player 1's instant: %+v", d.Options)
	}

	// The action under test: player 1 (non-active) casts.
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}

	if e.G.Priority != 1 {
		t.Fatalf("Priority = %d, want 1 (the caster) per CR 117.3c", e.G.Priority)
	}
	next := e.Pending()
	if next == nil || next.Kind != decision.KPriority || next.Player != 1 {
		t.Fatalf("next decision = %+v, want another priority decision for player 1 (the caster)", next)
	}
}

// TestNonActiveCasterKeepsPriorityWithTarget is the targeted counterpart:
// player 1 casts Lightning Bolt and chooses its target. Before Ruling
// T14-e, handleTarget's trailing Priority emit used e.G.Active (0) instead
// of the submitting player.
func TestNonActiveCasterKeepsPriorityWithTarget(t *testing.T) {
	names := []string{"a", "b"}
	cfg := Config{Seed: 22, Names: names,
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	bolt := card(t, "Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	o := e.G.AddObject(bolt, 1)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{o.ID})
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	e.emit(events.Event{Kind: events.ManaAdd, Player: 1, Counter: "R", Amount: 1})

	passToPlayerOne(t, e)
	d := e.Pending()
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == o.ID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for player 1's instant: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}

	target := e.Pending()
	if target == nil || target.Kind != decision.KTarget || target.Player != 1 {
		t.Fatalf("expected player 1's target decision, got %+v", target)
	}
	tIdx := -1
	for _, opt := range target.Options {
		if opt.Kind == "player" && opt.Player == 0 {
			tIdx = opt.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("active player not offered as a target: %+v", target.Options)
	}

	// The action under test: player 1 (non-active) chooses a target.
	if err := e.Submit(decision.Intent{Seq: target.Seq, Player: 1, Choices: []int{tIdx}}); err != nil {
		t.Fatalf("submit target: %v", err)
	}

	if e.G.Priority != 1 {
		t.Fatalf("Priority = %d, want 1 (the caster) per CR 117.3c", e.G.Priority)
	}
	next := e.Pending()
	if next == nil || next.Kind != decision.KPriority || next.Player != 1 {
		t.Fatalf("next decision = %+v, want another priority decision for player 1 (the caster)", next)
	}
}

// TestNonActiveCasterReplayThroughSubmit extends TestCastReplayThroughSubmit's
// replay coverage (rather than rewriting it and losing the active-caster
// case) to the scenario Finding 1 exposed: player 1, not the active player,
// casts and resolves a spell targeting the active player. Confirms the live
// priority sequence -- caster keeps it through casting and targeting,
// active player gets it back only once the spell actually resolves (CR
// 117.5, unrelated to and unaffected by this fix) -- and that the scalar
// half of that state replays.
func TestNonActiveCasterReplayThroughSubmit(t *testing.T) {
	names := []string{"a", "b"}
	cfg := Config{Seed: 23, Names: names,
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	bolt := card(t, "Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	o := e.G.AddObject(bolt, 1)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{o.ID})
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	e.emit(events.Event{Kind: events.ManaAdd, Player: 1, Counter: "R", Amount: 1})

	passToPlayerOne(t, e)
	d := e.Pending()
	castIdx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == o.ID {
			castIdx = opt.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("no cast option: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{castIdx}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}

	target := e.Pending()
	if target == nil || target.Kind != decision.KTarget || target.Player != 1 {
		t.Fatalf("expected player 1's target decision, got %+v", target)
	}
	tIdx := -1
	for _, opt := range target.Options {
		if opt.Kind == "player" && opt.Player == 0 {
			tIdx = opt.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("active player not offered as a target: %+v", target.Options)
	}
	if err := e.Submit(decision.Intent{Seq: target.Seq, Player: 1, Choices: []int{tIdx}}); err != nil {
		t.Fatalf("submit target: %v", err)
	}
	if e.G.Priority != 1 {
		t.Fatalf("Priority = %d, want 1 (the caster) per CR 117.3c", e.G.Priority)
	}

	// Both players pass; the spell resolves.
	for i := 0; i < 4 && len(e.G.Stack) > 0; i++ {
		if passAll(t, e, 1) == 0 {
			t.Fatal("expected priority decisions until the spell resolves")
		}
	}
	if e.G.Players[0].Life != 17 {
		t.Fatalf("life = %d, want 17 after Bolt resolves", e.G.Players[0].Life)
	}

	fresh := state.NewGame(cfg.Names)
	for _, ev := range e.L.Events {
		events.Apply(fresh, ev)
	}
	if fresh.Players[1].Pool != e.G.Players[1].Pool {
		t.Errorf("replayed pool = %v, want %v (live)", fresh.Players[1].Pool, e.G.Players[1].Pool)
	}
	if fresh.Players[0].Life != e.G.Players[0].Life {
		t.Errorf("replayed life = %d, want %d (live)", fresh.Players[0].Life, e.G.Players[0].Life)
	}
	if fresh.Passes != e.G.Passes {
		t.Errorf("replayed Passes = %d, want %d (live)", fresh.Passes, e.G.Passes)
	}
	if fresh.Priority != e.G.Priority {
		t.Errorf("replayed Priority = %d, want %d (live)", fresh.Priority, e.G.Priority)
	}
}
