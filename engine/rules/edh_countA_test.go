package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// The edh-countA class behaviour tests: the count heads this ticket added
// to the evaluator, played through a real game with legal actions and
// asserted on state. Each fails with the evaluator arm reverted.

// TestEDHCountHeadsANotoriousThrong: Notorious Throng's X is "the damage
// dealt to your opponents this turn" (Count$BloodthirstAmount): three Bolt
// damage on the opponent, then a resolved Throng creates three Faerie
// Rogue tokens.
func TestEDHCountHeadsANotoriousThrong(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 9421, []string{"Lightning Bolt", "Notorious Throng"}, nil, nil)
	bolt := findCardObj(t, e, 0, "Lightning Bolt", state.ZHand)
	addMana(t, e, 0, "RR")
	submitChoices(t, e, castOptionFor(t, e, bolt).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target ask for the Bolt: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no opponent target offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("precondition: opponent life = %d, want 17 (the Bolt landed)", got)
	}
	throng := findCardObj(t, e, 0, "Notorious Throng", state.ZHand)
	addMana(t, e, 0, "UUUU")
	castObj(t, e, throng)
	if n := battlefieldTokensNamed(t, e, "Faerie Rogue Token"); n != 3 {
		t.Fatalf("Faerie Rogue tokens = %d, want 3 (X = the 3 damage dealt to the opponent this turn)", n)
	}
	replayCheck(t, e, cfg)
}

