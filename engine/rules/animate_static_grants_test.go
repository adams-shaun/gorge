package rules

// cr-g220: an Animate staticAbilities$ Mode$ Continuous body that grants a
// keyword or ability, or reaches beyond the animated object, registers as a
// static the animated object HAS (CR 611.3a, 613.1f, 613.4c): Restless
// Spire's turn-gated first strike, The Irencrag's equipped pump, Mileva's
// indestructible to other creatures. A body with an unread Condition$ stays
// loud.

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const animGrantBearSrc = "Name:Grant Test Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// animGrantAnimatorSrc is an authored artifact whose free Animate makes it a
// 1/1 creature with the static body st.
func animGrantAnimatorSrc(st string) string {
	return "Name:Grant Test Animator\nTypes:Artifact\n" +
		"A:AB$ Animate | Cost$ 0 | Defined$ Self | Power$ 1 | Toughness$ 1 | Types$ Creature | staticAbilities$ ST | SpellDescription$ Animate.\n" +
		"SVar:ST:" + st + "\n" +
		"Oracle:x\n"
}

// animGrantNotes counts Notes whose text contains sub.
func animGrantNotes(e *Engine, sub string) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, sub) {
			n++
		}
	}
	return n
}

// animGrantSeatZero passes priority until seat 0 holds it.
func animGrantSeatZero(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 4; i++ {
		d := e.Pending()
		if d != nil && d.Kind == decision.KPriority && d.Player == 0 {
			return
		}
		passOnce(t, e)
	}
	t.Fatalf("seat 0 never got priority: %+v", e.Pending())
}

// animGrantFixture animates the authored animator carrying st next to a
// fixture bear, both on seat 0's battlefield.
func animGrantFixture(t *testing.T, st string) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	src := cardByName(t, animGrantAnimatorSrc(st))
	bear := cardByName(t, animGrantBearSrc)
	e, cfg := tokenReplGame(t, 5, src, bear)
	art := moveSeededCard(t, e, 0, src, state.ZBattlefield)
	b := moveSeededCard(t, e, 0, bear, state.ZBattlefield)
	addMana(t, e, 0, "")
	animGrantSeatZero(t, e)
	submitChoices(t, e, abilityOption(t, e, art, 0).Index)
	passUntilStackEmpty(t, e, 20)
	if e.Power(art) != 1 || e.Toughness(art) != 1 {
		t.Fatalf("precondition: the animation did not resolve, animator is %d/%d", e.Power(art), e.Toughness(art))
	}
	return e, cfg, art, b
}

func TestAnimateGrantsKeywordToOthers(t *testing.T) {
	t.Parallel()
	e, cfg, art, bear := animGrantFixture(t, "Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Indestructible | Description$ Other creatures you control have indestructible.")
	if !e.HasKeyword(bear, "Indestructible") {
		t.Fatalf("the bear is not indestructible while the animated artifact has the static")
	}
	if e.HasKeyword(art, "Indestructible") {
		t.Fatalf("Other: the animated artifact itself gained indestructible")
	}
	if n := animGrantNotes(e, "Animate staticAbilities$ ST"); n != 0 {
		t.Fatalf("a readable body still emitted %d Animate staticAbilities$ Notes", n)
	}
	turn := e.G.Turn
	passUntilTurn(t, e, turn+1)
	if e.G.Turn != turn+1 {
		t.Fatalf("precondition: did not reach turn %d (at %d)", turn+1, e.G.Turn)
	}
	if e.HasKeyword(bear, "Indestructible") {
		t.Fatalf("the grant outlived the until-end-of-turn animation")
	}
	replayCheck(t, e, cfg)
}

func TestAnimateUnreadConditionStaysLoud(t *testing.T) {
	t.Parallel()
	e, cfg, _, bear := animGrantFixture(t, "Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Indestructible | Condition$ Threshold | Description$ Threshold.")
	if n := animGrantNotes(e, "Animate staticAbilities$ ST mode Continuous is not implemented; ignored"); n != 1 {
		t.Fatalf("an unread Condition$ body emitted %d not-implemented Notes, want 1", n)
	}
	if e.HasKeyword(bear, "Indestructible") {
		t.Fatalf("an unread Condition$ body granted its keyword blanket")
	}
	replayCheck(t, e, cfg)
}

// animSpireActivate funds seat 0 with {U}{R} at the current Main1, hands it
// priority (an instant-speed activation on the opponent's turn) and
// resolves Restless Spire's animation.
func animSpireActivate(t *testing.T, e *Engine, spire state.ObjID) {
	t.Helper()
	toMain1(t, e)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 1})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 1})
	e.priorityRound()
	animGrantSeatZero(t, e)
	submitChoices(t, e, abilityOption(t, e, spire, 1).Index)
	passUntilStackEmpty(t, e, 20)
	if !e.IsCreature(spire) || e.Power(spire) != 2 {
		t.Fatalf("precondition: Restless Spire is not an animated 2/1 (creature=%v power=%d)", e.IsCreature(spire), e.Power(spire))
	}
}

