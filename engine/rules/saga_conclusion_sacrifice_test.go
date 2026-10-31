package rules

// CR 704.5s / 714.4: a Saga whose lore counters reach its final chapter
// number, and which is not the source of a chapter ability that has
// triggered but not yet left the stack, is SACRIFICED by its controller.
// CR 714.2d / 714.3c / 703.4f: the chapter abilities, the final chapter
// number and the precombat-main lore counter all read the Saga's CURRENT
// abilities, so a Saga that has lost all abilities (CR 613.1f) gets no
// lore counter, triggers no chapter and is not sacrificed.
// (ticket cr-g07-saga-conclusion-sacrifice)

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// sagaSettle answers every pending decision (pass for priority, the first
// Min options otherwise) until the trigger queue and the stack are empty.
// Unlike answerQuiet it tolerates a zero-option decision with Min 0 (a
// library search that finds nothing) by submitting the empty answer.
func sagaSettle(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for i := 0; i < limit && !e.G.Over; i++ {
		if len(e.pendingTriggers) == 0 && len(e.G.Stack) == 0 {
			return
		}
		d := e.Pending()
		if d == nil {
			e.priorityRound()
			continue
		}
		sagaAnswer(t, e, d)
	}
	t.Fatal("sagaSettle did not settle")
}

// sagaAnswer submits pass for a priority decision, else the first Min options.
func sagaAnswer(t *testing.T, e *Engine, d *decision.Decision) {
	t.Helper()
	if d.Kind == decision.KPriority {
		for _, o := range d.Options {
			if o.Kind == "pass" {
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
					t.Fatalf("submit pass: %v", err)
				}
				return
			}
		}
		t.Fatalf("priority decision without a pass option: %+v", d.Options)
	}
	ch := []int{}
	for k := 0; k < d.Min && k < len(d.Options); k++ {
		ch = append(ch, d.Options[k].Index)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}); err != nil {
		t.Fatalf("submit %v: %v", d.Kind, err)
	}
}

// sagaCards looks up each named corpus card (lookup fails the test on a miss).
func sagaCards(t *testing.T, reg *cards.Registry, names ...string) []*cards.Card {
	t.Helper()
	out := make([]*cards.Card, 0, len(names))
	for _, n := range names {
		out = append(out, lookup(t, reg, n))
	}
	return out
}

// sagaAddLore puts n lore counters on the Saga the way a non-turn-based
// effect (proliferate, a Storyweave-style CounterNum$) does: one
// CounterChange, the controller publishing as the adder.
func sagaAddLore(e *Engine, saga state.ObjID, n int32) {
	prev := e.SetCounterAdder(0)
	e.emit(events.Event{Kind: events.CounterChange, Obj: saga, Counter: "LORE", Amount: n})
	e.SetCounterAdder(prev)
}

// sagaSacrificeMoves counts the battlefield departures of obj that carry the
// sacrifice action marker (events.IsSacrifice), and all its departures.
func sagaSacrificeMoves(e *Engine, obj state.ObjID) (sacrificed, departed int) {
	for _, ev := range e.L.Events {
		if ev.Kind != events.MoveZone || ev.Obj != obj || ev.From != state.ZBattlefield {
			continue
		}
		departed++
		if events.IsSacrifice(ev) {
			sacrificed++
		}
	}
	return sacrificed, departed
}

