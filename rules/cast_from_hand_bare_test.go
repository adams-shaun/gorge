// The BARE wasCastFromYourHand filter predicate (task castprov3) — the
// third cast-provenance family, the one the "from anywhere other than your
// hand" carriers print WITHOUT the ByYou suffix — pinned end to end on real
// corpus carriers in both directions:
//
//   - Vega, the Watcher's `T:Mode$ SpellCast | ValidCard$
//     Card.!wasCastFromYourHand | ValidActivatingPlayer$ You` draw trigger:
//     fires for a flashback (non-hand) cast, never for a hand cast.
//   - Otterball Antics' `ConditionDefined$ Self | ConditionPresent$
//     Card.wasCast+!wasCastFromYourHand | ConditionCompare$ EQ1` gate on its
//     own flashback cast: the token enters WITH the +1/+1 counter from the
//     graveyard, without it from the hand.
//   - See the Truth's `SVar:X:Count$wasCastFromYourHand.1.3` branch head: a
//     non-hand cast takes all three dug cards into the hand, a hand cast one.
//   - Bilbo, Thief in the Night's `S:Mode$ ReduceCost | ValidCard$
//     Card.!wasCastFromYourHand | Amount$ 1` static: a non-hand cast is {1}
//     cheaper, a hand cast full price (the post-push re-price pins here).
//   - Mm'menon, the Right Hand's `RestrictValid$ Spell.!wasCastFromYourHand`
//     granted mana ability: the {U} pays only a non-hand cast.
//
// The non-hand casts are real flashback casts (PutOnStack From=ZGraveyard);
// See the Truth has no flashback, so its non-hand leg drives the cast from
// exile directly (the PutOnStack From=ZExile is exactly what a real
// non-hand cast records). No Forge .txt text is committed.

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

const flashbackTherapy = "Name:Therapy\nManaCost:B\nTypes:Sorcery\nK:Flashback:B\n" +
	"A:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"

// handEngineTokens is handEngine with the corpus's token scripts wired in
// (Config.Tokens): Otterball Antics creates a token, so its engine needs
// the corpus registry's Tokens the same way the acceptance fixtures pass
// them.
func handEngineTokens(t *testing.T, hand ...*cards.Card) *Engine {
	t.Helper()
	e := New(Config{Seed: 1, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)},
		Tokens: testutil.CorpusRegistry(t).Tokens})
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	var ids []state.ObjID
	for _, c := range hand {
		o := e.G.AddObject(c, 0)
		o.Zone = state.ZHand
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZHand, 0, ids)
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	return e
}

// placeOnBattlefield moves a hand-dealt object to the battlefield with a
// real MoveZone event (the eventless Zone write would leave the id in the
// hand zone list too, and every zone-walking scan would see it twice), then
// settles whatever the move queues.
func placeOnBattlefield(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	for i := 0; i < 50; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		if len(e.G.Stack) == 0 {
			return
		}
		e.resolveTop()
	}
	t.Fatalf("battlefield move did not settle: %d pending, %d on the stack", len(e.pendingTriggers), len(e.G.Stack))
}

// moveToGraveyard emits the raw hand→graveyard move and settles any queued
// trigger the move makes.
func moveToGraveyard(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	for i := 0; i < 50; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		if len(e.G.Stack) == 0 {
			return
		}
		e.resolveTop()
	}
	t.Fatalf("graveyard move did not settle: %d pending, %d on the stack", len(e.pendingTriggers), len(e.G.Stack))
}

func TestVegaTheWatcherNonHandCastDraws(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Vega, the Watcher"))
	vega := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, vega)
	therapy := e.G.AddObject(card(t, flashbackTherapy), 0)
	therapy.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{therapy.ID})
	e.G.Players[0].Pool[state.MB] = 1
	n0 := len(e.L.Events)
	castMode(t, e, therapy.ID, "flashback")
	drainQueuedTriggers(t, e)
	if got := logDrawsFor(e, n0, 0); got != 1 {
		t.Fatalf("flashback (non-hand) cast drew %d, want Vega's trigger draw of 1 (log draws=%d)", got, logDrawsFor(e, n0, 0))
	}
}