func TestRestlessSpireFirstStrikeOnlyOnYourTurn(t *testing.T) {
	t.Parallel()
	spire := tokenReplCorpusCard(t, "Restless Spire")

	t.Run("your_turn", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 7, spire)
		id := moveSeededCard(t, e, 0, spire, state.ZBattlefield)
		if e.G.Active != 0 {
			t.Fatalf("precondition: seat 0 is not active (active %d)", e.G.Active)
		}
		animSpireActivate(t, e, id)
		if !e.HasKeyword(id, "First Strike") {
			t.Fatalf("animated on its controller's turn, Restless Spire has no first strike")
		}
		replayCheck(t, e, cfg)
	})

	t.Run("opponents_turn", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 7, spire)
		id := moveSeededCard(t, e, 0, spire, state.ZBattlefield)
		next := e.G.Turn + 1
		e.Advance()
		passUntilTurn(t, e, next)
		driveToStep(t, e, next, 1, state.StepMain1)
		if e.G.Active != 1 {
			t.Fatalf("precondition: seat 1 is not active (active %d)", e.G.Active)
		}
		animSpireActivate(t, e, id)
		if e.HasKeyword(id, "First Strike") {
			t.Fatalf("animated on the opponent's turn, Restless Spire has first strike")
		}
		replayCheck(t, e, cfg)
	})
}

func TestIrencragEverflamePumpsEquippedCreature(t *testing.T) {
	t.Parallel()
	crag := tokenReplCorpusCard(t, "The Irencrag")
	legend := cardByName(t, "Name:Grant Test Legend\nTypes:Legendary Creature Human\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGame(t, 9, crag, legend)
	cragID := moveSeededCard(t, e, 0, crag, state.ZBattlefield)
	legendID := moveSeededCard(t, e, 0, legend, state.ZBattlefield)
	addMana(t, e, 0, "")
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			passOnce(t, e)
			continue
		}
		// The Optional$ "become Everflame" question: answer yes.
		idx := d.Options[0].Index
		for _, o := range d.Options {
			if strings.EqualFold(o.Kind, "yes") || strings.EqualFold(o.Label, "yes") {
				idx = o.Index
			}
		}
		submitChoices(t, e, idx)
	}
	if got := e.Name(cragID); got != "Everflame, Heroes' Legacy" {
		t.Fatalf("precondition: The Irencrag did not become Everflame (name %q)", got)
	}
	if e.Power(legendID) != 2 {
		t.Fatalf("precondition: the unequipped legend is %d power, want 2", e.Power(legendID))
	}
	e.emit(events.Event{Kind: events.Attach, Obj: cragID, IDs: []state.ObjID{legendID}})
	if p, tg := e.Power(legendID), e.Toughness(legendID); p != 5 || tg != 5 {
		t.Fatalf("equipped legend is %d/%d, want 5/5", p, tg)
	}
	replayCheck(t, e, cfg)
}

// milevaEvidence answers the pending payment asks of one Tenth District
// Hero activation, electing exactly the named graveyard card as evidence.
func milevaEvidence(t *testing.T, e *Engine, evidence state.ObjID) {
	t.Helper()
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			return
		}
		if len(d.Options) > 0 && d.Options[0].Kind == "evidence" {
			submitChoices(t, e, evidenceChoices(t, d, evidence)...)
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	t.Fatalf("the activation's asks never settled: %+v", e.Pending())
}

func TestMilevaGivesOtherCreaturesIndestructible(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ids := edrBoard(t, reg, 11, map[string]state.Zone{
		"Tenth District Hero": state.ZBattlefield,
		"Grizzly Bears":       state.ZBattlefield,
		"Hill Giant":          state.ZGraveyard,
		"Serra Angel":         state.ZGraveyard,
	})
	hero, bears := ids["Tenth District Hero"], ids["Grizzly Bears"]
	if e.HasKeyword(bears, "Indestructible") {
		t.Fatalf("precondition: Grizzly Bears is indestructible before Mileva")
	}
	activate := func(idx int, mana string, evidence state.ObjID) {
		t.Helper()
		addMana(t, e, 0, mana)
		animGrantSeatZero(t, e)
		submitChoices(t, e, abilityOption(t, e, hero, idx).Index)
		milevaEvidence(t, e, evidence)
		passUntilStackEmpty(t, e, 20)
	}
	activate(0, "CW", ids["Hill Giant"])
	if !slices.Contains(e.Derived(hero).Types, "Detective") {
		t.Fatalf("precondition: Tenth District Hero did not become a Detective")
	}
	activate(1, "CCW", ids["Serra Angel"])
	if got := e.Name(hero); got != "Mileva, the Stalwart" {
		t.Fatalf("precondition: the hero did not become Mileva (name %q)", got)
	}
	if !e.HasKeyword(bears, "Indestructible") {
		t.Fatalf("Mileva's other creature is not indestructible")
	}
	if e.HasKeyword(hero, "Indestructible") {
		t.Fatalf("Other: Mileva herself gained indestructible")
	}
	replayCheck(t, e, cfg)
}