// TestSagaConclusionIsASacrifice: Urza's Saga reaches chapter III with
// Mayhem Devil (pro-shaper's own pair) on the battlefield. After chapter III
// resolves the Saga leaves the battlefield as a SACRIFICE (CR 704.5s), so
// Mayhem Devil's "Whenever a player sacrifices a permanent" triggers and the
// this-turn sacrifice record carries it.
func TestSagaConclusionIsASacrifice(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg,
		sagaCards(t, reg, "Urza's Saga", "Mayhem Devil", "Sol Ring"),
		nil)
	devil := moveByName(t, e, 0, "Mayhem Devil", state.ZBattlefield)
	saga := moveByName(t, e, 0, "Urza's Saga", state.ZBattlefield)
	sagaSettle(t, e, 80)
	if got := e.G.Obj(saga).Counter("LORE"); got != 1 {
		t.Fatalf("precondition: Saga entered with %d lore, want 1", got)
	}
	sagaAddLore(e, saga, 2) // chapters II and III
	sagaSettle(t, e, 200)

	if o := e.G.Obj(saga); o.Zone != state.ZGraveyard {
		t.Fatalf("Saga at lore 3 with chapter III resolved is in %s, want the graveyard", o.Zone)
	}
	sac, dep := sagaSacrificeMoves(e, saga)
	if dep != 1 || sac != 1 {
		t.Fatalf("Saga battlefield departures: %d, of which sacrifices: %d; want exactly one, a sacrifice (CR 704.5s)", dep, sac)
	}
	sacrificed := false
	for _, z := range e.G.Entered {
		if z.Obj == saga && z.From == state.ZBattlefield && z.Sacrificed && z.Controller == 0 {
			sacrificed = true
		}
	}
	if !sacrificed {
		t.Fatal("the Saga's departure is not on the this-turn sacrifice record (ZoneEntry.Sacrificed by seat 0)")
	}
	devilTriggers := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.TriggerPush && ev.Obj == devil
	})
	if devilTriggers != 1 {
		t.Fatalf("Mayhem Devil triggered %d times, want 1 (the Saga's conclusion is a sacrifice)", devilTriggers)
	}
}

// TestSagaThatCantBeSacrificedStays: a Saga with "This permanent can't be
// sacrificed" (a CantSacrifice Card.Self static, the corpus's SCantSac
// shape) at its final chapter stays on the battlefield: the SBA is a
// sacrifice, and a permanent that can't be sacrificed isn't (CR 701.21a).
func TestSagaThatCantBeSacrificedStays(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	stubborn := card(t, "Name:Stubborn Saga\nManaCost:1 W\nTypes:Enchantment Saga\n"+
		"K:Chapter:2:DBGain,DBGain\n"+
		"SVar:DBGain:DB$ GainLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\n"+
		"S:Mode$ CantSacrifice | ValidCard$ Card.Self | Description$ CARDNAME can't be sacrificed.\n"+
		"Oracle:I, II — You gain 1 life. Stubborn Saga can't be sacrificed.\n")
	e := corpusEngine(t, reg, []*cards.Card{stubborn}, nil)
	saga := moveByName(t, e, 0, "Stubborn Saga", state.ZBattlefield)
	sagaSettle(t, e, 80)
	sagaAddLore(e, saga, 1)
	sagaSettle(t, e, 80)
	e.checkStateBased()
	if o := e.G.Obj(saga); o.Zone != state.ZBattlefield {
		t.Fatalf("a Saga that can't be sacrificed is in %s after its final chapter, want the battlefield", o.Zone)
	}
	if got := e.G.Players[0].Life; got != 22 {
		t.Fatalf("life %d, want 22 (both chapters resolved)", got)
	}
}

