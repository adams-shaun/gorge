package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// castOptions lists the "cast" options for the first card in seat 0's hand.
func castOptions(t *testing.T, e *Engine) []decision.Option {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority: %+v", d)
	}
	var out []decision.Option
	for _, o := range d.Options {
		if o.Kind == "cast" {
			out = append(out, o)
		}
	}
	return out
}

// submitChoices submits choices against whatever decision is currently
// pending, for that decision's own player.
func submitChoices(t *testing.T, e *Engine, choices ...int) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit %v: %v", choices, err)
	}
}

// toMain1 drives to Main1 of whatever turn/seat is currently active --
// idempotent (driveToStep returns immediately once already there), which is
// what lets addMana/putToken be called any number of times across a test
// without re-driving anything.
func toMain1(t *testing.T, e *Engine) {
	t.Helper()
	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepMain1)
}

// addMana drives to Main1 (idempotent), adds one ManaAdd event per mana
// symbol in symbols (each byte one of WUBRGC) into p's pool, then re-asks
// priority so the pending decision (what castOptions/castFirst read)
// reflects the funded pool -- the same driveToStep-then-priorityRound shape
// replacement_updated_test.go's castAndResolveTappedCreature already uses
// for the identical reason.
func addMana(t *testing.T, e *Engine, p state.PlayerID, symbols string) {
	t.Helper()
	toMain1(t, e)
	for _, r := range symbols {
		e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: string(r), Amount: 1})
	}
	e.priorityRound()
}

// putToken adds a fresh, freely-authored fixture card (never a corpus .txt,
// per the licensing rule) into the game via a logged TokenCreate event and,
// if the destination isn't the battlefield, a second logged MoveZone -- both
// real, replayable events, unlike a direct e.G.AddObject call (which
// replayFromLog can never reconstruct: it only rebuilds from cfg.Decks, see
// newFixtureDeck's own doc comment on exactly this hazard). TokenCreate
// resolves the card from e.G.Tokens, keyed here by the object id about to be
// minted; that map is shared with the test's own cfg.Tokens (newFixtureDeck
// seeds it non-nil, replayFromLog wires g.Tokens = cfg.Tokens) precisely so
// a later replayCheck sees the same card under the same key. These fixture
// objects are marked IsToken by TokenCreate's own Apply case (a side effect
// of reusing that mechanism, not real Forge tokens) -- Ephemeral() reads
// that, but only view's client-facing projection consults Ephemeral; no
// rules-package zone walk (Zone/HasKeyword/MatchesSpec) treats it
// differently, so it has no effect on anything these tests exercise.
//
// Deliberately leaves e.pending nil (not a fresh ask): the caller's own next
// Advance() (or a later addMana's priorityRound) is what produces a pending
// decision reflecting every fixture placed since the last real ask, in one
// snapshot -- exactly what TestFlashbackCastsFromTheGraveyardPaysASacrificeAndExiles
// needs when it raw-emits a second setup move right after this and then
// calls Advance once for both.
func putToken(t *testing.T, e *Engine, p state.PlayerID, src string, to state.Zone) state.ObjID {
	t.Helper()
	toMain1(t, e)
	c := card(t, src)
	key := fmt.Sprintf("fixture:%d", e.G.NextID)
	if e.G.Tokens == nil {
		e.G.Tokens = map[string]*cards.Card{}
	}
	e.G.Tokens[key] = c
	e.emit(events.Event{Kind: events.TokenCreate, Player: p, Text: key})
	id := e.G.NextID - 1
	if to != state.ZBattlefield {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: to})
	}
	e.pending = nil
	return id
}

func putCreature(t *testing.T, e *Engine, p state.PlayerID, src string) state.ObjID {
	t.Helper()
	return putToken(t, e, p, src, state.ZBattlefield)
}

func addToGraveyard(t *testing.T, e *Engine, p state.PlayerID, src string) state.ObjID {
	t.Helper()
	return putToken(t, e, p, src, state.ZGraveyard)
}

func addToHand(t *testing.T, e *Engine, p state.PlayerID, src string) state.ObjID {
	t.Helper()
	return putToken(t, e, p, src, state.ZHand)
}

