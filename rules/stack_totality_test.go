// Final whole-branch review, Critical C1: Task 29 added a totality guard
// (ensureLeftTheStack, rules/stack.go) for exactly one of resolveTop's five
// stack exits -- a permanent spell's ETB Move onto the battlefield. The
// other four (askTarget's cast-time "countered: no legal targets", an
// ability object's fizzle and its normal ceases-to-exist move, a spell's
// resolution-time fizzle, and -- the two cases this file covers -- an
// instant/sorcery's normal resolution and the ability-object exile move
// once more, from a different trigger source) carried the identical hazard,
// unguarded, since c19097f: a ReplacementResult$-absent R:Event$ Moved
// replacement that matches one of those moves and does not itself relocate
// the card leaves the object on the stack forever. The reviewer measured
// this as a genuine non-terminating match (100 000+ intents, still turn 1)
// reachable from 24 corpus cards this build's own coverage gate calls fully
// playable, including Rest in Peace and Dryad Militant.
//
// Card text below is authored for this test, in the same R:/SVar$ shape as
// the corpus cards that found the bug -- never copied from Forge's
// .cards/cardsfolder.
package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// plainLandSrc is a land with no replacement of its own, used only to give
// TestAbilityObjectResolvingUnderAnExileReplacementDoesNotStickOnTheStack a
// legal play_land action that fires watcherSrc's ETB trigger.
const plainLandSrc = `Name:Open Field
Types:Land
Oracle:x
`

// harmlessInstantSrc is a targetless instant (Defined$ You, not a chosen
// target), so casting and resolving it never needs a target decision --
// keeping this test about the totality guard, not target selection.
const harmlessInstantSrc = `Name:Minor Boon
ManaCost:R
Types:Instant
A:SP$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:x
`

// graveyardBlockingSpellsReplacementSrc is Dryad Militant's printed shape
// (final review, C1): the same broad "would go to a graveyard" replacement
// as graveyardBlockingReplacementSrc (replacement_updated_test.go), narrowed
// to ValidCard$ Instant,Sorcery instead of Card -- exactly the corpus's own
// narrowing, and exactly what makes an ordinary instant resolving under it
// the reviewer's reproduction.
const graveyardBlockingSpellsReplacementSrc = `Name:Militant Ward
ManaCost:1 W
Types:Creature Human Soldier
PT:2/1
R:Event$ Moved | Destination$ Graveyard | ValidCard$ Instant,Sorcery | ReplaceWith$ ExileInstead | Description$ if an instant or sorcery card would be put into a graveyard from anywhere, exile it instead
SVar:ExileInstead:DB$ ChangeZone | Origin$ All | Destination$ Exile | Defined$ ReplacedCard
Oracle:x
`

// exileBlockingReplacementSrc mirrors graveyardBlockingReplacementSrc's
// shape but for Destination$ Exile instead of Graveyard -- the review's I-2
// shape (an ability object's own ceases-to-exist move, normally parked in
// exile per CR 608.2m, discarded by an unrelated replacement). ReplaceWith$
// again names the unmodeled Defined$ ReplacedCard, so it relocates nothing.
const exileBlockingReplacementSrc = `Name:Void Ward
ManaCost:1 U
Types:Enchantment
R:Event$ Moved | Destination$ Exile | ValidCard$ Card | ReplaceWith$ RegraveInstead | Description$ if a card or ability would be exiled from anywhere, put it into a graveyard instead
SVar:RegraveInstead:DB$ ChangeZone | Origin$ All | Destination$ Graveyard | Defined$ ReplacedCard
Oracle:x
`