// TestEDHCountHeadsAMadnessZombies: From Under the Floorboards' token count
// is Count$Madness.X.3 -- three tapped 2/2 Zombies and three life on a
// plain cast (the fallback branch), X of each when the madness cost was
// paid (the madness branch, here X = 2 of the {X}{B}{B} madness cost).
func TestEDHCountHeadsAMadnessZombies(t *testing.T) {
	t.Parallel()
	t.Run("plain cast", func(t *testing.T) {
		t.Parallel()
		e, cfg, _ := altCostEngine(t, 9422, []string{"From Under the Floorboards"}, nil, nil)
		card := findCardObj(t, e, 0, "From Under the Floorboards", state.ZHand)
		addMana(t, e, 0, "BBBBBB")
		castObj(t, e, card)
		if n := battlefieldTokensNamed(t, e, "Zombie Token"); n != 3 {
			t.Fatalf("Zombie tokens = %d, want 3 (the fallback branch)", n)
		}
		if got := e.G.Players[0].Life; got != 23 {
			t.Fatalf("life = %d, want 23 (the fallback branch gained 3)", got)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("madness cast", func(t *testing.T) {
		t.Parallel()
		e, cfg, _ := altCostEngine(t, 9423, []string{"Tormenting Voice", "From Under the Floorboards"}, nil, nil)
		voice := findCardObj(t, e, 0, "Tormenting Voice", state.ZHand)
		card := findCardObj(t, e, 0, "From Under the Floorboards", state.ZHand)
		addMana(t, e, 0, "RRBBBBB")
		submitChoices(t, e, castOptionFor(t, e, voice).Index)
		// The discard cost asks for a card; discard the madness card.
		d := e.Pending()
		if d == nil {
			t.Fatal("no discard ask pending")
		}
		idx := -1
		for _, o := range d.Options {
			if o.Obj == card {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("the madness card is not offered for the discard: %+v", d.Options)
		}
		submitChoices(t, e, idx)
		// Drive to the madness trigger's cast-or-not ask without draining
		// past it.
		d = nil
		for i := 0; i < 16; i++ {
			d = e.Pending()
			if d == nil {
				t.Fatal("no decision while waiting for the madness ask")
			}
			if d.Kind == decision.KTriggerOptional && d.Source == card {
				break
			}
			if d.Kind == decision.KPriority {
				passOnceP(t, e)
				continue
			}
			submitChoices(t, e, 0)
		}
		if z := e.G.Obj(card).Zone; z != state.ZExile {
			t.Fatalf("precondition: the discarded madness card is in %v, want exile", z)
		}
		if d == nil || d.Kind != decision.KTriggerOptional || d.Source != card {
			t.Fatalf("no madness cast ask: %+v", d)
		}
		submitChoices(t, e, optionIndexForKind(d, "yes"))
		answered := false
		for i := 0; i < 12; i++ {
			d = e.Pending()
			if d == nil || d.Kind == decision.KPriority {
				break
			}
			if d.Kind == decision.KChoose && d.Prompt == "Choose a value for X" {
				for _, o := range d.Options {
					if o.Kind == "x" && o.Amount == 2 {
						submitChoices(t, e, o.Index)
						answered = true
						break
					}
				}
				continue
			}
			submitChoices(t, e, 0)
		}
		if !answered {
			t.Fatalf("the madness cast never announced X = 2 (last pending %+v)", e.Pending())
		}
		passUntilStackEmpty(t, e, 30)
		if n := battlefieldTokensNamed(t, e, "Zombie Token"); n != 2 {
			t.Fatalf("Zombie tokens = %d, want 2 (X = the madness cast's paid X)", n)
		}
		if got := e.G.Players[0].Life; got != 22 {
			t.Fatalf("life = %d, want 22 (the madness branch gained X = 2)", got)
		}
		replayCheck(t, e, cfg)
	})
}

// TestEDHCountHeadsASarevoksTome: Sarevok's Tome's mana ability adds {C},
// or {C}{C} when you have the initiative (Count$Initiative.2.1). The read
// is true only for the initiative holder: with the tome's ETB "you take
// the initiative" trigger countered the ability adds one {C}, with it
// resolved two.
func TestEDHCountHeadsASarevoksTome(t *testing.T) {
	t.Parallel()
	t.Run("holder", func(t *testing.T) {
		t.Parallel()
		e, cfg, _ := altCostEngine(t, 9424, []string{"Sarevok's Tome"}, nil, nil)
		tome := castTome(t, e)
		// Resolve the ETB trigger (and the venture into Undercity it
		// implies), declining the venture room's library search.
		drainStackDecliningSearch(t, e)
		if !e.G.HasInitiative || e.G.Initiative != 0 {
			t.Fatalf("precondition: seat 0 does not hold the initiative (%v %v)", e.G.HasInitiative, e.G.Initiative)
		}
		before := e.G.Players[0].Pool.Total()
		activateMana(t, e, tome)
		if got := e.G.Players[0].Pool.Total() - before; got != 2 {
			t.Fatalf("pool gain = %d, want 2: the initiative holder's read adds two {C}", got)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("not the holder", func(t *testing.T) {
		t.Parallel()
		e, cfg := tokenReplGameSeats(t, 9426,
			[]*cards.Card{corpusAlternativeCard(t, "Sarevok's Tome")},
			[]*cards.Card{corpusAlternativeCard(t, "Loot Dispute")})
		tome := castTome(t, e)
		drainStackDecliningSearch(t, e)
		if !e.G.HasInitiative || e.G.Initiative != 0 {
			t.Fatalf("precondition: seat 0 does not hold the initiative (%v %v)", e.G.HasInitiative, e.G.Initiative)
		}
		// Seat 1's Loot Dispute takes the initiative away on their next
		// turn; seat 0's tome activation then reads the {C} branch.
		dispute := findCardObj(t, e, 1, "Loot Dispute", state.ZHand)
		driveToStep(t, e, e.G.Turn+1, 1, state.StepMain1)
		addMana(t, e, 1, "RRRR")
		submitChoices(t, e, castOptionFor(t, e, dispute).Index)
		drainStackDecliningSearch(t, e)
		if e.G.HasInitiative && e.G.Initiative == 0 {
			t.Fatal("precondition: seat 1's Loot Dispute did not take the initiative")
		}
		passOnceP(t, e)
		before := e.G.Players[0].Pool.Total()
		activateMana(t, e, tome)
		if got := e.G.Players[0].Pool.Total() - before; got != 1 {
			t.Fatalf("pool gain = %d, want 1: not the initiative holder, so Count$Initiative.2.1 reads the {C} branch", got)
		}
		replayCheck(t, e, cfg)
	})
}

// castTome casts Sarevok's Tome from seat 0's hand and stops when the ETB
// "you take the initiative" trigger is the only thing on the stack.
func castTome(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	tome := findCardObj(t, e, 0, "Sarevok's Tome", state.ZHand)
	addMana(t, e, 0, "CCCCC")
	submitChoices(t, e, castOptionFor(t, e, tome).Index)
	// Drain the cast's own asks but stop at the next priority decision, so
	// the ETB trigger stays on the stack.
	for i := 0; i < 12; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while casting the Tome")
		}
		if d.Kind == decision.KPriority {
			return tome
		}
		submitChoices(t, e, 0)
	}
	t.Fatal("the Tome cast never reached priority")
	return 0
}

// drainStackDecliningSearch passes until the stack is empty, declining any
// "search a library: choose up to 1 card(s)" ask the Undercity venture
// rooms pose on the way (Min 0, so the empty answer is legal).
func drainStackDecliningSearch(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while draining the stack")
		}
		if d.Kind == decision.KPriority {
			passOnceP(t, e)
			continue
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "search" {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
				t.Fatalf("decline venture search: %v", err)
			}
			continue
		}
		t.Fatalf("unexpected ask while draining: %+v", d)
	}
}

// TestEDHCountHeadsABosssChauffeur: Boss's Chauffeur enters with a number
// of +1/+1 counters on it equal to ONE PLUS the number of other creatures
// you control (Count$LastStateBattlefield Creature.YouCtrl+Other/Plus.1),
// read off the battlefield frozen as the spell was cast.
func TestEDHCountHeadsABosssChauffeur(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 9425, []string{"Grizzly Bears", "Elvish Mystic", "Boss's Chauffeur"}, nil, nil)
	findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	findCardObj(t, e, 0, "Elvish Mystic", state.ZBattlefield)
	chauffeur := findCardObj(t, e, 0, "Boss's Chauffeur", state.ZHand)
	addMana(t, e, 0, "WWWWWW")
	castObj(t, e, chauffeur)
	o := e.G.Obj(chauffeur)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Boss's Chauffeur not on the battlefield (%v)", o)
	}
	if got := o.Counter("P1P1"); got != 3 {
		t.Fatalf("+1/+1 counters = %d, want 3 (two other creatures you control, plus one)", got)
	}
	replayCheck(t, e, cfg)
}

// TestEDHCountHeadsABosssChauffeurUncast: Boss's Chauffeur put onto the
// battlefield without being cast (Zombify from the graveyard) has no cast
// snapshot, so its last-known battlefield is the live one: it still enters
// with one plus the number of other creatures you control (3 here).
func TestEDHCountHeadsABosssChauffeurUncast(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 9426, []string{"Grizzly Bears", "Elvish Mystic", "Boss's Chauffeur", "Zombify"}, nil, nil)
	findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	findCardObj(t, e, 0, "Elvish Mystic", state.ZBattlefield)
	chauffeur := findCardObj(t, e, 0, "Boss's Chauffeur", state.ZGraveyard)
	zombify := findCardObj(t, e, 0, "Zombify", state.ZHand)
	if o := e.G.Obj(chauffeur); o == nil || o.CastBattlefield != nil {
		t.Fatalf("precondition: the Chauffeur must carry no cast snapshot: %+v", o)
	}
	addMana(t, e, 0, "BBBB")
	castObjTargeting(t, e, zombify, chauffeur)
	passUntilStackEmpty(t, e, 20)
	o := e.G.Obj(chauffeur)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Boss's Chauffeur not on the battlefield after Zombify (%v)", o)
	}
	if got := o.Counter("P1P1"); got != 3 {
		t.Fatalf("+1/+1 counters = %d, want 3 (two other creatures you control, plus one, on the live battlefield)", got)
	}
	replayCheck(t, e, cfg)
}

