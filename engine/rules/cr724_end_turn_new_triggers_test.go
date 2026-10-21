package rules

// CR 724.1a/f: when an effect ends the turn, the triggered abilities that
// triggered BEFORE the process began cease to exist, but any that trigger
// DURING it (724.1b's exile of the stack, 724.1c's state-based actions) are
// put onto the stack in the cleanup step, the active player gets priority,
// and another cleanup step follows.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// endTurnCast puts the named cards onto seat 0's battlefield (seat 0 is the
// active player in its turn-1 Main1), funds and casts the caster seat's
// spell, and passes until the turn ends. It returns the engine, its config
// and the ids moved.
func endTurnCast(t *testing.T, caster state.PlayerID, spell string, mana string, perms ...string) (*Engine, Config, map[string]state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	island := lookup(t, reg, "Island")
	var decks [2][]*cards.Card
	decks[caster] = []*cards.Card{lookup(t, reg, spell)}
	for _, n := range perms {
		decks[0] = append(decks[0], lookup(t, reg, n))
	}
	fill := func(d []*cards.Card) []*cards.Card {
		for len(d) < 60 {
			d = append(d, island)
		}
		return d
	}
	cfg := seatZeroStart(Config{Seed: 42, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{fill(decks[0]), fill(decks[1])}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	ids := map[string]state.ObjID{}
	for _, n := range perms {
		ids[n] = moveByName(t, e, 0, n, state.ZBattlefield)
	}
	ids[spell] = moveByName(t, e, caster, spell, state.ZHand)
	addMana(t, e, caster, mana)
	e.beginCast(caster, decision.Option{Kind: "cast", Obj: ids[spell]})
	if e.G.Obj(ids[spell]).Zone != state.ZStack {
		t.Fatalf("precondition: %s was not cast (zone %s)", spell, e.G.Obj(ids[spell]).Zone)
	}
	for i := 0; i < 400 && !e.G.Over && e.G.Turn == 1; i++ {
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("non-priority decision %+v while ending the turn", d)
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	if e.G.Turn == 1 {
		t.Fatalf("the turn never ended (still turn %d, step %s)", e.G.Turn, e.G.Step)
	}
	return e, cfg, ids
}

// TestCR724_1fTriggerDuringEndTurnResolves: seat 1 casts Time Stop in seat
// 0's turn. Seat 0's The Heron Moon ("Whenever one or more cards an opponent
// owns are put into exile, put a release counter on The Heron Moon") sees
// Time Stop, a card seat 1 owns, exiled from the stack (CR 724.1b). That
// ability triggered DURING the process, so it is put onto the stack in the
// cleanup step and resolves (CR 724.1f): The Heron Moon ends the turn with
// one release counter.
func TestCR724_1fTriggerDuringEndTurnResolves(t *testing.T) {
	t.Parallel()
	e, cfg, ids := endTurnCast(t, 1, "Time Stop", "UUUUUU", "The Heron Moon")
	if z := e.G.Obj(ids["Time Stop"]).Zone; z != state.ZExile {
		t.Fatalf("precondition: Time Stop ended in %s, want exile", z)
	}
	if n := e.G.Obj(ids["The Heron Moon"]).Counter("RELEASE"); n != 1 {
		t.Errorf("CR 724.1f seq %d: The Heron Moon has %d release counters, want 1 (its trigger from Time Stop's exile fired during the process and goes on the stack in cleanup)",
			len(e.L.Events), n)
	}
	replayCheck(t, e, cfg)
}

// TestCR724_1fHeronMoonSeesAPlainExile pins the trigger the acceptance test
// relies on: an opponent's card exiled outside the end-turn process gives The
// Heron Moon a release counter. Passes at base and with the fix.
func TestCR724_1fHeronMoonSeesAPlainExile(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "The Heron Moon")}, []*cards.Card{lookup(t, reg, "Time Stop")})
	moon := moveCorpusCard(t, e, "The Heron Moon", 0, state.ZBattlefield)
	moveCorpusCard(t, e, "Time Stop", 1, state.ZExile)
	passUntilStackEmpty(t, e, 20)
	if n := e.G.Obj(moon).Counter("RELEASE"); n != 1 {
		t.Errorf("seq %d: The Heron Moon has %d release counters after an opposing card was exiled, want 1", len(e.L.Events), n)
	}
	replayCheck(t, e, cfg)
}

// TestCR724_1aTriggerBeforeEndTurnCeases is the guard: Ultima destroys every
// artifact and creature, then ends the turn. Blood Artist's death triggers
// fired BEFORE the process began, so they cease to exist (CR 724.1a): no
// player loses life. Passes at base and with the fix.
func TestCR724_1aTriggerBeforeEndTurnCeases(t *testing.T) {
	t.Parallel()
	e, cfg, _ := endTurnCast(t, 0, "Ultima", "WWWWW", "Blood Artist", "Grizzly Bears")
	if l := e.G.Players[1].Life; l != 20 {
		t.Errorf("CR 724.1a seq %d: seat 1 life %d, want 20 (Blood Artist's death triggers fired before the end-turn process and cease to exist)",
			len(e.L.Events), l)
	}
	var pushed int
	for _, ev := range e.L.Events {
		if ev.Kind == events.AbilityPush {
			pushed++
		}
	}
	if pushed != 0 {
		t.Errorf("CR 724.1a: %d abilities were put onto the stack, want 0", pushed)
	}
	replayCheck(t, e, cfg)
}
