package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The opening-hand Effect registration's event-matched (non-phase) half:
// Chancellor of the Annex's `DB$ Effect | Triggers$ TrigCounter |
// EffectOwner$ Opponent` registers a one-off Mode$ SpellCast delayed trigger
// per opponent, which fires on that opponent's first cast and counters the
// spell unless its controller pays {1}. The Phase half
// (TestChancellorOpeningEffectRegistersAndRunsItsPhaseTrigger) already has a
// pin; these cover the shape findings-sol5 broke: the prior
// registerOpeningEffectTriggers silently dropped every non-Phase child.

// slowSpellCard is a targetless zero-cost spell whose resolution draws one:
// cheap enough to cast from an empty pool, and observable (the draw) when it
// resolves.
func slowSpellCard(t *testing.T) *cards.Card {
	t.Helper()
	return card(t, "Name:Slow Spell\nManaCost:0\nTypes:Sorcery\nA:SP$ Draw | Defined$ You\nOracle:x\n")
}

func TestChancellorOfTheAnnexRegistersOneSpellCastTriggerPerOpponent(t *testing.T) {
	annex := corpusAlternativeCard(t, "Chancellor of the Annex")

	e := handEngine(t, annex)
	id := e.G.Zone(state.ZHand, 0)[0]
	e.pending = nil
	e.applyOpeningEffect(openingEffect{player: 0, card: id, svar: "RevealCard"})
	if len(e.G.Delayed) != 1 {
		t.Fatalf("two-seat game registered %d delayed triggers, want one per opponent", len(e.G.Delayed))
	}
	dt := e.G.Delayed[0]
	if dt.EventMode != "SpellCast" || dt.Trigger != "TrigCounter" ||
		dt.Execute != "TrigCounterSpell" || dt.Controller != 1 || dt.Source != id {
		t.Fatalf("Chancellor of the Annex registration = %+v", dt)
	}

	// The EffectOwner$ Opponent fan-out: one registration per other seat, in
	// seat order, each owned by that opponent.
	three := newSeats(t, 3)
	three.pending = nil
	three.G.SetZone(state.ZHand, 0, nil)
	o := three.G.AddObject(annex, 0)
	o.Zone = state.ZHand
	three.G.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
	three.applyOpeningEffect(openingEffect{player: 0, card: o.ID, svar: "RevealCard"})
	if len(three.G.Delayed) != 2 {
		t.Fatalf("three-seat game registered %d delayed triggers, want one per opponent", len(three.G.Delayed))
	}
	for i, want := range []state.PlayerID{1, 2} {
		if got := three.G.Delayed[i].Controller; got != want {
			t.Fatalf("registration %d controller = %d, want %d", i, got, want)
		}
		if got := three.G.Delayed[i].EventMode; got != "SpellCast" {
			t.Fatalf("registration %d EventMode = %q", i, got)
		}
	}
}

