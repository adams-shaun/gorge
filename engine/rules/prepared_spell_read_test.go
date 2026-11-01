package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// preparedWildIdea resolves Sanar, Unfinished Genius with the named permanents
// on seat 0's battlefield and returns its prepared Wild Idea copy, a sorcery
// costing {3}{U}{R} in exile (CR 722.3c).
func preparedWildIdea(t *testing.T, permanents ...string) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	extras := []*cards.Card{lookup(t, reg, "Sanar, Unfinished Genius")}
	for _, name := range permanents {
		extras = append(extras, lookup(t, reg, name))
	}
	e, _ := corpusEngineCfg(t, reg, extras, nil)
	moveByName(t, e, 0, "Sanar, Unfinished Genius", state.ZHand)
	for _, name := range permanents {
		moveByName(t, e, 0, name, state.ZBattlefield)
	}
	addMana(t, e, 0, "UR")
	castNamed(t, e, "Sanar, Unfinished Genius")
	passUntilStackEmpty(t, e, 40)
	id := sosPreparedCopy(e, "Wild Idea")
	if id == 0 {
		t.Fatal("precondition: no Wild Idea prepared copy in exile")
	}
	return e, id
}

// noPredicatePrograms clears e's compiled predicate sidecar, under which a
// member spec is answered without the textual filter: a read must give the same
// answer through either route.
func noPredicatePrograms(e *Engine) {
	text := *e.compiledText
	text.predicates = nil
	e.compiledText = &text
}

func TestPreparedSpellCastReads(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, permanent, mana string
		offered               bool
	}{
		{"Baral reduces it", "Baral, Chief of Compliance", "URCC", true},
		{"Baral reduces only {1}", "Baral, Chief of Compliance", "URC", false},
		{"Spectacle Mage reduces it", "Spectacle Mage", "URCC", true},
		{"Thalia raises it", "Thalia, Guardian of Thraben", "URCCC", false},
		{"Thalia's raise is payable", "Thalia, Guardian of Thraben", "URCCCC", true},
		{"Gaddock Teeg prohibits it", "Gaddock Teeg", "URCCC", false},
	} {
		for _, sidecar := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/sidecar=%v", tc.name, sidecar), func(t *testing.T) {
				t.Parallel()
				e, id := preparedWildIdea(t, tc.permanent)
				if !sidecar {
					noPredicatePrograms(e)
				}
				addMana(t, e, 0, tc.mana)
				if _, got := sosPreparedCastOption(e, id); got != tc.offered {
					t.Fatalf("offered = %v, want %v, with %s floating: %+v", got, tc.offered, tc.mana, e.Pending().Options)
				}
			})
		}
	}
}

// Off-sorcery timing: Hypersonic Dragon's "You may cast sorcery spells as
// though they had flash" reaches the prepared sorcery.
func TestPreparedSpellFlashGrant(t *testing.T) {
	t.Parallel()
	e, id := preparedWildIdea(t, "Hypersonic Dragon")
	noPredicatePrograms(e)
	passUntilStep(t, e, 3, state.StepUpkeep)
	for _, r := range "URCCC" {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(r), Amount: 1})
	}
	e.priorityRound()
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("pending = %+v, want seat 0's upkeep priority", d)
	}
	if _, ok := sosPreparedCastOption(e, id); !ok {
		t.Fatalf("the flash grant did not reach the prepared sorcery: %+v", e.Pending().Options)
	}
}

// Witherbloom's affinity prices the offer and the charge alike: the copy is a
// spell its controller casts (CR 722.3c), on the stack as much as in exile.
func TestPreparedSpellAffinityGrantIsCharged(t *testing.T) {
	t.Parallel()
	e, id := preparedWildIdea(t, "Witherbloom, the Balancer")
	addMana(t, e, 0, "URC")
	opt, ok := sosPreparedCastOption(e, id)
	if !ok {
		t.Fatalf("not offered with {U}{R}{C} and two creatures: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	if z := e.G.Obj(id).Zone; z != state.ZStack {
		t.Fatalf("copy zone = %s, pending %+v", z, e.Pending())
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("%d mana left floating", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.SpellCastReversed {
			t.Fatalf("the cast reversed: %+v", ev)
		}
	}
}

// Abstract Paintmage's {U}{R} may be spent on the prepared sorcery (CR 106.6).
// With three lands that is the only way to pay; with five, the plan still spends
// it rather than tapping every land around it.
func TestPreparedSpellPlanSpendsRestrictedMana(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		lands []string
	}{
		{"three lands", []string{"Mountain", "Mountain", "Mountain"}},
		{"five lands", []string{"Island", "Island", "Mountain", "Mountain", "Mountain"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, id := preparedWildIdea(t, append([]string{"Abstract Paintmage"}, tc.lands...)...)
			passUntilStep(t, e, 3, state.StepMain1)
			passUntilStackEmpty(t, e, 40)
			if got := e.G.Players[0].Pool.Total(); got != 2 {
				t.Fatalf("precondition: Paintmage floats %d mana, want 2", got)
			}
			ppPriority(t, e)
			pp := faceCastPlan(t, e.PotentialPaymentPlans(0), id, "prepared_copy")
			if pp.Plan == nil || pp.Reason != "" {
				t.Fatalf("verdict %+v, want a witness", pp)
			}
			if got := lrPlanString(pp.Plan.Activations); len(pp.Plan.Activations) != 3 {
				t.Fatalf("witness taps %s, want three lands", got)
			}
		})
	}
}

// Mm'menon's artifact mana "only to cast a spell from anywhere other than your
// hand" pays for the copy, which is cast from exile.
func TestPreparedSpellSpendsNotFromHandMana(t *testing.T) {
	t.Parallel()
	e, id := preparedWildIdea(t, "Mm'menon, the Right Hand", "Mox Opal")
	addMana(t, e, 0, "RCCC")
	ppPriority(t, e)
	pp := faceCastPlan(t, e.PotentialPaymentPlans(0), id, "prepared_copy")
	if pp.Reason != "" || len(pp.Script) == 0 {
		t.Fatalf("verdict %+v, want the artifact's {U} activated first", pp)
	}
}

// The copy is not a card in exile to any read but the cast's own: Cosmogoyf does
// not count it and Pull from Eternity has nothing to target.
func TestPreparedCopyIsNotACardInExile(t *testing.T) {
	t.Parallel()
	e, copyID := preparedWildIdea(t, "Pull from Eternity", "Cosmogoyf")
	var goyf state.ObjID
	for _, name := range []string{"Pull from Eternity", "Cosmogoyf"} {
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if o := e.G.Obj(id); o.Face().Name == name {
				if name == "Cosmogoyf" {
					goyf = id
				} else {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZHand})
				}
			}
		}
	}
	if e.matchesSpecFrom("Card", goyf, 0, goyf) && e.matchesSpecFrom("Card", copyID, 0, copyID) {
		t.Errorf("the compiled predicate for Card matches the prepared copy")
	}
	if got := e.Derived(goyf).Power; got != 0 {
		t.Errorf("Cosmogoyf counts %d cards in exile, want 0", got)
	}
	addMana(t, e, 0, "W")
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Label == "Cast Pull from Eternity" {
			t.Errorf("Pull from Eternity offered with only the prepared copy in exile: %+v", o)
		}
	}
}