// TestInstantResolvingUnderAGraveyardReplacementDoesNotStickOnTheStack is
// C1's regression test (a): an ordinary instant, cast and resolved while a
// Dryad-Militant-shaped replacement is on the battlefield. Before the fix
// (resolveTop's else branch had no ensureLeftTheStack call), this hangs: the
// instant's own Stack->Graveyard Move is discarded by the replacement (whose
// ReplaceWith$ relocates nothing), nothing else removes it from the stack,
// and the bounded drive below exhausts its budget with the spell still on
// top, re-resolving (GainLife re-applying) every single pass.
func TestInstantResolvingUnderAGraveyardReplacementDoesNotStickOnTheStack(t *testing.T) {
	e, _, id := newFixtureDeck(t, 110, harmlessInstantSrc)
	onBoard(t, e, 1, graveyardBlockingSpellsReplacementSrc)
	driveToStep(t, e, 1, 0, state.StepMain1)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 1})
	e.priorityRound()

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority after genesis, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == id {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the fixture instant: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}
	if len(e.G.Stack) != 1 || e.G.Stack[0] != id {
		t.Fatalf("stack = %v, want [%d] right after casting", e.G.Stack, id)
	}

	const bound = 60
	n := passUntilStackEmpty(t, e, bound)
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v after %d bounded priority passes with a graveyard-blocking "+
			"replacement in play -- the instant/sorcery resolve exit is unguarded (final "+
			"review Critical C1)", e.G.Stack, bound)
	}
	if n != 2 {
		t.Fatalf("passes to drain the stack = %d, want exactly 2", n)
	}
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("zone = %s, want graveyard -- Militant Ward's ReplaceWith$ (Defined$ "+
			"ReplacedCard, unmodeled) relocates nothing, so the guard's own graveyard move "+
			"must be what lands it there", got)
	}
	if n := countKind(e.L.Events, events.Note, id); n != 1 {
		t.Fatalf("logged %d Note events for the guard's escape hatch, want exactly 1", n)
	}
	if n := countKind(e.L.Events, events.Resolve, id); n != 1 {
		t.Fatalf("logged %d Resolve events for the instant, want exactly 1 -- more means it "+
			"re-resolved instead of terminating", n)
	}
	if e.Pending() == nil {
		t.Fatal("no further decision pending -- the engine did not reach the next decision")
	}
}

// TestAbilityObjectResolvingUnderAnExileReplacementDoesNotStickOnTheStack is
// C1's regression test (b), and independently closes Task 29's own I-2
// ("the ability-object branch has the identical hazard... not fixed here"):
// a triggered ability (watcherSrc's plain ETB trigger, fired here by playing
// a harmless land) resolving normally while an exile-blocking replacement is
// on the battlefield. Before the fix, the ability object's own
// Stack->Exile "ceases to exist" Move is discarded and it re-resolves
// forever (final review measured intents=300000, never terminating, on an
// equivalent board).
func TestAbilityObjectResolvingUnderAnExileReplacementDoesNotStickOnTheStack(t *testing.T) {
	e, _, landID := newFixtureDeck(t, 111, plainLandSrc)
	onBoard(t, e, 0, watcherSrc)
	onBoard(t, e, 1, exileBlockingReplacementSrc)
	driveToStep(t, e, 1, 0, state.StepMain1)

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority after genesis, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "play_land" && opt.Obj == landID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no play_land option for the fixture land: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit play_land: %v", err)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the Sentinel's ETB trigger queued", e.G.Stack)
	}
	abilityID := e.G.Stack[0]

	const bound = 60
	n := passUntilStackEmpty(t, e, bound)
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v after %d bounded priority passes with an exile-blocking "+
			"replacement in play -- the ability-object resolve exit is unguarded (final "+
			"review Critical C1 / Task 29 I-2)", e.G.Stack, bound)
	}
	if n != 2 {
		t.Fatalf("passes to drain the stack = %d, want exactly 2", n)
	}
	if got := e.G.Obj(abilityID).Zone; got != state.ZExile {
		t.Fatalf("zone = %s, want exile -- Void Ward's ReplaceWith$ (Defined$ ReplacedCard, "+
			"unmodeled) relocates nothing, so the guard's own exile move must be what lands "+
			"it there", got)
	}
	if n := countKind(e.L.Events, events.Note, abilityID); n != 1 {
		t.Fatalf("logged %d Note events for the guard's escape hatch, want exactly 1", n)
	}
	if n := countKind(e.L.Events, events.Resolve, abilityID); n != 1 {
		t.Fatalf("logged %d Resolve events for the ability, want exactly 1 -- more means it "+
			"re-resolved instead of terminating", n)
	}
	if e.Pending() == nil {
		t.Fatal("no further decision pending -- the engine did not reach the next decision")
	}
}

