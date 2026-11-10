// miracle.go / miracle_test.go drive the Miracle keyword (CR 702.93) through
// the trigger queue: the FIRST draw of a turn offers to cast the drawn card
// for its miracle cost, the owner declines or accepts, and a yes reveals the
// card and runs the ordinary cast flow with the miracle cost and FlagMiracle.
package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// moveToLibraryTop moves id into its owner's library so the test's own Draw
// event is the one that finds it. It must be a LOGGED MoveZone, not a raw
// state.SetZone: Move (engine/events/apply.go) decides which zone to remove an
// object from by reading the object's o.Zone field, so a bare SetZone that
// leaves o.Zone stale would desync the field from the zone slice and a
// replay would diverge. Moving hand->library via the same events.Apply path
// the game uses keeps the object and the log in lockstep, so a log-only
// replay (which replays this very MoveZone, then the Draw) ends exactly as
// the live game does. The card does not need to sit at index 0: the tests
// draw it by explicit Obj, so the drawn card is the Miracle one regardless
// of where in the library it lands.
func moveToLibraryTop(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil {
		t.Fatalf("moveToLibraryTop: no such object %d", id)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZLibrary,
		Text: "moved to library top for the miracle test"})
}

// passToMiracleAsk passes priority until the MiracleCast trigger on the stack
// resolves and poses its cast-or-keep ask (ResumeKind "miracle"), which it
// returns. It returns nil when the stack empties without one.
func passToMiracleAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for range 12 {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while waiting for the miracle cast ask")
		}
		if d.Kind == decision.KTriggerOptional && d.ResumeKind == "miracle" {
			return d
		}
		if len(e.G.Stack) == 0 {
			return nil
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if pass < 0 {
			t.Fatalf("waiting for the miracle cast ask: priority decision with no pass: %+v", d)
		}
		submitChoices(t, e, pass)
	}
	t.Fatal("the miracle cast ask never appeared")
	return nil
}

// miracleOption is the index of the option of the given kind in d.
func miracleOption(t *testing.T, d *decision.Decision, kind string) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == kind {
			return o.Index
		}
	}
	t.Fatalf("no %q option in %+v", kind, d.Options)
	return 0
}

// miracleTriggerOnStack asserts the stack holds exactly the MiracleCast
// ability sourced by card, and returns nothing: the precondition every
// "responds to the trigger" assertion depends on.
func miracleTriggerOnStack(t *testing.T, e *Engine, card state.ObjID) {
	t.Helper()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack %v, want exactly the miracle trigger", e.G.Stack)
	}
	top := e.G.Obj(e.G.Stack[0])
	if top.Ability == nil || top.Ability.API != "MiracleCast" || top.Source != card {
		t.Fatalf("stack object %+v is not the MiracleCast ability of %d", top, card)
	}
}