// TestEDHCountHeadsAImpactResonanceSingleMax: Count$SingleMaxDamageThisTurn is
// the greatest amount of damage a source dealt to ONE permanent or player: a
// source that dealt 3 to a player and 2 to a creature reads 3 (not 5), and
// two hits on the same recipient add.
func TestEDHCountHeadsAImpactResonanceSingleMax(t *testing.T) {
	t.Parallel()
	e, ids := pcdrEngine(t, "Impact Resonance", "Grizzly Bears", "Elvish Mystic")
	body := corpusCardOrFatal(t, "Impact Resonance").Faces[0].SVars["X"]
	if body != "Count$SingleMaxDamageThisTurn" {
		t.Fatalf("precondition: Impact Resonance SVar X = %q", body)
	}
	ctx := &effects.Ctx{Controller: 0, Source: ids["Impact Resonance"]}
	if got := effects.EvalCount(e, ctx, body); got != 0 {
		t.Fatalf("no damage dealt yet: X = %d, want 0", got)
	}
	bears, mystic := ids["Grizzly Bears"], ids["Elvish Mystic"]
	damageCountEvent(e, bears, 0, 1, 3, true)
	damageCountEvent(e, bears, mystic, 0, 2, true)
	if got := effects.EvalCount(e, ctx, body); got != 3 {
		t.Fatalf("X = %d, want 3 (a source's 3 to a player and 2 to a creature are separate recipients)", got)
	}
	damageCountEvent(e, bears, 0, 1, 2, false)
	if got := effects.EvalCount(e, ctx, body); got != 5 {
		t.Fatalf("X = %d, want 5 (two hits on the same player add)", got)
	}
}