// damageWatcherSrc is watcherSrc's shape (final review R1(a)) but with a
// TARGETED Execute$ SVar (DealDamage | ValidTgts$ Creature) instead of
// watcherSrc's untargeted GainLife -- the shape the review names for
// stack.go:216, the ability branch's own CR 608.2b recheck.
//
// No board state is needed to make the target illegal: under Task 7 a
// triggered ability with ValidTgts$ pops askTarget the instant it is placed
// (pushTrigger, immediately after its TriggerPush), so with no matching
// creature anywhere this trigger is miscast at ASK time -- askTarget's CR
// 608.2b fizzle moves the just-created ability object straight to exile,
// before it ever sits on the stack to be rechecked at resolution. That means
// the totality guard tested below (ensureLeftTheStack under an exile-blocking
// replacement) is now exercised at ask time, not at resolution -- the C1
// hang this file exists to forbid cannot occur either way.
const damageWatcherSrc = `Name:Reflex Sentinel
ManaCost:1
Types:Artifact
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card | Execute$ TrigZap | TriggerDescription$ x
SVar:TrigZap:DB$ DealDamage | ValidTgts$ Creature | NumDmg$ 1
Oracle:x
`

// TestAbilityFizzlingAtResolutionUnderAnExileReplacementDoesNotStickOnTheStack
// is final review R1(a): the ability branch's own CR 608.2b fizzle. Task 7
// moved WHEN that fizzle runs for a ValidTgts$ trigger from resolution-time
// (resolveTop's recheck) to ask-time: pushTrigger now asks targets the
// instant the trigger is placed, and askTarget with no legal target fizzles
// the just-created ability straight to exile rather than letting it sit on
// the stack. The totality property this file exists to pin -- a fizzling
// ability whose own Stack->Exile Move is discarded by
// exileBlockingReplacementSrc (ReplaceWith$ relocates nothing) must not hang
// or re-resolve forever -- is the same, but it is now exercised at ask time:
// the ability is created in the same Submit as the land entry and
// immediately parked in exile, the match terminates, and no Resolve ever
// runs. Before the guard, deleting askTarget's ensureLeftTheStack call
// leaves the ability stuck on the stack forever; this asserts the terminal
// state that guard delivers.
func TestAbilityFizzlingAtResolutionUnderAnExileReplacementDoesNotStickOnTheStack(t *testing.T) {
	e, _, landID := newFixtureDeck(t, 112, plainLandSrc)
	onBoard(t, e, 0, damageWatcherSrc)
	onBoard(t, e, 1, exileBlockingReplacementSrc)
	driveToStep(t, e, 1, 0, state.StepMain1)

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority after genesis, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "play_land" && opt.Obj == landID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no play_land option for the fixture land: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit play_land: %v", err)
	}
	// Task 7: the Sentinel's ValidTgts ETB trigger fizzles AT ASK TIME (no
	// creature to target), so the ability object never finishes its turn on
	// the stack -- it is created by its own TriggerPush and immediately
	// parked in exile by askTarget's CR 608.2b fizzle. Void Ward discards
	// that fizzle MoveZone (its ReplaceWith$ relocates nothing), so the
	// logged to-exile move is the guard's own re-emit; find the ability id
	// by that move (the only thing this board ever moves to exile).
	abilityID := state.ObjID(0)
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.To == state.ZExile {
			abilityID = ev.Obj
			break
		}
	}
	if abilityID == 0 {
		t.Fatal("no guarded ask-time fizzle MoveZone to exile found in the log")
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want empty -- the trigger's ability never sat on the stack "+
			"(ask-time fizzle)", e.G.Stack)
	}
	if got := e.G.Obj(abilityID).Zone; got != state.ZExile {
		t.Fatalf("zone = %s, want exile -- Void Ward's ReplaceWith$ (Defined$ ReplacedCard, "+
			"unmodeled) relocates nothing, so the guard's own exile move must be what lands "+
			"it there", got)
	}
	noteCount := 0
	noneLeft := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note {
			noteCount++
			if strings.Contains(ev.Text, "no legal targets") {
				noneLeft = true
			}
		}
	}
	if noteCount < 1 {
		t.Fatal("no guard Note logged for the fizzled ability")
	}
	if !noneLeft {
		t.Fatal(`no guard Note mentions "no legal targets"`)
	}
	if n := countKind(e.L.Events, events.Resolve, abilityID); n != 0 {
		t.Fatalf("logged %d Resolve events for the fizzled ability, want 0 -- a fizzle "+
			"returns before the Resolve emit and never runs the script", n)
	}
	if e.Pending() == nil {
		t.Fatal("no further decision pending -- the engine did not reach the next decision")
	}
}