// TestSagaConclusionWaitsOnlyForChapterAbilities: Urza's Saga at lore 2 has
// chapter II's "{2},{T}: create a Construct" ability. With that NON-chapter
// ability on the stack, a third lore counter triggers chapter III above it.
// Once chapter III resolves, the Saga is no longer the source of a chapter
// ability on the stack, so it is sacrificed (CR 704.5s) before the Construct
// ability resolves.
func TestSagaConclusionWaitsOnlyForChapterAbilities(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, sagaCards(t, reg, "Urza's Saga"), nil)
	saga := moveByName(t, e, 0, "Urza's Saga", state.ZBattlefield)
	sagaSettle(t, e, 80)
	sagaAddLore(e, saga, 1) // chapter II grants the Construct ability
	sagaSettle(t, e, 80)
	addMana(t, e, 0, "CC")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == saga {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("precondition: chapter II's Construct ability is not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("activate the Construct ability: %v", err)
	}
	for i := 0; i < 20 && len(e.G.Stack) == 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		sagaAnswer(t, e, d)
	}
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Source != saga {
		t.Fatalf("precondition: want the Saga's Construct ability alone on the stack, stack %v", e.G.Stack)
	}
	construct := e.G.Stack[0]
	sagaAddLore(e, saga, 1) // chapter III triggers above the Construct ability
	// Guard: chapter III has triggered but is not yet on the stack (CR
	// 704.5s "triggered but not yet left the stack"), so the Saga stays.
	e.checkStateBased()
	if o := e.G.Obj(saga); o.Zone != state.ZBattlefield {
		t.Fatalf("Saga is in %s while its chapter III is still pending, want the battlefield", o.Zone)
	}
	e.priorityRound()
	if len(e.G.Stack) != 2 || e.G.Stack[0] != construct {
		t.Fatalf("precondition: want chapter III above the Construct ability, stack %v", e.G.Stack)
	}
	// Guard: while chapter III is on the stack the Saga is the source of a
	// chapter ability that has not left the stack, so it stays.
	e.checkStateBased()
	if o := e.G.Obj(saga); o.Zone != state.ZBattlefield {
		t.Fatalf("Saga is in %s while its chapter III is still on the stack, want the battlefield", o.Zone)
	}
	for i := 0; i < 200; i++ {
		if len(e.pendingTriggers) == 0 && len(e.G.Stack) == 1 && e.G.Stack[0] == construct {
			break
		}
		d := e.Pending()
		if d == nil {
			e.priorityRound()
			continue
		}
		sagaAnswer(t, e, d)
	}
	if len(e.G.Stack) != 1 || e.G.Stack[0] != construct {
		t.Fatalf("precondition: chapter III should have resolved with the Construct ability still on the stack, stack %v", e.G.Stack)
	}
	if o := e.G.Obj(saga); o.Zone != state.ZGraveyard {
		t.Fatalf("Saga is in %s with chapter III resolved and only a non-chapter ability of it on the stack, want the graveyard (CR 704.5s)", o.Zone)
	}
}

// TestSagaThatLostItsAbilitiesHasNoChapters: Urza's Saga enchanted by
// Imprisoned in the Moon ("loses all other card types and abilities") has no
// chapter abilities (CR 714.2d: final chapter number 0). Its controller's
// precombat-main turn-based action puts no lore counter on it (CR 714.3c /
// 703.4f), lore counters an effect adds trigger no chapter, and the Saga is
// not sacrificed however many lore counters it has (CR 714.4). When the Aura
// leaves, the abilities return and the SBA sacrifices it.
func TestSagaThatLostItsAbilitiesHasNoChapters(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, sagaCards(t, reg, "Urza's Saga", "Imprisoned in the Moon"), nil)
	saga := moveByName(t, e, 0, "Urza's Saga", state.ZBattlefield)
	sagaSettle(t, e, 80)
	aura := lossAttach(t, e, "Imprisoned in the Moon", 0, saga)
	sagaSettle(t, e, 80)
	if !e.printedAbilitiesGone(e.G.Obj(saga)) {
		t.Fatal("precondition: Imprisoned in the Moon does not remove the Saga's abilities")
	}

	// CR 714.3c: no chapter abilities, no turn-based lore counter.
	e.advanceSagas(0)
	if got := e.G.Obj(saga).Counter("LORE"); got != 1 {
		t.Fatalf("the precombat-main action put a lore counter on an ability-less Saga: lore %d, want 1", got)
	}
	// An effect may still add lore counters; none triggers a chapter.
	sagaAddLore(e, saga, 2)
	for _, pt := range e.pendingTriggers {
		if pt.Source == saga && pt.Chapter {
			t.Fatalf("an ability-less Saga queued chapter %q", pt.Execute)
		}
	}
	sagaSettle(t, e, 80)
	e.checkStateBased()
	if o := e.G.Obj(saga); o.Zone != state.ZBattlefield {
		t.Fatalf("an ability-less Saga at lore 3 is in %s, want the battlefield (final chapter number 0)", o.Zone)
	}

	// Guard: with the Aura gone the Saga has its chapters again, and with
	// lore 3 >= 3 and no chapter ability pending it is sacrificed.
	e.emit(events.Event{Kind: events.MoveZone, Obj: aura, From: state.ZBattlefield, To: state.ZGraveyard})
	sagaSettle(t, e, 80)
	e.checkStateBased()
	if o := e.G.Obj(saga); o.Zone != state.ZGraveyard {
		t.Fatalf("Saga at lore 3 with its abilities back is in %s, want the graveyard", o.Zone)
	}
}