func TestMiracleOffersOnTheFirstDrawOnly(t *testing.T) {
	t.Parallel()
	terminus := "Name:Terminus\nManaCost:4 W W\nTypes:Sorcery\nK:Miracle:W\n" +
		"A:SP$ ChangeZoneAll | ChangeType$ Creature | Origin$ Battlefield | Destination$ Library | LibraryPosition$ -1\n" +
		"Oracle:x\n"
	bearSrc := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, cfg, term := newFixtureDeck(t, 101, terminus, bearSrc)
	// Put Terminus on top of seat 0's library and give them a creature to wipe.
	moveToLibraryTop(t, e, term)
	bear := putCreature(t, e, 0, bearSrc)
	addMana(t, e, 0, "W")
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: term, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if len(e.pendingTriggers) != 1 || !e.pendingTriggers[0].Miracle {
		t.Fatalf("no miracle offer: %+v", e.pendingTriggers)
	}
	// The draw queued the offer; the engine must now hand it out as the next
	// decision. addMana's priorityRound left a priority decision pending, so
	// clear it (an ordinary test-arrangement idiom, as cast_test/choose_test
	// do) and let Advance reach the offer.
	e.pending = nil
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional || d.Player != 0 {
		t.Fatalf("offer decision %+v", d)
	}
	submitChoices(t, e, 0) // reveal: yes
	miracleTriggerOnStack(t, e, term)
	ask := passToMiracleAsk(t, e)
	if ask == nil {
		t.Fatal("no miracle cast ask after the trigger resolved")
	}
	submitChoices(t, e, miracleOption(t, ask, "yes"))
	if e.G.Obj(term).Zone != state.ZStack || e.G.Obj(term).CastFlags&state.FlagMiracle == 0 || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("terminus %s flags %d pool %d", e.G.Obj(term).Zone, e.G.Obj(term).CastFlags, e.G.Players[0].Pool.Total())
	}
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(bear).Zone != state.ZLibrary {
		t.Fatal("Terminus did not resolve")
	}
	replayCheck(t, e, cfg)

	// A second Miracle card drawn the same turn gets no offer.
	e2, _, t2 := newFixtureDeck(t, 102, terminus)
	moveToLibraryTop(t, e2, t2)
	e2.emit(events.Event{Kind: events.Draw, Player: 0, Obj: e2.G.Zone(state.ZLibrary, 0)[1], From: state.ZLibrary, To: state.ZHand, Secret: true})
	e2.pendingTriggers = nil
	e2.emit(events.Event{Kind: events.Draw, Player: 0, Obj: t2, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if len(e2.pendingTriggers) != 0 {
		t.Fatal("miracle offered on a second draw")
	}
}

func TestMiracleWithXAsksX(t *testing.T) {
	t.Parallel()
	// The REAL repo-deck card, from the corpus (see
	// entreat_the_angels_test.go for why a name-sharing fixture would defend
	// nothing): entreatCorpusCard asserts the compiled script carries
	// K:Miracle:X W W and the TokenAmount$ X body, and the engine's token
	// registry is the corpus's own, so the minted angels are the real
	// w_4_4_angel_flying token.
	e, cfg, en, entreatCard := entreatCorpusEngine(t, 103)
	if e.G.Obj(en).Card != entreatCard {
		t.Fatal("the miracle-test object is not the corpus Entreat the Angels card")
	}
	moveToLibraryTop(t, e, en)
	addMana(t, e, 0, "WWWW")
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: en, From: state.ZLibrary, To: state.ZHand, Secret: true})
	e.pending = nil
	e.Advance()
	if d := e.Pending(); d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("offer decision %+v", d)
	}
	submitChoices(t, e, 0) // reveal: yes
	miracleTriggerOnStack(t, e, en)
	ask := passToMiracleAsk(t, e)
	if ask == nil {
		t.Fatal("no miracle cast ask after the trigger resolved")
	}
	submitChoices(t, e, miracleOption(t, ask, "yes"))
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" || len(d.Options) != 3 { // X = 0,1,2 with WWWW
		t.Fatalf("X after miracle: %+v", d)
	}
	// Miracle cost is {X}{W}{W}: choosing X=2 charges {2}{W}{W} of the 4
	// white tapped in, flags the spell, puts it on the stack with X recorded.
	submitChoices(t, e, 2)
	o := e.G.Obj(en)
	if o.Zone != state.ZStack || o.X != 2 || o.CastFlags&state.FlagMiracle == 0 || o.Card != entreatCard {
		t.Fatalf("after X=2: zone=%s X=%d flags=%d corpusCard=%v", o.Zone, o.X, o.CastFlags, o.Card == entreatCard)
	}
	// The resolved effect creates TokenAmount$ X Angels (xPaid == 2):
	// resolveTop's spell branch binds the CastInfo-recorded X into
	// effects.Ctx.X, so SVar:X:Count$xPaid answers the chosen value.
	passUntilStackEmpty(t, e, 30)
	if e.G.Obj(en).Zone != state.ZGraveyard {
		t.Fatalf("Entreat zone %s after resolving", e.G.Obj(en).Zone)
	}
	angels := 0
	for _, tok := range e.G.Zone(state.ZBattlefield, 0) {
		if f := e.G.Obj(tok).Face(); f != nil && f.Name == "Angel Token" {
			angels++
		}
	}
	if angels != 2 {
		t.Fatalf("Angels on the battlefield = %d, want 2", angels)
	}
	replayCheck(t, e, cfg)
}

