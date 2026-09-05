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