// TestSagaConclusionIgnoresOpponentCauseRestriction: Sigarda, Host of Herons'
// "spells and abilities your opponents control can't cause you to sacrifice
// permanents" is a ValidCause$ SpellAbility.OppCtrl CantSacrifice static. It
// does not stop the CR 704.5s SBA: an SBA has no cause, so an opponent's
// ability merely sitting on the stack is not the reason the Saga is
// sacrificed. Pre-fix SacrificeBlocked evaluated ValidCause$ against
// actionCause() (that unrelated ability) and wrongly left the Saga on the
// battlefield.
func TestSagaConclusionIgnoresOpponentCauseRestriction(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sagaCard := card(t, "Name:Cause Saga\nManaCost:1 W\nTypes:Enchantment Saga\n"+
		"K:Chapter:2:DBGain,DBGain\n"+
		"SVar:DBGain:DB$ GainLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\n"+
		"Oracle:I, II — You gain 1 life.\n")
	relic := card(t, "Name:Opp Relic\nTypes:Artifact\n"+
		"A:AB$ GainLife | Cost$ T | Defined$ You | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\n"+
		"Oracle:{T}: You gain 1 life.\n")
	e := corpusEngine(t, reg,
		[]*cards.Card{sagaCard, lookup(t, reg, "Sigarda, Host of Herons")},
		[]*cards.Card{relic})
	saga := moveByName(t, e, 0, "Cause Saga", state.ZBattlefield)
	moveByName(t, e, 0, "Sigarda, Host of Herons", state.ZBattlefield)
	relicID := moveByName(t, e, 1, "Opp Relic", state.ZBattlefield)
	sagaSettle(t, e, 80)
	if got := e.G.Obj(saga).Counter("LORE"); got != 1 {
		t.Fatalf("precondition: Saga entered with %d lore, want 1", got)
	}
	// The opponent's ability is the top of the stack at the SBA check.
	e.emit(events.Event{Kind: events.AbilityPush, Obj: relicID, Player: 1})
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Controller != 1 {
		t.Fatalf("precondition: want one opponent-controlled ability on the stack, stack %v", e.G.Stack)
	}
	sagaAddLore(e, saga, 1) // chapter II
	e.priorityRound()       // chapter II above the opponent's ability
	if len(e.G.Stack) != 2 || e.G.Obj(e.G.Stack[1]).Source != saga {
		t.Fatalf("precondition: want chapter II above the opponent's ability, stack %v", e.G.Stack)
	}
	e.resolveTop() // chapter II resolves; the opponent's ability stays
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Controller != 1 {
		t.Fatalf("precondition: the opponent's ability should still be on the stack, stack %v", e.G.Stack)
	}
	e.checkStateBased()
	if o := e.G.Obj(saga); o.Zone != state.ZGraveyard {
		t.Fatalf("Saga is in %s while an opponent's unrelated ability is on the stack, want the graveyard (an SBA has no cause)", o.Zone)
	}
}