func TestVegaTheWatcherHandCastDrawsNothing(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Vega, the Watcher"))
	vega := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, vega)
	van := e.G.AddObject(card(t, "Name:Van\nManaCost:G\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	van.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), van.ID))
	e.G.Players[0].Pool[state.MG] = 1
	n0 := len(e.L.Events)
	castMode(t, e, van.ID, "")
	finishCast(t, e, van.ID)
	passUntilStackEmpty(t, e, 60)
	if got := logDrawsFor(e, n0, 0); got != 0 {
		t.Fatalf("hand cast drew %d, want 0 (the !wasCastFromYourHand gate must deny)", got)
	}
}

func otterballCounterOnToken(t *testing.T, e *Engine, spell state.ObjID) int32 {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if id == spell {
			continue
		}
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield {
			return o.Counter("P1P1")
		}
	}
	t.Fatal("no otter token on the battlefield")
	return 0
}

func TestOtterballAnticsFlashbackEntersTokenWithCounter(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Otterball Antics"))
	ott := e.G.Zone(state.ZHand, 0)[0]
	moveToGraveyard(t, e, ott)
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = 3, 1
	castMode(t, e, ott, "flashback")
	finishCast(t, e, ott)
	passUntilStackEmpty(t, e, 60)
	if got := otterballCounterOnToken(t, e, ott); got != 1 {
		t.Fatalf("flashback (non-hand) cast's token has %d P1P1, want 1 (the !wasCastFromYourHand gate must hold)", got)
	}
}

func TestOtterballAnticsHandCastEntersTokenWithoutCounter(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Otterball Antics"))
	ott := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = 1, 1
	castMode(t, e, ott, "")
	finishCast(t, e, ott)
	passUntilStackEmpty(t, e, 60)
	if got := otterballCounterOnToken(t, e, ott); got != 0 {
		t.Fatalf("hand cast's token has %d P1P1, want 0 (the !wasCastFromYourHand gate must deny)", got)
	}
}

func TestSeeTheTruthNonHandCastPutsAllThreeIntoHand(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "See the Truth"))
	truth := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: truth, From: state.ZHand, To: state.ZExile})
	handBefore := make(map[state.ObjID]bool)
	for _, id := range e.G.Zone(state.ZHand, 0) {
		handBefore[id] = true
	}
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = 1, 1
	castMode(t, e, truth, "")
	// ChangeNum$ X = 3 for the non-hand cast: the three-card window holds
	// exactly ChangeNum eligible cards, so the strict-supersets rule poses
	// no dig ask and the take is direct.
	finishCast(t, e, truth)
	passUntilStackEmpty(t, e, 60)
	fresh := 0
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if !handBefore[id] {
			fresh++
		}
	}
	if fresh != 3 {
		t.Fatalf("non-hand cast put %d cards into hand, want all 3 (X's false branch)", fresh)
	}
}

func TestSeeTheTruthHandCastPutsOneIntoHand(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "See the Truth"))
	truth := e.G.Zone(state.ZHand, 0)[0]
	handBefore := make(map[state.ObjID]bool)
	for _, id := range e.G.Zone(state.ZHand, 0) {
		handBefore[id] = true
	}
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = 1, 1
	castMode(t, e, truth, "")
	// X = 1 for the hand cast: three eligible, ChangeNum 1, so the dig ask
	// suspends mid-resolution; take the first card.
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.ResumeKind != "dig" {
		t.Fatalf("no dig ask for the hand cast: %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 60)
	fresh := 0
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if !handBefore[id] {
			fresh++
		}
	}
	if fresh != 1 {
		t.Fatalf("hand cast put %d cards into hand, want 1 (X's true branch)", fresh)
	}
}

func TestBilboThiefInTheNightReducesNonHandCast(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Bilbo, Thief in the Night"))
	bilbo := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, bilbo)
	flash := e.G.AddObject(card(t, "Name:Flash\nManaCost:2 B\nTypes:Sorcery\nK:Flashback:2 B\n"+
		"A:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"), 0)
	flash.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{flash.ID})
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 2, 1
	castMode(t, e, flash.ID, "flashback")
	finishCast(t, e, flash.ID)
	passUntilStackEmpty(t, e, 60)
	if got := e.G.Players[0].Pool[state.MC]; got != 1 {
		t.Fatalf("non-hand cast pool shows %d generic, want 1 (the {1} reduction must apply to the flashback's 2)", got)
	}
	if got := e.G.Players[0].Pool[state.MB]; got != 0 {
		t.Fatalf("non-hand cast paid %d black, want 1", got)
	}
}

