// ValidSAonCard$ — the card-scoped trigger clause on the cast-family trigger
// modes (task validsaoncard1).
//
// Forge's TriggerSpellAbilityCastOrCopy evaluates ValidSAonCard$ against the
// triggering spell's / activated ability's OWN card, with the ACTIVATING
// player as the reference "You" — a different reference point from ValidSA$,
// whose "You" is the trigger source's controller. The reference distinction
// is what makes Avalanche of Sector 7's pair satisfiable: ValidSA$
// Activated.OppCtrl (the activator is an opponent of the trigger source) and
// ValidSAonCard$ Activated.YouCtrl (the activator is the ability's own card's
// controller) hold together on "an opponent activates an ability of an
// artifact they control".
//
// The end-to-end pins run on the REAL corpus carriers (Avalanche of Sector 7,
// Blazing Bomb, Tokka & Rahzar, Dragonlord Kolaghan, Gonti, Night Minister)
// through the shared .cards registry. The discriminator leaf is authored,
// because a real carrier whose ValidSAonCard$ rides a ValidSA$ twin cannot
// distinguish a read clause from an absent one on this arm (see the
// card-relative reference note on the authored fixture).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// saonEngine builds a two-seat engine whose hands/boards are exactly the
// given cards, with the corpus registry's token scripts in Config.Tokens so a
// corpus token body (Gonti's Treasure Token) can mint -- counterHands does
// not carry Tokens, and e.G.Tokens = cfg.Tokens is read at New. Cards are
// added post-genesis, so these games are not log-replayable and no test here
// calls replayCheck (the same shape the handEngine/counterHands users run).
func saonEngine(t *testing.T, hand0, hand1, board0, board1 []*cards.Card) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e := New(Config{Seed: 1, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)},
		Tokens: reg.Tokens})
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
		add := func(h []*cards.Card, z state.Zone) {
			var ids []state.ObjID
			for _, c := range h {
				o := e.G.AddObject(c, p)
				o.Zone = z
				ids = append(ids, o.ID)
			}
			e.G.SetZone(z, p, append(e.G.Zone(z, p), ids...))
		}
		if p == 0 {
			add(hand0, state.ZHand)
			add(board0, state.ZBattlefield)
		} else {
			add(hand1, state.ZHand)
			add(board1, state.ZBattlefield)
		}
	}
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	return e
}

// saonGear is the activated-ability vehicle: an artifact whose one ability
// draws a card for {T} -- cheap, target-less, and not a mana ability, so it
// reaches abilityCastMatches as an ordinary AbilityPush.
const saonGear = "Name:Test Saon Gear\nManaCost:2\nTypes:Artifact\n" +
	"A:AB$ Draw | Cost$ T | NumCards$ 1\nOracle:x\n"

// saonWatcherCardRelative is the authored DISCRIMINATOR: an AbilityCast
// trigger whose ONLY narrowing is the card-relative clause. An activator is
// always the controller of the card it activates on (this build never lets a
// player activate another permanent's ability), so a card-relative OppCtrl
// reference never holds and the trigger must fire on NOTHING; an unread
// clause leaves the trigger ungated and it fires on every activation. That
// is the observable difference the read makes on this arm.
const saonWatcherCardRelative = "Name:Test Card Relative Watcher\nManaCost:2\nTypes:Artifact\n" +
	"T:Mode$ AbilityCast | ValidSAonCard$ Activated.OppCtrl | TriggerZones$ Battlefield | Execute$ TrigDmg | TriggerDescription$ x\n" +
	"SVar:TrigDmg:DB$ DealDamage | NumDmg$ 1 | Defined$ TriggeredActivator\n" +
	"Oracle:x\n"

// saonWatcherSelfRelative is the positive twin: the card-relative YouCtrl
// reference holds on every activation of one's own card, so the trigger
// fires (and would also fire on an unread clause -- this leaf documents the
// clause's real semantics, the discriminator above is what proves the read).
const saonWatcherSelfRelative = "Name:Test Self Relative Watcher\nManaCost:2\nTypes:Artifact\n" +
	"T:Mode$ AbilityCast | ValidSAonCard$ Activated.YouCtrl | TriggerZones$ Battlefield | Execute$ TrigDmg | TriggerDescription$ x\n" +
	"SVar:TrigDmg:DB$ DealDamage | NumDmg$ 1 | Defined$ TriggeredActivator\n" +
	"Oracle:x\n"