// tappedCreatureTargetInstantSrc requires a TAPPED creature target. The
// review's shape for stack.go:137 calls for casting into "no creature
// anywhere", but the only reusable graveyard-blocking replacement fixture
// in this file (graveyardBlockingSpellsReplacementSrc, Militant Ward) is
// ITSELF a 2/1 creature: onBoarding it to keep its replacement active while
// this spell's own ValidTgts$ was the bare "Creature" the review's prose
// names would hand the spell a legal target of its own blocker's host and
// defeat the "no legal targets" premise the test needs. Narrowing to
// Creature.tapped keeps Militant Ward on the battlefield exactly the way
// TestInstantResolvingUnderAGraveyardReplacementDoesNotStickOnTheStack does
// (untapped, so it never qualifies) while still reaching askTarget's
// zero-option branch: CR 608.2b cares about LEGAL targets, and an untapped
// Militant Ward is not one under this spec.
const tappedCreatureTargetInstantSrc = `Name:Sudden Spark
ManaCost:R
Types:Instant
A:SP$ DealDamage | ValidTgts$ Creature.tapped | NumDmg$ 1
Oracle:x
`

// TestSpellCounteredForNoTargetsAtCastUnderAGraveyardReplacementDoesNotStickOnTheStack
// is final review R1(b): stack.go:137, askTarget's own CR 608.2b cast-time
// counter for a spell that finds zero legal targets the instant it is put
// on the stack (before any decision is ever asked). Before the fix,
// deleting the :137 call alone leaves a reachable game hung: the
// "countered: no legal targets" Move is discarded by
// graveyardBlockingSpellsReplacementSrc (its ReplaceWith$ relocates
// nothing, the same unmodeled Defined$ ReplacedCard as elsewhere in this
// file), and nothing else ever takes the spell off the stack.
func TestSpellCounteredForNoTargetsAtCastUnderAGraveyardReplacementDoesNotStickOnTheStack(t *testing.T) {
	e, _, id := newFixtureDeck(t, 113, tappedCreatureTargetInstantSrc)
	onBoard(t, e, 1, graveyardBlockingSpellsReplacementSrc)
	driveToStep(t, e, 1, 0, state.StepMain1)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 1})
	e.priorityRound()

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority after genesis, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == id {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the fixture instant: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}

	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v right after casting into zero legal targets, want empty -- "+
			"askTarget's countered-at-cast exit is unguarded (final review R1(b), "+
			"stack.go:137)", e.G.Stack)
	}
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("zone = %s, want graveyard -- Militant Ward's ReplaceWith$ (Defined$ "+
			"ReplacedCard, unmodeled) relocates nothing, so the guard's own graveyard move "+
			"must be what lands it there", got)
	}
	noteCount := 0
	noteMentionsCountered := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Obj == id {
			noteCount++
			if strings.Contains(ev.Text, "countered: no legal targets") {
				noteMentionsCountered = true
			}
		}
	}
	if noteCount != 1 {
		t.Fatalf("logged %d Note events for the guard's escape hatch, want exactly 1 per cast", noteCount)
	}
	if !noteMentionsCountered {
		t.Fatal(`no guard Note mentions "countered: no legal targets"`)
	}
	if e.Pending() == nil {
		t.Fatal("no further decision pending -- the engine did not reach the next decision")
	}
}

// twinDamageInstantSrc is a minimal creature-removal instant. Two copies of
// it, both aimed at the same 1/1 (fieldRatSrc below), are the review's shape
// for stack.go:291: the top copy resolves first and kills the shared
// target, so the bottom copy finds zero legal targets when ITS turn to
// resolve comes -- CR 608.2b's resolution-time recheck, not the cast-time
// one R1(b) above covers.
const twinDamageInstantSrc = `Name:Twin Spark
ManaCost:R
Types:Instant
A:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 1
Oracle:x
`

