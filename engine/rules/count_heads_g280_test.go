package rules

// The cr-g280 count heads: Count$EachSpentToCast.<colour> (Bladecoil Serpent,
// Clay Champion, Dawnglow Infusion), Count$TotalDamageDoneByThisTurn
// (Corrupt and the other "equal to the damage dealt" riders) and the summed
// PlayerCountPlayers$/PlayerCountOpponents$CardsDiscardedThisTurn (The Raven
// Man, Tinybones, Trinket Thief). Every test runs real corpus cards through
// the registry and ends with replayCheck; the preconditions name the data the
// assertion depends on (the captured per-colour spend, the recorded damage,
// the discard the trigger must see), so a head that silently reads zero fails
// at the precondition instead of passing for the wrong reason.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// g280Deck builds a replayable two-seat engine from an explicit seat-0 deck.
// The first len(hand) cards are moved to hand and the next len(board) cards
// onto the battlefield; the rest of the 40-card deck (and seat 1) is inline
// Mountains. It is corpusDeckEngine's shape with the one difference these
// tests need: repeated copies of the same card (three Swamps) are matched
// one-for-one instead of re-emitting a move for every pointer match.
func g280Deck(t *testing.T, reg *cards.Registry, hand, board []*cards.Card) (*Engine, Config, []state.ObjID) {
	t.Helper()
	deck := append(append([]*cards.Card{}, hand...), board...)
	if len(deck) > 40 {
		t.Fatalf("deck of %d cards does not fit the 40-card fixture", len(deck))
	}
	cfg := Config{Seed: 2801, Names: []string{"g280a", "g280b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append(deck, mountainDeck(t, 40-len(deck))...),
			mountainDeck(t, 40),
		}}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	used := make(map[state.ObjID]bool)
	take := func(c *cards.Card, to state.Zone) state.ObjID {
		for _, from := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(from, 0) {
				o := e.G.Obj(id)
				if o == nil || o.Card != c || used[id] {
					continue
				}
				used[id] = true
				if from != to {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: from, To: to})
				}
				return id
			}
		}
		t.Fatalf("card %q not found in seat 0's hand or library", c.Faces[0].Name)
		return 0
	}
	handIDs := make([]state.ObjID, 0, len(hand))
	for _, c := range hand {
		handIDs = append(handIDs, take(c, state.ZHand))
	}
	for _, c := range board {
		take(c, state.ZBattlefield)
	}
	e.pending = nil
	e.Advance()
	return e, cfg, handIDs
}

// g280BattlefieldByName returns seat p's battlefield object named name.
func g280BattlefieldByName(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("%q is not on seat %d's battlefield", name, p)
	return 0
}

// g280PermanentsWithType counts seat p's battlefield permanents carrying the
// printed type word.
func g280PermanentsWithType(e *Engine, p state.PlayerID, typ string) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && slices.Contains(o.Face().Types, typ) {
			n++
		}
	}
	return n
}

// g280DamageDealt is the host's recorded damage-dealt-this-turn total.
func g280DamageDealt(o *state.Object) int32 {
	var n int32
	if o == nil {
		return 0
	}
	for _, rec := range o.DamageDealtThisTurn {
		n += rec.Amount
	}
	return n
}

// g280DiscardsBy counts the log's discards attributed to player p.
func g280DiscardsBy(e *Engine, p state.PlayerID) int {
	n := 0
	for _, ev := range e.L.Events {
		if events.IsDiscard(ev) && ev.Player == p {
			n++
		}
	}
	return n
}

// g280ActivateDiscardAndDrain activates the pending ability, answers the
// opponent's discard ask, and drains priority until the discard has landed
// and the stack is empty.
func g280ActivateDiscardAndDrain(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 60; i++ {
		if g280DiscardsBy(e, 1) > 0 && len(e.G.Stack) == 0 {
			return
		}
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.ResumeKind == "discard" {
			if len(d.Options) == 0 {
				t.Fatalf("discard ask with no option: %+v", d)
			}
			submitChoices(t, e, d.Options[0].Index)
			continue
		}
		if d.Kind == decision.KPriority {
			submitPass(t, e)
			continue
		}
		t.Fatalf("unexpected decision %+v while resolving the discard ability", d)
	}
	t.Fatalf("the discard ability did not discard (discards=%d, stack=%v)", g280DiscardsBy(e, 1), e.G.Stack)
}