func TestChancellorOfTheAnnexCountersOnlyEachOpponentsFirstSpell(t *testing.T) {
	annex := corpusAlternativeCard(t, "Chancellor of the Annex")
	e := handEngine(t, annex)
	id := e.G.Zone(state.ZHand, 0)[0]
	e.pending = nil
	e.applyOpeningEffect(openingEffect{player: 0, card: id, svar: "RevealCard"})
	if len(e.G.Delayed) != 1 {
		t.Fatalf("registration = %+v", e.G.Delayed)
	}

	slow := slowSpellCard(t)
	s0 := e.G.AddObject(slow, 0)
	s0.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{id, s0.ID})
	s1 := e.G.AddObject(slow, 1)
	s1.Zone = state.ZHand
	s1b := e.G.AddObject(slow, 1)
	s1b.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{s1.ID, s1b.ID})

	delayedPushes := func() int {
		n := 0
		for _, ev := range e.L.Events {
			if ev.Kind == events.DelayedPush {
				n++
			}
		}
		return n
	}

	// The REVEALER's own first spell is not "each opponent's first spell": the
	// registration is owned by seat 1 and must not fire on seat 0's cast.
	e.G.Active, e.G.Priority = 0, 0
	e.askPriority(0)
	pre := countDraw(e)
	submitChoices(t, e, passToCast(t, e, s0.ID))
	passUntilStackEmpty(t, e, 20)
	if countDraw(e) != pre+1 {
		t.Fatalf("seat 0's own spell drew %d, want exactly one resolution", countDraw(e)-pre)
	}
	if n := delayedPushes(); n != 0 {
		t.Fatalf("seat 0's own cast fired the opponent-owned trigger (%d DelayedPush)", n)
	}
	if len(e.G.Delayed) != 1 {
		t.Fatalf("seat 0's own cast consumed the registration: %+v", e.G.Delayed)
	}

	// Seat 1's first spell, empty pool: the {1} tax cannot be paid, so the
	// decline counters it and the registration is consumed (one-shot).
	e.G.Active, e.G.Priority = 1, 1 // a sorcery needs its controller's own main phase
	e.askPriority(1)
	submitChoices(t, e, passToCast(t, e, s1.ID))
	pay := drainUntilUnlessPay(t, e, 40)
	if pay == nil {
		t.Fatal("seat 1's first cast was never taxed")
	}
	if pay.Player != 1 {
		t.Fatalf("unless_pay payer = seat %d, want the taxed spell's controller (seat 1)", pay.Player)
	}
	submitChoices(t, e, pay.Options[1].Index) // decline
	passUntilStackEmpty(t, e, 20)
	if z := e.G.Obj(s1.ID).Zone; z != state.ZGraveyard {
		t.Fatalf("unpayable first spell zone = %s, want Graveyard", z)
	}
	if countDraw(e) != pre+1 {
		t.Fatalf("countered spell still resolved (draw count moved by %d)", countDraw(e)-pre-1)
	}
	if n := delayedPushes(); n != 1 {
		t.Fatalf("DelayedPush count = %d, want exactly one", n)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("first spell did not consume the registration: %+v", e.G.Delayed)
	}
	if !hasEventKind(e, events.ModeChosen) {
		t.Fatal("no ModeChosen event recorded the unless-pay answer")
	}

	// A SECOND spell is not the first spell: no registration remains, so it
	// resolves normally.
	e.G.Active, e.G.Priority = 1, 1
	e.askPriority(1)
	submitChoices(t, e, passToCast(t, e, s1b.ID))
	passUntilStackEmpty(t, e, 20)
	if countDraw(e) != pre+2 {
		t.Fatalf("second spell resolved %d draws past baseline, want one", countDraw(e)-pre-1)
	}
	if n := delayedPushes(); n != 1 {
		t.Fatalf("second cast fired a dead registration (%d DelayedPush)", n)
	}
}

// TestEventDelayedTriggerInterveningIfUsesEffectOwnersLife pins the review
// finding on checkEventDelayedTriggers: an event-matched registration's
// intervening-if "you" must resolve to the registration's effect owner
// (dt.Controller), not the source card's own controller. A synthetic
// EffectOwner$ Opponent registration carries a LifeTotal$ You | LifeAmount$
// GE10 condition; the revealer (source's controller) is left below 10 life
// and the opponent (the registration's effect owner) above it, so the
// trigger fires only if "you" is read as the effect owner.
func TestEventDelayedTriggerInterveningIfUsesEffectOwnersLife(t *testing.T) {
	watcher := card(t, "Name:Test Watcher\nManaCost:0\nTypes:Creature\nPT:1/1\n"+
		"K:MayEffectFromOpeningHand:RevealCard\n"+
		"SVar:RevealCard:DB$ Effect | Triggers$ TrigWatch | EffectOwner$ Opponent | Duration$ Permanent\n"+
		"SVar:TrigWatch:Mode$ SpellCast | ValidActivatingPlayer$ You | LifeTotal$ You | LifeAmount$ GE10 | "+
		"Execute$ TrigWatchNote | OneOff$ True | TriggerZones$ Command\n"+
		"SVar:TrigWatchNote:DB$ Draw\n"+
		"Oracle:x\n")

	e := handEngine(t, watcher)
	id := e.G.Zone(state.ZHand, 0)[0]
	e.pending = nil
	e.applyOpeningEffect(openingEffect{player: 0, card: id, svar: "RevealCard"})
	if len(e.G.Delayed) != 1 {
		t.Fatalf("registered %d delayed triggers, want 1", len(e.G.Delayed))
	}

	e.G.Players[0].Life = 5  // the revealer, source's controller: below the threshold
	e.G.Players[1].Life = 20 // the effect owner, dt.Controller: above it

	slow := slowSpellCard(t)
	s1 := e.G.AddObject(slow, 1)
	s1.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{s1.ID})

	e.G.Active, e.G.Priority = 1, 1
	e.askPriority(1)
	pre := countDraw(e)
	submitChoices(t, e, passToCast(t, e, s1.ID))
	passUntilStackEmpty(t, e, 20)
	// One draw from Slow Spell's own resolution, one more from TrigWatchNote
	// if (and only if) the intervening-if held against the effect owner's
	// life rather than the revealer's.
	if got, want := countDraw(e)-pre, 2; got != want {
		t.Fatalf("draws = %d, want %d (intervening-if must read the effect owner's life, not the revealer's)", got, want)
	}
}

