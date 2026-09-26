package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The AddsCounters$ mana-spend rider, corpus-shape coverage (task opalp2).
// The sibling Opal Palace suite (rules/opal_palace_test.go) pins the commander
// shape and rules/adds_counters_ability_test.go pins the per-unit and
// per-ability attribution with synthetic fixtures. This file pins the three
// REMAINING real corpus carriers, each as an inline script SHAPE (never a
// .cards/ .txt, per the licensing rule):
//
//   Biophagus          A:AB$ Mana | Cost$ T | Produced$ Any | AddsCounters$ Card.Creature_P1P1_1
//   Animal Attendant   A:AB$ Mana | Cost$ T | Produced$ Any | AddsCounters$ Creature.nonHuman_P1P1_1
//   Guildmages' Forum  A:AB$ Mana | Cost$ 1 T | Produced$ Any | AddsCounters$ Card.Creature+MultiColor_P1P1_1
//
// Each test asserts: the mana ability really produced a unit, that unit was
// really the one consumed by the cast (the batch count drops to zero), the
// spell really entered the battlefield, and the rider filter's positive
// creature gets exactly one +1/+1 while its negative counterpart gets none.

// acrBiophagusSrc is a Biophagus-shaped mana creature: "{T}: Add one mana of any
// color. If this mana is spent to cast a creature spell, that creature enters
// with an additional +1/+1 counter on it." Produced$ Any asks a colour, so the
// activation is a two-stage mana decision.
const acrBiophagusSrc = `Name:Biophagus Test
ManaCost:1 G
Types:Creature Human Tyranid Wizard
PT:1/3
A:AB$ Mana | Cost$ T | Produced$ Any | AddsCounters$ Card.Creature_P1P1_1 | SpellDescription$ Add one mana of any color. If this mana is spent to cast a creature spell, that creature enters with an additional +1/+1 counter on it.
Oracle:x
`

// acrAttendantSrc is an Animal Attendant-shaped mana creature with the nonHuman
// filter, so a Human creature must NOT receive the counter.
const acrAttendantSrc = `Name:Animal Attendant Test
ManaCost:1 G
Types:Creature Human Citizen
PT:1/2
A:AB$ Mana | Cost$ T | Produced$ Any | AddsCounters$ Creature.nonHuman_P1P1_1 | SpellDescription$ Add one mana of any color. If that mana is spent to cast a non-Human creature spell, that creature enters with an additional +1/+1 counter on it.
Oracle:x
`

// acrForumSrc is a Guildmages' Forum-shaped land: the SECOND ability (the {1}
// one) carries the multicolour rider, the first is an ordinary {C} ability
// with no rider, so the test can also prove rider attribution is per ability.
const acrForumSrc = `Name:Guildmages' Forum Test
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.
A:AB$ Mana | Cost$ 1 T | Produced$ Any | AddsCounters$ Card.Creature+MultiColor_P1P1_1 | SpellDescription$ Add one mana of any color. If that mana is spent on a multicolored creature spell, that creature enters with an additional +1/+1 counter on it.
Oracle:x
`

// --- fixtures for the positive/negative entrant of each filter ---

// acrMonoCreatureSrc is a plain monocoloured creature: matches Card.Creature and
// Creature.nonHuman, does NOT match Card.Creature+MultiColor.
const acrMonoCreatureSrc = `Name:Mono Beast
ManaCost:R
Types:Creature Beast
PT:2/2
Oracle:x
`

// acrHumanCreatureSrc is a monocoloured Human creature: matches Card.Creature but
// fails Creature.nonHuman, so Animal Attendant must not pay it.
const acrHumanCreatureSrc = `Name:Human Soldier
ManaCost:R
Types:Creature Human Soldier
PT:2/2
Oracle:x
`

// acrGoldCreatureSrc is a two-colour creature: the Card.Creature+MultiColor
// positive, and also a Card.Creature positive.
const acrGoldCreatureSrc = `Name:Gold Beast
ManaCost:R G
Types:Creature Beast
PT:2/3
Oracle:x
`

// acrArtifactSrc is a noncreature artifact: matches no rider filter, so the
// Biophagus negative pays it but grants nothing.
const acrArtifactSrc = `Name:Plain Relic
ManaCost:R
Types:Artifact
Oracle:x
`