// TestEachSpentToCastBladecoilDrawsPerBluePair: with X=4 paid as {U}{U}{U}{U}
// plus six non-blue generic mana, the ETB's Count$EachSpentToCast.U/HalfDown
// is 2, so the blue trigger draws two cards. The captured blue spend is the
// precondition that proves the pay-time vector ran; the black and red
// triggers (each HalfDown of zero) must do nothing.
func TestEachSpentToCastBladecoilDrawsPerBluePair(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	serpent := searchCorpusCard(t, reg, "Bladecoil Serpent")
	e, cfg, ids := g280Deck(t, reg, []*cards.Card{serpent}, nil)
	id := ids[0]
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Bladecoil Serpent not in hand: %+v", e.G.Obj(id))
	}

	addMana(t, e, 0, "UUUUCCCCCC")
	castCardNow(t, e, "Bladecoil Serpent")
	announceXCast(t, e, id, 4)
	o := e.G.Obj(id)
	if o.ManaColorSpent[state.MU] != 4 {
		t.Fatalf("precondition: captured blue spend = %d, want 4 (the pay-time CastInfo did not run)",
			o.ManaColorSpent[state.MU])
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	oppHandBefore := len(e.G.Zone(state.ZHand, 1))

	passUntilStackEmpty(t, e, 40)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Bladecoil Serpent zone = %s, want battlefield", o.Zone)
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - handBefore; got != 2 {
		t.Fatalf("blue trigger drew %d cards, want 2 (4 blue / HalfDown)", got)
	}
	if got := len(e.G.Zone(state.ZHand, 1)) - oppHandBefore; got != 0 {
		t.Fatalf("black trigger discarded %d opponent cards, want 0 (0 black)", -got)
	}
	if e.HasKeyword(id, "Trample") || e.HasKeyword(id, "Haste") {
		t.Fatal("red trigger granted trample/haste despite zero red mana spent")
	}
	replayCheck(t, e, cfg)
}

// TestEachSpentToCastClayChampionCounters: X=2 paid as {G}{G} plus four other
// mana makes Count$EachSpentToCast.G/HalfDown 1, so the etbCounter's GGx3
// gives three +1/+1 counters. The captured green spend is the precondition.
func TestEachSpentToCastClayChampionCounters(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	champion := searchCorpusCard(t, reg, "Clay Champion")
	e, cfg, ids := g280Deck(t, reg, []*cards.Card{champion}, nil)
	id := ids[0]
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Clay Champion not in hand: %+v", e.G.Obj(id))
	}

	addMana(t, e, 0, "GGCCCC")
	castCardNow(t, e, "Clay Champion")
	announceXCast(t, e, id, 2)
	o := e.G.Obj(id)
	if o.ManaColorSpent[state.MG] != 2 {
		t.Fatalf("precondition: captured green spend = %d, want 2 (the pay-time CastInfo did not run)",
			o.ManaColorSpent[state.MG])
	}

	passUntilStackEmpty(t, e, 40)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Clay Champion zone = %s, want battlefield", o.Zone)
	}
	if got := o.Counter("P1P1"); got != 3 {
		t.Fatalf("P1P1 counters = %d, want 3 (green 2 / HalfDown, times 3)", got)
	}
	replayCheck(t, e, cfg)
}

// TestTotalDamageDoneCorruptGainsDamageDealt: with three Swamps, Corrupt deals
// 3 and its "gain life equal to the damage dealt this way" reads
// Count$TotalDamageDoneByThisTurn = 3, so the caster gains 3. The recorded
// damage on the Corrupt object is the precondition.
func TestTotalDamageDoneCorruptGainsDamageDealt(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	corrupt := searchCorpusCard(t, reg, "Corrupt")
	swamp := searchCorpusCard(t, reg, "Swamp")
	e, cfg, ids := g280Deck(t, reg, []*cards.Card{corrupt},
		[]*cards.Card{swamp, swamp, swamp})
	id := ids[0]
	if got := g280PermanentsWithType(e, 0, "Swamp"); got != 3 {
		t.Fatalf("precondition: seat 0 controls %d Swamps, want 3", got)
	}

	addMana(t, e, 0, "BBBBBB")
	castCardNow(t, e, "Corrupt")
	d := passToTargetAsk(t, e)
	tgt := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("no opponent target offered: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 40)

	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("opponent life = %d, want 17 (3 damage)", got)
	}
	if got := e.G.Players[0].Life; got != 23 {
		t.Fatalf("caster life = %d, want 23 (3 damage dealt, 3 gained)", got)
	}
	o := e.G.Obj(id)
	if o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Corrupt zone = %s, want graveyard", o.Zone)
	}
	if got := g280DamageDealt(o); got != 3 {
		t.Fatalf("precondition: Corrupt's recorded damage this turn = %d, want 3", got)
	}
	replayCheck(t, e, cfg)
}