// castObj picks the cast option for id out of the current priority decision,
// answers a target decision with option 0 if one appears, then drains the
// stack -- a real player would let their spell resolve (or at least empty
// the stack) before taking a further sorcery-speed action, and the surge
// test's own Reckless Bushwhacker (a creature, sorcery-speed-only) is not
// offered at all while anything else sits on the stack.
func castObj(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for %d: %+v", id, d.Options)
	}
	submitChoices(t, e, idx)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget && len(d.Options) > 0 {
		submitChoices(t, e, d.Options[0].Index)
	}
	passUntilStackEmpty(t, e, 20)
}

// hasEvent reports whether the log carries an event of kind against obj.
func hasEvent(e *Engine, kind events.Kind, obj state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == kind && ev.Obj == obj {
			return true
		}
	}
	return false
}

// hasNote reports whether the log carries a Note event whose Text contains substr.
func hasNote(e *Engine, substr string) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, substr) {
			return true
		}
	}
	return false
}

// replayCheck rebuilds the game from the log alone and compares it with the
// live game — the same fidelity check trigger_test.go's replayFromLog gives.
func replayCheck(t *testing.T, e *Engine, cfg Config) {
	t.Helper()
	if diff := diffGames(e.G, replayFromLog(t, cfg, e.L.Events)); diff != "" {
		t.Fatalf("log-only replay differs:\n%s", diff)
	}
}

func TestXIsChosenAndRecorded(t *testing.T) {
	e, cfg, id := newFixtureDeck(t, 21, "Name:Endless\nManaCost:X\nTypes:Creature Eldrazi\nPT:0/0\nOracle:x\n")
	addMana(t, e, 0, "GGG") // helper: three ManaAdd events into seat 0's pool
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" || len(d.Options) != 4 || d.Options[3].Label != "X = 3" {
		t.Fatalf("X decision %+v", d)
	}
	submitChoices(t, e, 2)
	o := e.G.Obj(id)
	if o.Zone != state.ZStack || o.X != 2 || e.G.Players[0].Pool.Total() != 1 {
		t.Fatalf("after X=2: zone %s X %d pool %d", o.Zone, o.X, e.G.Players[0].Pool.Total())
	}
	if !hasEvent(e, events.CastInfo, id) {
		t.Fatal("no CastInfo event")
	}
	replayCheck(t, e, cfg)
}

func TestKickerOffersASecondCastOptionAndFlagsTheSpell(t *testing.T) {
	src := "Name:Whacker\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nK:Kicker:R\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self+kicked | Execute$ TrigPump | TriggerDescription$ if kicked\n" +
		"SVar:TrigPump:DB$ PumpAll | ValidCards$ Creature.YouCtrl | NumAtt$ +1\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 22, src)
	addMana(t, e, 0, "R")
	if opts := castOptions(t, e); len(opts) != 1 || opts[0].Mode != "" {
		t.Fatalf("one mana: %+v", opts)
	}
	addMana(t, e, 0, "R")
	opts := castOptions(t, e)
	if len(opts) != 2 || opts[1].Mode != "kicked" || opts[1].Label != "Cast Whacker (kicked)" {
		t.Fatalf("two mana: %+v", opts)
	}
	submitChoices(t, e, opts[1].Index)
	if o := e.G.Obj(id); o.CastFlags&state.FlagKicked == 0 || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("kicked cast: %+v pool %d", o, e.G.Players[0].Pool.Total())
	}
	passUntilStackEmpty(t, e, 20)
	if len(e.pendingTriggers) == 0 && !hasEvent(e, events.TriggerPush, id) {
		t.Fatal("the 'if kicked' trigger did not fire")
	}
	// Not kicked: the trigger must not fire.
	e2, _, id2 := newFixtureDeck(t, 23, src)
	addMana(t, e2, 0, "RR")
	submitChoices(t, e2, castOptions(t, e2)[0].Index)
	passUntilStackEmpty(t, e2, 20)
	if hasEvent(e2, events.TriggerPush, id2) {
		t.Fatal("the 'if kicked' trigger fired on an unkicked cast")
	}
}