// saonInstant is a vanilla instant of the given generic cost and name: the
// cast vehicle the ManaSpent and YouDontOwn clauses observe. An instant, so
// the OPPONENT's casts in the seat-1 scenarios are legal on the active
// player's turn (CR 116.1a).
func saonInstant(cost, name string) string {
	return "Name:" + name + "\nManaCost:" + cost + "\nTypes:Instant\nOracle:x\n"
}

// saonCreature is a vanilla creature of the given cost and name, with Flash
// so the opponent's seat-1 casts are legal on the active player's turn
// (CR 116.1a): the cast vehicle the sharesNameWith clause observes.
func saonCreature(cost, name string) string {
	return "Name:" + name + "\nManaCost:" + cost + "\nTypes:Creature Bear\nPT:2/2\nK:Flash\nOracle:x\n"
}

// saonDelveAngler is the delve vehicle: a {5}{B} creature with Flash and
// Delve whose full mana value 6 exceeds the {B}{G} the test funds, so the
// cast's pool spend (2) is strictly below its mana value exactly as a real
// Gurmag Angler delve cast behaves (the real card carries no Flash and is
// not castable on the active player's turn).
const saonDelveAngler = "Name:Test Saon Angler\nManaCost:5 B\nTypes:Creature Zombie Fish\nPT:5/5\nK:Delve\nK:Flash\nOracle:x\n"

// saonSettle drives the stack to empty: every queued trigger is drained onto
// the stack (priorityRound's own drain), every priority ask is passed, and
// the loop STOPS once the stack is empty and no trigger is outstanding -- it
// never drives the turn's next step, so no declare-attackers ask is reached.
// An unexpected decision kind is a test bug.
func saonSettle(t *testing.T, e *Engine, limit int) {
	t.Helper()
	settled := func() bool {
		return len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0
	}
	for i := 0; i < limit; i++ {
		if e.pending == nil {
			if settled() {
				return
			}
			e.priorityRound()
			continue
		}
		d := e.Pending()
		switch d.Kind {
		case decision.KPriority:
			if settled() {
				return
			}
			passFirst(t, e)
		case decision.KTriggerOptional:
			submitChoices(t, e, 0)
		default:
			t.Fatalf("unexpected decision %q while settling: %+v", d.Kind, d.Options)
		}
	}
	t.Fatalf("saonSettle never settled: %d pending triggers, stack %d", len(e.pendingTriggers), len(e.G.Stack))
}

// saonActivateAbility gives seat p priority and submits its ability option
// (the one ability its board holds).
func saonActivateAbility(t *testing.T, e *Engine, p state.PlayerID) {
	t.Helper()
	e.G.Priority = p
	e.askPriority(p)
	d := e.Pending()
	if d == nil || d.Player != p {
		t.Fatalf("expected seat %d's priority, got %+v", p, d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "ability" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("seat %d has no ability option: %+v", p, d.Options)
	}
	submitChoices(t, e, idx)
}

// saonCast gives seat p priority and submits the cast option for obj.
func saonCast(t *testing.T, e *Engine, p state.PlayerID, obj state.ObjID) {
	t.Helper()
	e.G.Priority = p
	e.askPriority(p)
	d := e.Pending()
	if d == nil || d.Player != p {
		t.Fatalf("expected seat %d's priority, got %+v", p, d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == obj {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("seat %d has no cast option for %d: %+v", p, obj, d.Options)
	}
	submitChoices(t, e, idx)
}

// saonCounterN reads one counter kind off an object's counter list.
func saonCounterN(o *state.Object, kind string) int32 {
	for _, c := range o.Counters {
		if c.Kind == kind {
			return c.N
		}
	}
	return 0
}

// saonTreasureTokens counts seat p's battlefield Treasure tokens (the
// corpus script's face name is "Treasure Token").
func saonTreasureTokens(t *testing.T, e *Engine, p state.PlayerID) int {
	t.Helper()
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Treasure Token" {
			n++
		}
	}
	return n
}

// TestAvalancheOfSector7OpponentArtifactAbilityDealsDamage pins the REAL
// corpus carrier end to end: an opponent activating an ability of an artifact
// THEY control deals them 1 damage (ValidSA$ Activated.OppCtrl + ValidSAonCard$
// Activated.YouCtrl on the same line), and the trigger source's own activation
// fires nothing.
func TestAvalancheOfSector7OpponentArtifactAbilityDealsDamage(t *testing.T) {
	av := corpusAlternativeCard(t, "Avalanche of Sector 7")
	gear0, gear1 := card(t, saonGear), card(t, saonGear)
	e := saonEngine(t, nil, nil, []*cards.Card{av, gear0}, []*cards.Card{gear1})
	avID := e.G.Zone(state.ZBattlefield, 0)[0]
	if e.G.Obj(avID).Face().Name != "Avalanche of Sector 7" {
		t.Fatalf("test precondition: seat 0's board must hold the carrier, got %+v", e.G.Obj(avID).Face())
	}
	gear1ID := e.G.Zone(state.ZBattlefield, 1)[0]
	if gear1ID == 0 || e.G.Obj(gear1ID).Face().Name != "Test Saon Gear" {
		t.Fatalf("test precondition: seat 1's board must hold the gear")
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("test precondition: seat 1 life = %d, want 20", life)
	}

	// The trigger source's own controller activates its own gear: OppCtrl
	// fails, nothing fires.
	saonActivateAbility(t, e, 0)
	saonSettle(t, e, 60)
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("seat 0's own activation dealt %d damage, want none", 20-life)
	}

	// Seat 1 activates its own gear's ability: 1 damage to seat 1, plus the
	// gear's own draw.
	saonActivateAbility(t, e, 1)
	saonSettle(t, e, 60)
	if life := e.G.Players[1].Life; life != 19 {
		t.Fatalf("after the opponent's artifact ability seat 1 life = %d, want 19", life)
	}
	if got := len(e.G.Zone(state.ZHand, 1)); got != 1 {
		t.Fatalf("seat 1 hand = %d, want 1 (the gear's own draw)", got)
	}
}