// fieldRatSrc is a vanilla 1/1 -- twinDamageInstantSrc's shared target, with
// exactly enough toughness that one copy's damage is lethal.
const fieldRatSrc = `Name:Field Rat
ManaCost:B
Types:Creature Rat
PT:1/1
Oracle:x
`

// TestSpellFizzlingAtResolutionUnderAGraveyardReplacementDoesNotStickOnTheStack
// is final review R1(c): stack.go:291, the spell branch's own CR 608.2b
// resolution-time recheck -- a target legal when chosen (fieldRatSrc, alive
// on the battlefield when both copies were cast and targeted) stops being
// legal before the bottom copy's own turn to resolve, because the top copy
// resolved first and killed it. Before the fix, deleting the :291 call
// alone leaves a reachable game hung: the bottom copy's "fizzled: no legal
// targets remain" Move is discarded by graveyardBlockingSpellsReplacementSrc
// (ReplaceWith$ relocates nothing, as elsewhere in this file), and nothing
// else ever takes it off the stack.
//
// Militant Ward's own ValidCard$ is "Instant,Sorcery", so it does not
// intercept fieldRatSrc's own (creature) death move -- only the two spells'
// exits are affected, keeping this test about the spell branch's guard, not
// the creature's.
func TestSpellFizzlingAtResolutionUnderAGraveyardReplacementDoesNotStickOnTheStack(t *testing.T) {
	spell := card(t, twinDamageInstantSrc)
	e := handEngine(t, spell, spell)
	target := onBoard(t, e, 1, fieldRatSrc)
	onBoard(t, e, 1, graveyardBlockingSpellsReplacementSrc)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 2})
	e.askPriority(0)

	chooseTarget := func() {
		t.Helper()
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("expected a target decision, got %+v", d)
		}
		idx := -1
		for _, opt := range d.Options {
			if opt.Kind == "permanent" && opt.Obj == target {
				idx = opt.Index
			}
		}
		if idx < 0 {
			t.Fatalf("no target option for the fixture rat: %+v", d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit target: %v", err)
		}
	}

	castFirst(t, e, "cast")
	chooseTarget()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the first Twin Spark", e.G.Stack)
	}
	bottom := e.G.Stack[0]
	castFirst(t, e, "cast")
	chooseTarget()
	if len(e.G.Stack) != 2 {
		t.Fatalf("stack = %v, want both copies of Twin Spark", e.G.Stack)
	}

	const bound = 60
	n := passUntilStackEmpty(t, e, bound)
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v after %d bounded priority passes with a graveyard-blocking "+
			"replacement in play -- the spell resolution-time fizzle exit is unguarded "+
			"(final review R1(c), stack.go:291)", e.G.Stack, bound)
	}
	if n != 4 {
		t.Fatalf("passes to drain the stack = %d, want exactly 4 -- two per resolution, "+
			"two resolutions", n)
	}
	if got := e.G.Obj(target).Zone; got != state.ZGraveyard {
		t.Fatalf("Field Rat zone = %s, want graveyard -- the top copy's damage should have "+
			"killed it", got)
	}
	if got := e.G.Obj(bottom).Zone; got != state.ZGraveyard {
		t.Fatalf("zone = %s, want graveyard -- Militant Ward's ReplaceWith$ (Defined$ "+
			"ReplacedCard, unmodeled) relocates nothing, so the guard's own graveyard move "+
			"must be what lands it there", got)
	}
	noteCount := 0
	noteMentionsFizzled := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Obj == bottom {
			noteCount++
			if strings.Contains(ev.Text, "fizzled: no legal targets") {
				noteMentionsFizzled = true
			}
		}
	}
	if noteCount < 1 {
		t.Fatal("no guard Note logged for the fizzled (bottom) spell")
	}
	if !noteMentionsFizzled {
		t.Fatal(`no guard Note mentions "fizzled: no legal targets"`)
	}
	if e.Pending() == nil {
		t.Fatal("no further decision pending -- the engine did not reach the next decision")
	}
}