// TestChancellorOfTheAnnexOpeningRevealDrivesTheCounter drives the real
// pregame flow (New -> the opening_yes decision -> handleOpening) rather than
// calling applyOpeningEffect directly, so the registration is proven to come
// out of the actual reveal, and the opponent's first spell is countered
// through the ordinary stack.
func TestChancellorOfTheAnnexOpeningRevealDrivesTheCounter(t *testing.T) {
	annex := corpusAlternativeCard(t, "Chancellor of the Annex")
	fill := card(t, "Name:Filler\nTypes:Basic Land\nOracle:x\n")
	slow := slowSpellCard(t)
	deck0 := func() []*cards.Card {
		out := make([]*cards.Card, 40)
		out[0] = annex
		for i := 1; i < len(out); i++ {
			out[i] = fill
		}
		return out
	}
	deck1 := func() []*cards.Card {
		out := make([]*cards.Card, 40)
		out[0] = slow
		for i := 1; i < len(out); i++ {
			out[i] = fill
		}
		return out
	}
	for seed := uint64(1); seed < 400; seed++ {
		e := New(Config{Seed: seed, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{deck0(), deck1()}})
		d := e.Pending()
		if d == nil || len(d.Options) == 0 || d.Options[0].Kind != "opening_yes" || d.Player != 0 {
			continue
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
		var slowID state.ObjID
		for _, sid := range e.G.Zone(state.ZHand, 1) {
			if e.G.Obj(sid).Face().Name == "Slow Spell" {
				slowID = sid
			}
		}
		if slowID == 0 {
			continue // the spell did not reach seat 1's opening hand this seed
		}
		if len(e.G.Delayed) != 1 || e.G.Delayed[0].EventMode != "SpellCast" ||
			e.G.Delayed[0].Trigger != "TrigCounter" || e.G.Delayed[0].Controller != 1 {
			t.Fatalf("opening reveal registered %+v", e.G.Delayed)
		}
		pre := countDraw(e)
		e.G.Active, e.G.Priority = 1, 1 // a sorcery needs its controller's own main phase
		e.askPriority(1)
		submitChoices(t, e, passToCast(t, e, slowID))
		pay := drainUntilUnlessPay(t, e, 40)
		if pay == nil {
			t.Fatal("the opponent's first spell was never taxed")
		}
		if pay.Player != 1 {
			t.Fatalf("unless_pay payer = seat %d, want seat 1", pay.Player)
		}
		submitChoices(t, e, pay.Options[1].Index) // decline: cannot pay {1}
		passUntilStackEmpty(t, e, 20)
		if z := e.G.Obj(slowID).Zone; z != state.ZGraveyard {
			t.Fatalf("unpayable first spell zone = %s, want Graveyard", z)
		}
		if countDraw(e) != pre {
			t.Fatal("the countered spell still resolved")
		}
		// The registration's event-matched fields travel inside the
		// DelayedRegister event's Text, so a log-alone reconstruction (events
		// folded into a fresh Game, no engine) must agree with the live game
		// exactly -- registration shape, its one-shot consumption, and the
		// minted ability all included. Every card here came from cfg.Decks,
		// so the reconstruction can rebuild them.
		cfg := Config{Seed: seed, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck0(), deck1()}}
		if fresh := replayFromLog(t, cfg, e.L.Events); diffGames(e.G, fresh) != "" {
			t.Fatalf("log-alone reconstruction diverged:\n%s", diffGames(e.G, fresh))
		}
		return
	}
	t.Fatal("could not construct a seat-0 Chancellor opening hand")
}