// TestAbilityCastValidSAonCardCardRelativeOppCtrlNeverFiresWide is the
// discriminator: with the card-relative clause READ, an opponent's activation
// (activator == the ability card's controller) can never satisfy a
// card-relative OppCtrl reference, so the trigger fires on nothing. Without
// the read the clause is absent and the trigger fires on every activation.
func TestAbilityCastValidSAonCardCardRelativeOppCtrlNeverFiresWide(t *testing.T) {
	e := saonEngine(t, nil, nil, []*cards.Card{card(t, saonWatcherCardRelative)}, []*cards.Card{card(t, saonGear)})
	watcher := e.G.Zone(state.ZBattlefield, 0)[0]
	if e.G.Obj(watcher).Face().Name != "Test Card Relative Watcher" {
		t.Fatalf("test precondition: watcher not on seat 0's board")
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("test precondition: seat 1 life = %d, want 20", life)
	}
	before := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage {
			before++
		}
	}

	// Seat 1 activates its own gear: the AbilityPush happens (the activation
	// is real), but the card-relative OppCtrl reference never matches an
	// activation of one's own card.
	saonActivateAbility(t, e, 1)
	saonSettle(t, e, 60)
	if got := len(e.G.Zone(state.ZHand, 1)); got != 1 {
		t.Fatalf("test precondition: the gear's activation never ran (hand %d, want 1)", got)
	}
	after := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage {
			after++
		}
	}
	if after != before {
		t.Fatalf("card-relative OppCtrl fired %d damage events on an own-card activation, want none", after-before)
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("seat 1 life = %d, want 20 (no trigger fire)", life)
	}
}

// TestAbilityCastValidSAonCardYouCtrlFiresOnOwnActivation is the positive
// twin: the card-relative YouCtrl reference holds on every activation of one's
// own card, so the same activation fires the trigger.
func TestAbilityCastValidSAonCardYouCtrlFiresOnOwnActivation(t *testing.T) {
	e := saonEngine(t, nil, nil, []*cards.Card{card(t, saonWatcherSelfRelative)}, []*cards.Card{card(t, saonGear)})
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("test precondition: seat 1 life = %d, want 20", life)
	}
	saonActivateAbility(t, e, 1)
	saonSettle(t, e, 60)
	if life := e.G.Players[1].Life; life != 19 {
		t.Fatalf("card-relative YouCtrl did not fire on the activator's own ability: life %d, want 19", life)
	}
}