// activateAnyRider activates the AddsCounters$-bearing mana ability on obj,
// answering the ability wheel (only posed when the source has more than one
// mana ability) with that ability and then the colour ask with the requested
// colour. It leaves exactly one rider unit in the pool. A {1}-style activation
// cost must already be funded by the caller.
func activateAnyRider(t *testing.T, e *Engine, obj state.ObjID, colour string) {
	t.Helper()
	mas := e.availableManaAbilities(0, obj)
	riderIdx := -1
	for i, ma := range mas {
		if strings.TrimSpace(ma.Params["AddsCounters"]) != "" {
			riderIdx = i
		}
	}
	if riderIdx < 0 {
		t.Fatalf("precondition: %d has no AddsCounters$ mana ability (%d abilities)", obj, len(mas))
	}
	e.priorityRound()
	activateMana(t, e, obj)
	// Stage 1: the ability wheel is posed only for a multi-ability source and
	// is named by its prompt; a single-ability source drops straight to the
	// colour ask (rules/mana_activation.go resolves one ability directly).
	d := e.Pending()
	if d != nil && d.Kind == decision.KChoose && strings.HasPrefix(d.Prompt, "Choose a mana ability") {
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "mana" && o.Ability == riderIdx {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("no rider mana-ability option (index %d): %+v", riderIdx, d.Options)
		}
		submitChoices(t, e, idx)
		d = e.Pending()
	}
	// Stage 2: the concrete colour.
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("no colour ask after activating the rider mana on %d: %+v", obj, d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "mana" && strings.HasSuffix(o.Label, "Add "+colour) {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no Add %s choice for the rider mana: %+v", colour, d.Options)
	}
	submitChoices(t, e, idx)
}

// riderBatchFor asserts exactly one provenance batch is present, sourced to
// src and carrying a non-empty AddsCounters$ rider, and returns it.
func riderBatchFor(t *testing.T, e *Engine, src state.ObjID) {
	t.Helper()
	batches := e.G.Players[0].RestrictedMana
	if len(batches) != 1 {
		t.Fatalf("precondition: provenance batches = %d, want 1", len(batches))
	}
	if b := batches[0]; b.Source != src || strings.TrimSpace(b.AddsCounters) == "" {
		t.Fatalf("precondition: batch = %+v, want a rider batch sourced to %d", b, src)
	}
}

// TestBiophagusAddsCountersPaysCreatureNotArtifact pins Biophagus's
// Card.Creature rider: the mana produced by its ability pays for a creature
// spell, which enters with exactly one +1/+1; the same ability's mana pays for
// a noncreature artifact, which enters with none.
func TestBiophagusAddsCountersPaysCreatureNotArtifact(t *testing.T) {
	e, cfg := riderGame(t, 601,
		card(t, acrBiophagusSrc), card(t, acrMonoCreatureSrc), card(t, acrArtifactSrc))
	bio := moveToBattlefieldByName(t, e, 0, "Biophagus Test")
	beast := moveSeededToHand(t, e, 0, "Mono Beast")
	relic := moveSeededToHand(t, e, 0, "Plain Relic")

	if o := e.G.Obj(bio); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: Biophagus = %+v, want untapped on the battlefield", o)
	}

	// Negative: the rider mana pays for a noncreature artifact.
	activateAnyRider(t, e, bio, "R")
	riderBatchFor(t, e, bio)
	if got := e.G.Players[0].Pool[state.MR]; got != 1 {
		t.Fatalf("precondition: pool R = %d, want 1 after the rider activation", got)
	}
	castSeeded(t, e, relic)
	passUntilStackEmpty(t, e, 30)
	ro := e.G.Obj(relic)
	if ro == nil || ro.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Plain Relic zone = %v, want the battlefield", ro.Zone)
	}
	if len(e.G.Players[0].RestrictedMana) != 0 {
		t.Fatalf("precondition: the rider batch was not consumed by the artifact cast: %+v", e.G.Players[0].RestrictedMana)
	}

	// Positive: untap, produce another rider unit, and pay for a creature.
	e.emit(events.Event{Kind: events.Untap, Obj: bio})
	e.pending = nil
	e.Advance()
	activateAnyRider(t, e, bio, "R")
	riderBatchFor(t, e, bio)
	castSeeded(t, e, beast)
	passUntilStackEmpty(t, e, 30)
	bo := e.G.Obj(beast)
	if bo == nil || bo.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Mono Beast zone = %v, want the battlefield", bo.Zone)
	}
	if len(e.G.Players[0].RestrictedMana) != 0 {
		t.Fatalf("precondition: the rider batch was not consumed by the creature cast: %+v", e.G.Players[0].RestrictedMana)
	}
	if got := bo.Counter("P1P1"); got != 1 {
		t.Fatalf("Mono Beast counters after the Biophagus rider mana paid = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

// TestAnimalAttendantAddsCountersPaysNonHumanNotHuman pins Animal Attendant's
// Creature.nonHuman rider: a non-Human creature gets one +1/+1, a Human
// creature gets none even though it is a creature.
func TestAnimalAttendantAddsCountersPaysNonHumanNotHuman(t *testing.T) {
	e, cfg := riderGame(t, 602,
		card(t, acrAttendantSrc), card(t, acrMonoCreatureSrc), card(t, acrHumanCreatureSrc))
	att := moveToBattlefieldByName(t, e, 0, "Animal Attendant Test")
	beast := moveSeededToHand(t, e, 0, "Mono Beast")
	human := moveSeededToHand(t, e, 0, "Human Soldier")

	if o := e.G.Obj(att); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: Animal Attendant = %+v, want untapped on the battlefield", o)
	}

	// Negative: a Human creature spell paid with the rider mana gets nothing.
	activateAnyRider(t, e, att, "R")
	riderBatchFor(t, e, att)
	if got := e.G.Players[0].Pool[state.MR]; got != 1 {
		t.Fatalf("precondition: pool R = %d, want 1 after the rider activation", got)
	}
	castSeeded(t, e, human)
	passUntilStackEmpty(t, e, 30)
	ho := e.G.Obj(human)
	if ho == nil || ho.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Human Soldier zone = %v, want the battlefield", ho.Zone)
	}
	if len(e.G.Players[0].RestrictedMana) != 0 {
		t.Fatalf("precondition: the rider batch was not consumed by the Human cast: %+v", e.G.Players[0].RestrictedMana)
	}
	if got := ho.Counter("P1P1"); got != 0 {
		t.Fatalf("Human Soldier counters after the rider mana paid = %d, want 0 (nonHuman fails)", got)
	}

	// Positive: a non-Human creature spell gets exactly one.
	e.emit(events.Event{Kind: events.Untap, Obj: att})
	e.pending = nil
	e.Advance()
	activateAnyRider(t, e, att, "R")
	riderBatchFor(t, e, att)
	castSeeded(t, e, beast)
	passUntilStackEmpty(t, e, 30)
	bo := e.G.Obj(beast)
	if bo == nil || bo.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Mono Beast zone = %v, want the battlefield", bo.Zone)
	}
	if len(e.G.Players[0].RestrictedMana) != 0 {
		t.Fatalf("precondition: the rider batch was not consumed by the non-Human cast: %+v", e.G.Players[0].RestrictedMana)
	}
	if got := bo.Counter("P1P1"); got != 1 {
		t.Fatalf("Mono Beast counters after the Animal Attendant rider mana paid = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

// TestGuildmagesForumAddsCountersPaysMultiColorNotMono pins Guildmages'
// Forum's Card.Creature+MultiColor rider: a two-colour creature gets one
// +1/+1, a monocoloured creature gets none. The land also has a rider-LESS
// {C} ability, so the negative is paid by ordinary mana from the same
// permanent, proving the grant belongs to the rider ability.
func TestGuildmagesForumAddsCountersPaysMultiColorNotMono(t *testing.T) {
	e, cfg := riderGame(t, 603,
		card(t, acrForumSrc), card(t, acrGoldCreatureSrc), card(t, acrMonoCreatureSrc))
	forum := moveToBattlefieldByName(t, e, 0, "Guildmages' Forum Test")
	gold := moveSeededToHand(t, e, 0, "Gold Beast")
	mono := moveSeededToHand(t, e, 0, "Mono Beast")

	if o := e.G.Obj(forum); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: Guildmages' Forum = %+v, want untapped on the battlefield", o)
	}

	// Negative: pay the monocoloured creature with the rider ability's mana;
	// Card.Creature+MultiColor fails, so no counter. The {1} activation fee is
	// funded with ordinary colourless mana (the payability gate must see it).
	addMana(t, e, 0, "C")
	activateAnyRider(t, e, forum, "R")
	riderBatchFor(t, e, forum)
	if got := e.G.Players[0].Pool[state.MR]; got != 1 {
		t.Fatalf("precondition: pool R = %d, want 1 after the rider activation", got)
	}
	castSeeded(t, e, mono)
	passUntilStackEmpty(t, e, 30)
	mo := e.G.Obj(mono)
	if mo == nil || mo.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Mono Beast zone = %v, want the battlefield", mo.Zone)
	}
	if len(e.G.Players[0].RestrictedMana) != 0 {
		t.Fatalf("precondition: the rider batch was not consumed by the monocolour cast: %+v", e.G.Players[0].RestrictedMana)
	}
	if got := mo.Counter("P1P1"); got != 0 {
		t.Fatalf("Mono Beast counters after the multicolour rider mana paid = %d, want 0 (MultiColor fails)", got)
	}

	// Positive: the two-colour creature gets exactly one. Untap, fund {1},
	// produce the rider unit, then an ordinary G for the second pip.
	e.emit(events.Event{Kind: events.Untap, Obj: forum})
	e.pending = nil
	e.Advance()
	addMana(t, e, 0, "C")
	activateAnyRider(t, e, forum, "G")
	riderBatchFor(t, e, forum)
	addMana(t, e, 0, "R")
	castSeeded(t, e, gold)
	passUntilStackEmpty(t, e, 30)
	go2 := e.G.Obj(gold)
	if go2 == nil || go2.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Gold Beast zone = %v, want the battlefield", go2.Zone)
	}
	if len(e.G.Players[0].RestrictedMana) != 0 {
		t.Fatalf("precondition: the rider batch was not consumed by the multicolour cast: %+v", e.G.Players[0].RestrictedMana)
	}
	if got := go2.Counter("P1P1"); got != 1 {
		t.Fatalf("Gold Beast counters after the multicolour rider mana paid = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}