// terminusMiracleFixture draws a Terminus (K:Miracle:W) as seat 0's first card
// with W available and a bear to wipe, and answers the reveal ask yes: the
// MiracleCast trigger is then the only thing on the stack.
func terminusMiracleFixture(t *testing.T, seed uint64) (e *Engine, cfg Config, term, bear state.ObjID) {
	t.Helper()
	terminus := "Name:Terminus\nManaCost:4 W W\nTypes:Sorcery\nK:Miracle:W\n" +
		"A:SP$ ChangeZoneAll | ChangeType$ Creature | Origin$ Battlefield | Destination$ Library | LibraryPosition$ -1\n" +
		"Oracle:x\n"
	bearSrc := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, cfg, term = newFixtureDeck(t, seed, terminus, bearSrc)
	moveToLibraryTop(t, e, term)
	bear = putCreature(t, e, 0, bearSrc)
	addMana(t, e, 0, "W")
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: term, From: state.ZLibrary, To: state.ZHand, Secret: true})
	e.pending = nil
	e.Advance()
	if d := e.Pending(); d == nil || d.Kind != decision.KTriggerOptional || d.ResumeKind == "miracle" {
		t.Fatalf("reveal ask %+v", d)
	}
	submitChoices(t, e, 0) // reveal: yes
	return e, cfg, term, bear
}

// CR 702.94a: revealing the card creates a triggered ability on the stack; the
// card is cast only as that trigger resolves, so players may respond to it.
func TestMiracleTriggerResolvesThenCasts(t *testing.T) {
	t.Parallel()
	e, cfg, term, bear := terminusMiracleFixture(t, 201)
	miracleTriggerOnStack(t, e, term)
	if z := e.G.Obj(term).Zone; z != state.ZHand {
		t.Fatalf("Terminus is in %s while the trigger is on the stack, want hand", z)
	}
	revealed := false
	for _, ev := range e.L.Events {
		revealed = revealed || (ev.Kind == events.Note && strings.Contains(ev.Text, "reveals Terminus (miracle)"))
	}
	if !revealed {
		t.Fatal("no (miracle) reveal Note logged")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("with the trigger on the stack players should get priority, got %+v", d)
	}
	ask := passToMiracleAsk(t, e)
	if ask == nil || ask.Player != 0 {
		t.Fatalf("cast ask %+v", ask)
	}
	submitChoices(t, e, miracleOption(t, ask, "yes"))
	o := e.G.Obj(term)
	if o.Zone != state.ZStack || o.CastFlags&state.FlagMiracle == 0 || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("terminus %s flags %d pool %d", o.Zone, o.CastFlags, e.G.Players[0].Pool.Total())
	}
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(bear).Zone != state.ZLibrary {
		t.Fatal("Terminus did not resolve")
	}
	replayCheck(t, e, cfg)
}

// The card leaving the hand while the trigger is on the stack (a response)
// leaves the trigger nothing to cast: no cast ask is posed.
func TestMiracleTriggerCardLeftHandNoCast(t *testing.T) {
	t.Parallel()
	e, _, term, _ := terminusMiracleFixture(t, 202)
	miracleTriggerOnStack(t, e, term)
	e.emit(events.Event{Kind: events.MoveZone, Obj: term, From: state.ZHand, To: state.ZGraveyard, Text: "discarded in response"})
	if z := e.G.Obj(term).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: Terminus in %s, want graveyard", z)
	}
	if ask := passToMiracleAsk(t, e); ask != nil {
		t.Fatalf("a cast ask was posed for a card no longer in hand: %+v", ask)
	}
	if z := e.G.Obj(term).Zone; z != state.ZGraveyard || len(e.G.Stack) != 0 {
		t.Fatalf("Terminus %s, stack %v: want it still in the graveyard, stack empty", z, e.G.Stack)
	}
}

// Declining the cast as the trigger resolves keeps the card in hand.
func TestMiracleCastDeclinedKeepsCardInHand(t *testing.T) {
	t.Parallel()
	e, _, term, _ := terminusMiracleFixture(t, 203)
	miracleTriggerOnStack(t, e, term)
	ask := passToMiracleAsk(t, e)
	if ask == nil {
		t.Fatal("no cast ask")
	}
	if miracleOption(t, ask, "yes") == miracleOption(t, ask, "no") {
		t.Fatal("yes and no are the same option")
	}
	submitChoices(t, e, miracleOption(t, ask, "no"))
	if z := e.G.Obj(term).Zone; z != state.ZHand || len(e.G.Stack) != 0 {
		t.Fatalf("Terminus %s, stack %v: want hand and an empty stack", z, e.G.Stack)
	}
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != e.G.Active {
		t.Fatalf("after the decline the active player should have priority: %+v", d)
	}
}