// TestBlazingBombValidSAonCardManaSpentGate pins the real carrier's
// Spell.ManaSpent GE4 clause: a noncreature spell costing at least four mana
// of actual spend puts the +1/+1 counter, a cheaper cast does not.
func TestBlazingBombValidSAonCardManaSpentGate(t *testing.T) {
	bomb := corpusAlternativeCard(t, "Blazing Bomb")
	e := saonEngine(t,
		[]*cards.Card{card(t, saonInstant("4", "Test Four Ritual")), card(t, saonInstant("1", "Test One Ritual"))},
		nil,
		[]*cards.Card{bomb}, nil)
	bombID := e.G.Zone(state.ZBattlefield, 0)[0]
	if e.G.Obj(bombID).Face().Name != "Blazing Bomb" {
		t.Fatalf("test precondition: carrier not on seat 0's board")
	}
	if n := saonCounterN(e.G.Obj(bombID), "P1P1"); n != 0 {
		t.Fatalf("test precondition: carrier already carries %d P1P1", n)
	}
	e.G.Players[0].Pool[state.MC] = 10

	// A {4} cast spends exactly 4: the gate holds, the counter lands.
	saonCast(t, e, 0, e.G.Zone(state.ZHand, 0)[0])
	saonSettle(t, e, 60)
	if n := saonCounterN(e.G.Obj(bombID), "P1P1"); n != 1 {
		t.Fatalf("the 4-mana cast left %d P1P1 on the carrier, want 1", n)
	}

	// A {1} cast spends 1: below the gate, no second counter.
	one := handObj(t, e, 0, "Test One Ritual")
	saonCast(t, e, 0, one)
	saonSettle(t, e, 60)
	if n := saonCounterN(e.G.Obj(bombID), "P1P1"); n != 1 {
		t.Fatalf("the 1-mana cast moved the carrier to %d P1P1, want still 1 (below the gate)", n)
	}
}

// TestTokkaRahzarValidSAonCardManaSpentLTXDelve pins the real carrier's
// Spell.ManaSpent LTX clause: a DELVE cast pays part of its generic from the
// graveyard, so the pool spend is strictly below the spell's mana value and
// the trigger deals its 3 to the caster; a full-price cast of the same shape
// fires nothing.
func TestTokkaRahzarValidSAonCardManaSpentLTXDelve(t *testing.T) {
	tokka := corpusAlternativeCard(t, "Tokka & Rahzar, Terrible Twos")
	junk := saonInstant("1", "Test Saon Junk")
	e := saonEngine(t, nil,
		[]*cards.Card{card(t, saonDelveAngler), card(t, junk), card(t, junk), card(t, junk), card(t, junk), card(t, junk)},
		[]*cards.Card{tokka}, nil)
	if e.G.Obj(e.G.Zone(state.ZBattlefield, 0)[0]).Face().Name != "Tokka & Rahzar, Terrible Twos" {
		t.Fatalf("test precondition: carrier not on seat 0's board")
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("test precondition: seat 1 life = %d, want 20", life)
	}
	// Delve fodder: four of the five {1} instants into the graveyard; the
	// fifth stays in hand as the control leaf's full-price cast.
	anglerID := handObj(t, e, 1, "Test Saon Angler")
	for i := 0; i < 4; i++ {
		moveToGraveyard(t, e, handObj(t, e, 1, "Test Saon Junk"))
		if got := len(e.G.Zone(state.ZGraveyard, 1)); got != i+1 {
			t.Fatalf("test precondition: delve fodder after move %d = %d", i, got)
		}
	}
	if got := len(e.G.Zone(state.ZHand, 1)); got != 2 {
		t.Fatalf("test precondition: seat 1 hand after fodder = %d, want 2 (Angler + one control junk)", got)
	}
	// {5}{B} off {B}{G} plus four exiled cards: spent 2, mana value 6.
	e.G.Players[1].Pool[state.MB] = 1
	e.G.Players[1].Pool[state.MG] = 1
	saonCast(t, e, 1, anglerID)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "exile" {
		t.Fatalf("test precondition: delve did not ask an exile decision: %+v", d)
	}
	submitChoices(t, e, 0, 1, 2, 3)
	if o := e.G.Obj(anglerID); o == nil || o.Zone != state.ZStack {
		t.Fatalf("test precondition: Angler did not reach the stack: %+v", o)
	}
	saonSettle(t, e, 60)
	if life := e.G.Players[1].Life; life != 17 {
		t.Fatalf("after the delve cast (spent 2 of mana value 6) seat 1 life = %d, want 17 (the LTX trigger's 3)", life)
	}

	// Control: a full-price {1} cast spends exactly its mana value -- the
	// clause does not hold, no second trigger. The junk cards remaining in
	// hand are all named alike; the one on top of the hand is fine.
	e.G.Players[1].Pool[state.MC] = 2
	saonCast(t, e, 1, e.G.Zone(state.ZHand, 1)[0])
	saonSettle(t, e, 60)
	if life := e.G.Players[1].Life; life != 17 {
		t.Fatalf("the full-price cast fired the LTX clause: seat 1 life = %d, want 17", life)
	}
}