func TestSurgeNeedsAnotherSpellThisTurn(t *testing.T) {
	src := "Name:Reckless\nManaCost:2 R\nTypes:Creature Goblin\nPT:2/1\nK:Surge:1 R\nK:Haste\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 24, src)
	bolt := addToHand(t, e, 0, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	addMana(t, e, 0, "RRR")
	for _, o := range castOptions(t, e) {
		if o.Mode == "surged" {
			t.Fatal("surge offered before any spell this turn")
		}
	}
	castObj(t, e, bolt) // helper: choose the cast option for this object, answer its target with the first option
	if e.spellsCastThisTurn(0) != 1 {
		t.Fatalf("spells cast this turn = %d", e.spellsCastThisTurn(0))
	}
	var surged *decision.Option
	for _, o := range castOptions(t, e) {
		if o.Mode == "surged" {
			surged = &o
		}
	}
	if surged == nil {
		t.Fatal("surge not offered after a spell this turn")
	}
	pool := e.G.Players[0].Pool.Total()
	submitChoices(t, e, surged.Index)
	if e.G.Players[0].Pool.Total() != pool-2 {
		t.Fatal("surge cost {1}{R} not what was paid")
	}
}

func TestFlashbackCastsFromTheGraveyardPaysASacrificeAndExiles(t *testing.T) {
	src := "Name:Therapy\nManaCost:B\nTypes:Sorcery\nK:Flashback:Sac<1/Creature>\n" +
		"A:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e, cfg, therapy := newFixtureDeck(t, 25, src)
	bear := putCreature(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: therapy, From: state.ZHand, To: state.ZGraveyard})
	e.Advance()
	var fb *decision.Option
	for _, o := range castOptions(t, e) {
		if o.Mode == "flashback" && o.Obj == therapy {
			fb = &o
		}
	}
	if fb == nil {
		t.Fatal("flashback not offered from the graveyard")
	}
	submitChoices(t, e, fb.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "sacrifice" || d.Options[0].Obj != bear {
		t.Fatalf("sacrifice choice %+v", d)
	}
	submitChoices(t, e, 0)
	if e.G.Obj(bear).Zone != state.ZGraveyard || e.G.Obj(therapy).Zone != state.ZStack || e.G.Obj(therapy).CastFlags&state.FlagFlashback == 0 {
		t.Fatalf("after paying: bear %s therapy %s", e.G.Obj(bear).Zone, e.G.Obj(therapy).Zone)
	}
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(therapy).Zone != state.ZExile {
		t.Fatalf("flashback spell went to %s, want exile", e.G.Obj(therapy).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestDelveExilesFromTheGraveyardToPayGeneric(t *testing.T) {
	src := "Name:Angler\nManaCost:6 B\nTypes:Creature Zombie Fish\nPT:5/5\nK:Delve\nOracle:x\n"
	e, cfg, angler := newFixtureDeck(t, 26, src)
	var gy []state.ObjID
	for i := 0; i < 4; i++ {
		gy = append(gy, addToGraveyard(t, e, 0, "Name:Junk\nManaCost:1\nTypes:Sorcery\nOracle:x\n"))
	}
	addMana(t, e, 0, "BGG")
	opts := castOptions(t, e)
	if len(opts) != 1 {
		t.Fatalf("with delve, {6}{B} is castable off B+2 and four graveyard cards: %+v", opts)
	}
	submitChoices(t, e, opts[0].Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "exile" || d.Min != 0 || d.Max != 4 {
		t.Fatalf("delve choice %+v", d)
	}
	submitChoices(t, e, 0, 1, 2, 3)
	if e.G.Obj(angler).Zone != state.ZStack || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("angler %s pool %d", e.G.Obj(angler).Zone, e.G.Players[0].Pool.Total())
	}
	for _, id := range gy {
		if e.G.Obj(id).Zone != state.ZExile {
			t.Fatal("delved card not exiled")
		}
	}
	replayCheck(t, e, cfg)
	// Without delve fodder the same hand is not castable.
	e2, _, _ := newFixtureDeck(t, 27, src)
	addMana(t, e2, 0, "BGG")
	if len(castOptions(t, e2)) != 0 {
		t.Fatal("castable without enough mana or graveyard")
	}
}