// TestTotalDamageDonePreventedDamageGainsLess: the same Corrupt aimed at a
// Craw Wurm that has a Samite Healer prevention shield for 1. Only 2 damage
// is dealt, so the caster gains 2, not 3 -- the head reads damage actually
// dealt (after prevention), never the printed amount.
func TestTotalDamageDonePreventedDamageGainsLess(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	corrupt := searchCorpusCard(t, reg, "Corrupt")
	swamp := searchCorpusCard(t, reg, "Swamp")
	healer := searchCorpusCard(t, reg, "Samite Healer")
	wurm := searchCorpusCard(t, reg, "Craw Wurm")
	e, cfg, ids := g280Deck(t, reg, []*cards.Card{corrupt},
		[]*cards.Card{swamp, swamp, swamp, healer, wurm})
	id := ids[0]
	if got := g280PermanentsWithType(e, 0, "Swamp"); got != 3 {
		t.Fatalf("precondition: seat 0 controls %d Swamps, want 3", got)
	}

	// Their own next untap step clears summoning sickness through the ordinary
	// logged flow; a direct SummonSick=false would replay differently.
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	healerID := g280BattlefieldByName(t, e, 0, "Samite Healer")
	wurmID := g280BattlefieldByName(t, e, 0, "Craw Wurm")
	if o := e.G.Obj(healerID); o == nil || o.SummonSick {
		t.Fatalf("precondition: Samite Healer is still summoning sick at turn 3")
	}
	e.priorityRound()

	opt, ok := findAbilityOption(e, healerID, 0)
	if !ok {
		t.Fatalf("Samite Healer's prevention ability not offered: %+v", e.Pending())
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("prevention target ask: %+v", d)
	}
	tgt := -1
	for _, o := range d.Options {
		if o.Obj == wurmID {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("Craw Wurm not offered for the prevention shield: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 40)

	lifeBefore := e.G.Players[0].Life
	addMana(t, e, 0, "BBBBBB")
	castCardNow(t, e, "Corrupt")
	d = passToTargetAsk(t, e)
	tgt = -1
	for _, o := range d.Options {
		if o.Obj == wurmID {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("Craw Wurm not offered as a Corrupt target: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 40)

	wo := e.G.Obj(wurmID)
	if wo == nil || wo.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Craw Wurm left the battlefield: %+v", wo)
	}
	if got := wo.DamageReceivedThisTurn; got != 2 {
		t.Fatalf("Craw Wurm took %d damage, want 2 (3 with 1 prevented)", got)
	}
	if got := e.G.Players[0].Life - lifeBefore; got != 2 {
		t.Fatalf("caster gained %d life, want 2 (damage actually dealt, not the printed 3)", got)
	}
	o := e.G.Obj(id)
	if got := g280DamageDealt(o); got != 2 {
		t.Fatalf("precondition: Corrupt's recorded damage this turn = %d, want 2", got)
	}
	replayCheck(t, e, cfg)
}

// TestRavenManSeesAnyPlayersDiscard: the opponent discards during the turn, so
// PlayerCountPlayers$CardsDiscardedThisTurn is 1 at the end step and The Raven
// Man's trigger creates a Bird. With no discard that turn the same trigger
// must not fire -- the second engine is the negative half that keeps the
// positive assertion from passing on a head that always reads 1.
func TestRavenManSeesAnyPlayersDiscard(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	raven := searchCorpusCard(t, reg, "The Raven Man")

	e, cfg, _ := g280Deck(t, reg, nil, []*cards.Card{raven})
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	ravenID := g280BattlefieldByName(t, e, 0, "The Raven Man")
	if o := e.G.Obj(ravenID); o == nil || o.SummonSick {
		t.Fatalf("precondition: The Raven Man is still summoning sick at turn 3")
	}
	if got := tokensNamed(e, 0, "Bird"); got != 0 {
		t.Fatalf("precondition: %d Birds already on the battlefield", got)
	}
	e.priorityRound()
	addMana(t, e, 0, "BBBB")
	discardsBefore := g280DiscardsBy(e, 1)
	opt, ok := findAbilityOption(e, ravenID, 0)
	if !ok {
		t.Fatalf("The Raven Man's discard ability not offered: %+v", e.Pending())
	}
	submitChoices(t, e, opt.Index)
	g280ActivateDiscardAndDrain(t, e)
	if got := g280DiscardsBy(e, 1) - discardsBefore; got != 1 {
		t.Fatalf("precondition: the ability made the opponent discard %d cards, want 1", got)
	}

	driveToStepAll(t, e, 3, 0, state.StepEnd)
	passUntilStackEmpty(t, e, 40)
	if got := tokensNamed(e, 0, "Bird"); got != 1 {
		t.Fatalf("The Raven Man created %d Birds after a discard, want 1", got)
	}
	replayCheck(t, e, cfg)

	// Negative half: same board and turn, no discard during this turn.
	e2, cfg2, _ := g280Deck(t, reg, nil, []*cards.Card{raven})
	driveToStepAll(t, e2, 3, 0, state.StepMain1)
	mark := len(e2.L.Events)
	driveToStepAll(t, e2, 3, 0, state.StepEnd)
	passUntilStackEmpty(t, e2, 40)
	thisTurn := 0
	for _, ev := range e2.L.Events[mark:] {
		if events.IsDiscard(ev) && ev.Player == 1 {
			thisTurn++
		}
	}
	if thisTurn != 0 {
		t.Fatalf("precondition: the no-discard game discarded %d opponent cards this turn", thisTurn)
	}
	if got := tokensNamed(e2, 0, "Bird"); got != 0 {
		t.Fatalf("The Raven Man created %d Birds with no discard, want 0", got)
	}
	replayCheck(t, e2, cfg2)
}