// TestDragonlordKolaghanSharesNameWithTheirGraveyard pins the real carrier's
// two-field clause: an opponent casting a creature whose name is in THEIR OWN
// graveyard loses 10; the same-name card in the TRIGGER CONTROLLER's graveyard
// does not satisfy the clause.
func TestDragonlordKolaghanSharesNameWithTheirGraveyard(t *testing.T) {
	kolaghan := corpusAlternativeCard(t, "Dragonlord Kolaghan")
	bears := saonCreature("1 G", "Test Kolaghan Bears")
	elf := saonCreature("1 G", "Test Kolaghan Elf")
	e := saonEngine(t, nil,
		[]*cards.Card{card(t, bears), card(t, bears), card(t, elf)},
		[]*cards.Card{kolaghan}, nil)
	if e.G.Obj(e.G.Zone(state.ZBattlefield, 0)[0]).Face().Name != "Dragonlord Kolaghan" {
		t.Fatalf("test precondition: carrier not on seat 0's board")
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("test precondition: seat 1 life = %d, want 20", life)
	}
	// One Bears copy into seat 1's OWN graveyard.
	moveToGraveyard(t, e, handObj(t, e, 1, "Test Kolaghan Bears"))
	if got := len(e.G.Zone(state.ZGraveyard, 1)); got != 1 {
		t.Fatalf("test precondition: seat 1 graveyard = %d, want 1", got)
	}
	e.G.Players[1].Pool[state.MG] = 2
	e.G.Players[1].Pool[state.MC] = 2

	// Cast the second Bears: same name as the graveyard card, the activator's
	// own graveyard -- 10 life.
	bearsID := handObj(t, e, 1, "Test Kolaghan Bears")
	saonCast(t, e, 1, bearsID)
	saonSettle(t, e, 60)
	if life := e.G.Players[1].Life; life != 10 {
		t.Fatalf("after casting a creature sharing a name with the caster's own graveyard card, seat 1 life = %d, want 10", life)
	}

	// Control: the Elf's name is in NO graveyard (its twin sits in seat 1's
	// hand) -- no fire.
	saonCast(t, e, 1, handObj(t, e, 1, "Test Kolaghan Elf"))
	saonSettle(t, e, 60)
	if life := e.G.Players[1].Life; life != 10 {
		t.Fatalf("the no-name-share cast fired the clause: seat 1 life = %d, want 10", life)
	}
}

// TestGontiNightMinisterSpellYouDontOwn pins the real carrier's single-field
// clause through the YouDontOwn predicate: a player casting a spell they do
// not OWN creates their Treasure; casting their own spell does not.
func TestGontiNightMinisterSpellYouDontOwn(t *testing.T) {
	gonti := corpusAlternativeCard(t, "Gonti, Night Minister")
	e := saonEngine(t, nil, nil, []*cards.Card{gonti}, nil)
	if e.G.Obj(e.G.Zone(state.ZBattlefield, 0)[0]).Face().Name != "Gonti, Night Minister" {
		t.Fatalf("test precondition: carrier not on seat 0's board")
	}
	if n := saonTreasureTokens(t, e, 1); n != 0 {
		t.Fatalf("test precondition: seat 1 already holds %d Treasures", n)
	}
	// A spell in seat 1's hand OWNED by seat 0 (counterHands assigned owner 1;
	// the ownership is what the clause reads).
	fObj := e.G.AddObject(card(t, saonInstant("2", "Test Foreign Ritual")), 0)
	fObj.Zone = state.ZHand
	foreign := fObj.ID
	e.G.SetZone(state.ZHand, 1, append(e.G.Zone(state.ZHand, 1), foreign))
	oObj := e.G.AddObject(card(t, saonInstant("2", "Test Own Ritual")), 1)
	oObj.Zone = state.ZHand
	own := oObj.ID
	e.G.SetZone(state.ZHand, 1, append(e.G.Zone(state.ZHand, 1), own))
	if e.G.Obj(foreign).Owner != 0 {
		t.Fatalf("test precondition: the foreign spell's owner = %d, want 0", e.G.Obj(foreign).Owner)
	}
	e.G.Players[1].Pool[state.MC] = 10

	// Cast the foreign spell: the activator does not own it -- Treasure.
	saonCast(t, e, 1, foreign)
	saonSettle(t, e, 60)
	if n := saonTreasureTokens(t, e, 1); n != 1 {
		t.Fatalf("casting a spell seat 1 does not own created %d Treasures, want 1", n)
	}

	// Control: seat 1's own spell -- owned by the caster, no fire.
	saonCast(t, e, 1, own)
	saonSettle(t, e, 60)
	if n := saonTreasureTokens(t, e, 1); n != 1 {
		t.Fatalf("casting an owned spell created another Treasure, want none (total %d)", n)
	}
}