// TestEDHCountHeadsAExcessNumCombatDamage: Excess's X is the number of
// creatures you controlled that dealt COMBAT damage to a player this turn
// (Count$NumCombatDamageThisTurn Creature.YouCtrl Player): noncombat damage,
// damage to a creature and an opposing source do not count.
func TestEDHCountHeadsAExcessNumCombatDamage(t *testing.T) {
	t.Parallel()
	e, ids := pcdrEngine(t, "Indulge", "Grizzly Bears", "Elvish Mystic", "Llanowar Elves")
	body := corpusCardOrFatal(t, "Indulge").Faces[1].SVars["X"]
	if body != "Count$NumCombatDamageThisTurn Creature.YouCtrl Player" {
		t.Fatalf("precondition: Excess SVar X = %q", body)
	}
	ctx := &effects.Ctx{Controller: 0, Source: ids["Indulge"]}
	bears, mystic, elves := ids["Grizzly Bears"], ids["Elvish Mystic"], ids["Llanowar Elves"]
	foe := onBoardCard(t, e, 1, corpusCardOrFatal(t, "Grizzly Bears"))
	if got := effects.EvalCount(e, ctx, body); got != 0 {
		t.Fatalf("no damage yet: X = %d, want 0", got)
	}
	damageCountEvent(e, bears, 0, 1, 2, true)      // counts
	damageCountEvent(e, mystic, 0, 1, 1, true)     // counts
	damageCountEvent(e, elves, 0, 1, 1, false)     // noncombat: no
	damageCountEvent(e, elves, mystic, 0, 1, true) // to a creature: no
	damageCountEvent(e, foe, 0, 0, 2, true)        // opposing source: no
	if got := effects.EvalCount(e, ctx, body); got != 2 {
		t.Fatalf("X = %d, want 2 (Bears and Mystic dealt combat damage to a player)", got)
	}
}

// TestEDHCountHeadsAEpharaLastTurnEntered: Ephara's upkeep trigger is gated on
// Count$LastTurnEntered_Battlefield_Creature.YouCtrl+Other: a creature that
// entered under your control on the previous turn draws you a card at the next
// upkeep, and with no such entry it does not.
func TestEDHCountHeadsAEpharaLastTurnEntered(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T, seed uint64, bearsEnter bool) (before, after int) {
		e, _, _ := altCostEngine(t, seed, []string{"Ephara, God of the Polis", "Grizzly Bears"}, nil, nil)
		findCardObj(t, e, 0, "Ephara, God of the Polis", state.ZBattlefield)
		if bearsEnter {
			findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
		}
		turn := e.G.Turn
		before = handSize(e, 0)
		passUntilTurn(t, e, turn+1)
		passUntilStackEmpty(t, e, 30)
		return before, handSize(e, 0)
	}
	b, a := run(t, 9427, true)
	if a != b+1 {
		t.Fatalf("hand %d -> %d, want one Ephara draw (a creature entered last turn)", b, a)
	}
	b, a = run(t, 9427, false)
	if a != b {
		t.Fatalf("hand %d -> %d, want no draw (no other creature entered last turn)", b, a)
	}
}

// TestEDHCountHeadsACastBattlefieldNeedle pins which SVar bodies freeze the
// as-cast battlefield: only a Count$LastStateBattlefield read (either
// spelling). Miss Highwater's Defined$ TriggeredLastStateBattlefieldLKICopy
// body names the substring but reads neither head, so casting her must not
// retain a battlefield snapshot.
func TestEDHCountHeadsACastBattlefieldNeedle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"plain", "Count$LastStateBattlefield Artifact.YouCtrl", true},
		{"withFallback", "Count$LastStateBattlefieldWithFallback Land.OppCtrl", true},
		{"missHighwaterLKICopy", "DB$ CopyPermanent | Defined$ TriggeredLastStateBattlefieldLKICopy.Creature+ControlledBy TriggeredPlayer,Artifact+ControlledBy TriggeredPlayer", false},
		{"unrelated", "Count$Valid Creature.YouCtrl", false},
	}
	for _, tc := range cases {
		if got := effects.SVarsReadCastBattlefield(map[string]string{"S": tc.body}); got != tc.want {
			t.Errorf("%s: SVarsReadCastBattlefield = %v, want %v", tc.name, got, tc.want)
		}
	}
}