func TestBilboThiefInTheNightHandCastFullPrice(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Bilbo, Thief in the Night"))
	bilbo := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, bilbo)
	van := e.G.AddObject(card(t, "Name:Van\nManaCost:2 B\nTypes:Sorcery\nOracle:x\n"), 0)
	van.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), van.ID))
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 2, 1
	castMode(t, e, van.ID, "")
	finishCast(t, e, van.ID)
	passUntilStackEmpty(t, e, 60)
	if got := e.G.Players[0].Pool[state.MC]; got != 0 {
		t.Fatalf("hand cast pool shows %d generic, want 0 left of 2 (the reduction must not apply)", got)
	}
	if got := e.G.Players[0].Pool[state.MB]; got != 0 {
		t.Fatalf("hand cast paid %d black, want 1", got)
	}
}

func TestMmmenonManaSpendableOnlyOnNonHandCast(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Mm'menon, the Right Hand"))
	mm := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, mm)
	rock := onBoard(t, e, 0, "Name:Rock\nTypes:Artifact\nOracle:x\n")
	e.priorityRound()
	d := e.Pending()
	rockOption := -1
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == rock {
			rockOption = o.Index
		}
	}
	if rockOption < 0 {
		t.Fatalf("mmmenon did not grant the rock its restricted mana ability: %+v", d.Options)
	}
	submitChoices(t, e, rockOption)
	if got := e.G.Players[0].Pool[state.MU]; got != 1 || len(e.G.Players[0].RestrictedMana) != 1 {
		t.Fatalf("mmmenon mana did not retain its restriction: pool=%+v restrictions=%+v",
			e.G.Players[0].Pool, e.G.Players[0].RestrictedMana)
	}
	// A hand cast: the restriction refuses, so the plain generic pays and
	// the restricted {U} stays.
	van := e.G.AddObject(card(t, "Name:Van\nManaCost:1\nTypes:Sorcery\nOracle:x\n"), 0)
	van.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), van.ID))
	e.G.Players[0].Pool[state.MC] = 1
	castMode(t, e, van.ID, "")
	finishCast(t, e, van.ID)
	passUntilStackEmpty(t, e, 60)
	if got := e.G.Players[0].Pool[state.MU]; got != 1 || len(e.G.Players[0].RestrictedMana) != 1 {
		t.Fatalf("hand cast consumed the restricted mana: pool=%+v restrictions=%+v",
			e.G.Players[0].Pool, e.G.Players[0].RestrictedMana)
	}
	if got := e.G.Players[0].Pool[state.MC]; got != 0 {
		t.Fatalf("hand cast did not pay from the plain pool: pool=%+v", e.G.Players[0].Pool)
	}
	// A non-hand (flashback) cast: the restriction admits, so the restricted
	// {U} pays first and the plain generic survives.
	flash := e.G.AddObject(card(t, "Name:Flash\nManaCost:2 U\nTypes:Sorcery\nK:Flashback:1\nOracle:x\n"), 0)
	flash.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, append(e.G.Zone(state.ZGraveyard, 0), flash.ID))
	castMode(t, e, flash.ID, "flashback")
	finishCast(t, e, flash.ID)
	passUntilStackEmpty(t, e, 60)
	if got := len(e.G.Players[0].RestrictedMana); got != 0 {
		t.Fatalf("non-hand cast did not consume the restricted mana: restrictions=%+v", e.G.Players[0].RestrictedMana)
	}
	if got := e.G.Players[0].Pool[state.MU]; got != 0 || e.G.Players[0].Pool[state.MC] != 0 {
		t.Fatalf("non-hand cast's pool wrong: pool=%+v (the plain {C} was already spent by the hand cast and must stay spent)", e.G.Players[0].Pool)
	}
}
