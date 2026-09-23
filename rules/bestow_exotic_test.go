package rules

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Bestow's three exotic costs (CR 702.114a) are priced and offered, not
// withheld: Nyxborn Hydra's {X}{G}{G} announces X through the ordinary cast
// machinery, Detective's Phoenix's {R} plus CollectEvidence<6> settles through
// the shared evidence payment, and Hypnotic Siren's colon-suffixed
// "5 U U:GainControl" line pays {5}{U}{U} -- the ":GainControl" is Forge
// metadata for the card's own GainControl static, not cost text. The CR
// 702.114c nuance is pinned separately: a bestowed spell is an Aura spell, not
// a creature spell, so it fires no "cast a creature spell" trigger and does
// fire a "cast an Aura spell" one.
//
// The corpus is required: every test loads the real carrier through
// testutil.CorpusRegistry.

// lifeGainFor counts the LifeChange events a trigger source emitted for p.
func lifeGainFor(e *Engine, p state.PlayerID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.LifeChange && ev.Player == p && ev.Amount > 0 {
			n += int(ev.Amount)
		}
	}
	return n
}

// triggerGainLifeSrc builds a battlefield enchantment that gains `amount`
// life whenever its controller casts a spell matching spec (a real SpellCast
// trigger, the Soul Warden shape). Distinct amounts let a test tell WHICH
// watcher fired, not merely that some life was gained.
func triggerGainLifeSrc(name, spec string, amount int) string {
	return "Name:" + name + "\nManaCost:1 W\nTypes:Enchantment\nOracle:x\n" +
		"T:Mode$ SpellCast | ValidCard$ " + spec + " | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigGainLife | TriggerDescription$ Whenever you cast a spell, you gain life.\n" +
		"SVar:TrigGainLife:DB$ GainLife | LifeAmount$ " + strconv.Itoa(amount) + "\n"
}

// TestNyxbornHydraBestowXCostIsOfferedAndAnnounced: the {X}{G}{G} bestow cost
// is offered as a bestowed cast, announces X before payment (CR 601.2b), and
// resolves as an attached Aura. X is announced as 1 so the announcement is
// observable, and the pool carries exactly one extra green for it.
func TestNyxbornHydraBestowXCostIsOfferedAndAnnounced(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	hydra := mustCorpusCard(t, reg, "Nyxborn Hydra")
	e, cfg := tokenReplGame(t, 1451, hydra)
	hydraID := moveSeededCard(t, e, 0, hydra, state.ZHand)
	bear := putToken(t, e, 0, bestowBearerSrc, state.ZBattlefield)
	addMana(t, e, 0, "GGG") // {X=1}{G}{G}

	bestowed := bestowedCastOption(t, e, hydraID) // fails the test if not offered
	submitChoices(t, e, bestowed.Index)
	// Precondition: the X announcement is posed before targets/payment.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("bestowed hydra did not announce X first: %+v", d)
	}
	x1 := -1
	for _, o := range d.Options {
		if o.Label == "X = 1" {
			x1 = o.Index
		}
	}
	if x1 < 0 {
		t.Fatalf("no X = 1 option on the announcement: %+v", d.Options)
	}
	submitChoices(t, e, x1)
	// Target ask through the synthesized attach SA.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("bestowed hydra target ask: %+v", d)
	}
	tgt := indexOfObjOption(d, bear)
	if tgt < 0 {
		t.Fatalf("bear not offered as a bestow target: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 20)

	o := e.G.Obj(hydraID)
	if o.Zone != state.ZBattlefield || o.AttachedTo != bear {
		t.Fatalf("hydra zone %s attached %d, want battlefield/bear %d", o.Zone, o.AttachedTo, bear)
	}
	if e.IsCreature(hydraID) {
		t.Fatal("bestowed-attached Nyxborn Hydra must not be a creature (CR 702.114e)")
	}
	if !sawBestowedCastInfo(t, e, hydraID) {
		t.Fatal("no FlagBestowed CastInfo on the bestowed hydra cast")
	}
	replayCheck(t, e, cfg)
}

// TestDetectivesPhoenixBestowCollectEvidenceIsPaid: the CollectEvidence<6>
// bestow cost is offered as a bestowed cast, poses the evidence ask, and exiles
// the chosen graveyard cards as the payment before resolving attached.
func TestDetectivesPhoenixBestowCollectEvidenceIsPaid(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	phoenix := mustCorpusCard(t, reg, "Detective's Phoenix")
	relicCard := card(t, "Name:Big Relic\nManaCost:6\nTypes:Artifact\nOracle:x\n")
	e, cfg := tokenReplGame(t, 1452, phoenix, relicCard)
	phoenixID := moveSeededCard(t, e, 0, phoenix, state.ZHand)
	bear := putToken(t, e, 0, bestowBearerSrc, state.ZBattlefield)
	relic := moveSeededCard(t, e, 0, relicCard, state.ZGraveyard)
	addMana(t, e, 0, "R")

	// Precondition: without the parsed Evidence part the offer would either
	// not appear or would charge a phantom generic; the graveyard card really
	// is in the graveyard.
	if e.G.Obj(relic).Zone != state.ZGraveyard {
		t.Fatalf("setup: evidence card zone %s", e.G.Obj(relic).Zone)
	}
	bestowed := bestowedCastOption(t, e, phoenixID)
	submitChoices(t, e, bestowed.Index)
	// The target ask precedes the evidence ask (CR 601.2c before 601.2h).
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("bestowed phoenix target ask: %+v", d)
	}
	tgt := indexOfObjOption(d, bear)
	if tgt < 0 {
		t.Fatalf("bear not offered as a bestow target: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "evidence" {
		t.Fatalf("bestowed phoenix evidence ask: %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 20)

	if e.G.Obj(relic).Zone != state.ZExile {
		t.Fatalf("evidence card zone %s, want Exile", e.G.Obj(relic).Zone)
	}
	o := e.G.Obj(phoenixID)
	if o.Zone != state.ZBattlefield || o.AttachedTo != bear {
		t.Fatalf("phoenix zone %s attached %d, want battlefield/bear %d", o.Zone, o.AttachedTo, bear)
	}
	if !e.HasKeyword(bear, "Flying") || !e.HasKeyword(bear, "Haste") {
		t.Fatalf("bestowed phoenix static not live on the bearer: %v", e.Derived(bear).Keywords)
	}
	replayCheck(t, e, cfg)
}

// TestHypnoticSirenBestowColonSuffixIsStripped: the "5 U U:GainControl"
// keyword parameter pays {5}{U}{U} (only the colon-free field is cost text),
// and the card's own GainControl static gives the caster control of the
// enchanted creature.
func TestHypnoticSirenBestowColonSuffixIsStripped(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	siren := mustCorpusCard(t, reg, "Hypnotic Siren")
	e, cfg := tokenReplGame(t, 1453, siren)
	sirenID := moveSeededCard(t, e, 0, siren, state.ZHand)
	// The bearer is seat 1's, so the control static's effect is observable.
	bear := putToken(t, e, 1, bestowBearerSrc, state.ZBattlefield)
	addMana(t, e, 0, "UUUUUUU") // {5}{U}{U}

	if e.G.Obj(bear).Controller != 1 {
		t.Fatalf("setup: bearer controller %d, want seat 1", e.G.Obj(bear).Controller)
	}
	bestowed := bestowedCastOption(t, e, sirenID)
	submitChoices(t, e, bestowed.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("bestowed siren target ask: %+v", d)
	}
	tgt := indexOfObjOption(d, bear)
	if tgt < 0 {
		t.Fatalf("bear not offered as a bestow target: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 20)

	o := e.G.Obj(sirenID)
	if o.Zone != state.ZBattlefield || o.AttachedTo != bear {
		t.Fatalf("siren zone %s attached %d, want battlefield/bear %d", o.Zone, o.AttachedTo, bear)
	}
	// Precondition: the bearer is still on the battlefield and its printed
	// controller differs from the siren's caster, so the control change is a
	// real transition.
	if e.G.Obj(bear).Zone != state.ZBattlefield {
		t.Fatalf("bear zone %s, want battlefield", e.G.Obj(bear).Zone)
	}
	if got := e.G.Obj(bear).Controller; got != 0 {
		t.Fatalf("bear controller %d after the bestowed siren resolved, want the siren's controller 0 (GainControl$ You)", got)
	}
	replayCheck(t, e, cfg)
}

// TestBestowedSpellIsAuraNotCreature is the CR 702.114c pin: a card cast with
// its bestow ability is an Aura spell, not a creature spell, so a "whenever
// you cast a creature spell" trigger does NOT fire, while a "whenever you cast
// an Aura spell" trigger DOES -- and the SAME creature-spell trigger still
// fires for the plain creature cast of the same card.
func TestBestowedSpellIsAuraNotCreature(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	archon := mustCorpusCard(t, reg, "Celestial Archon")

	// bestowedLeg: cast the Archon bestowed; the creature-spell watcher (3
	// life) must stay silent and the Aura-spell watcher (1 life) must fire.
	t.Run("bestowed", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 1454, archon)
		archonID := moveSeededCard(t, e, 0, archon, state.ZHand)
		bear := putToken(t, e, 0, bestowBearerSrc, state.ZBattlefield)
		putToken(t, e, 0, triggerGainLifeSrc("Creature Watcher", "Creature", 3), state.ZBattlefield)
		putToken(t, e, 0, triggerGainLifeSrc("Aura Watcher", "Aura", 1), state.ZBattlefield)
		addMana(t, e, 0, "WWWWWWW")

		// Precondition: both watchers are on the battlefield before the cast.
		if got := countNameOnBattlefield(e, 0, "Creature Watcher"); got != 1 {
			t.Fatalf("creature watcher count = %d, want 1", got)
		}
		if got := countNameOnBattlefield(e, 0, "Aura Watcher"); got != 1 {
			t.Fatalf("aura watcher count = %d, want 1", got)
		}
		bestowCastOnto(t, e, bestowedCastOption(t, e, archonID), bear)
		if got := lifeGainFor(e, 0); got != 1 {
			t.Fatalf("life gained = %d, want exactly 1 (the Aura-spell watcher; the 3-life creature-spell watcher must not fire)", got)
		}
		replayCheck(t, e, cfg)
	})

	// plainLeg: the ordinary creature cast still fires the 3-life creature-
	// spell watcher, which proves the bestowed leg's silence is the type switch
	// and not a dead watcher.
	t.Run("plain", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 1455, archon)
		archonID := moveSeededCard(t, e, 0, archon, state.ZHand)
		putToken(t, e, 0, triggerGainLifeSrc("Creature Watcher", "Creature", 3), state.ZBattlefield)
		putToken(t, e, 0, triggerGainLifeSrc("Aura Watcher", "Aura", 1), state.ZBattlefield)
		addMana(t, e, 0, "WWWWW") // the plain {3}{W}{W}

		if got := countNameOnBattlefield(e, 0, "Creature Watcher"); got != 1 {
			t.Fatalf("creature watcher count = %d, want 1", got)
		}
		submitChoices(t, e, plainCastOption(t, e, archonID).Index)
		passUntilStackEmpty(t, e, 20)
		if got := lifeGainFor(e, 0); got != 3 {
			t.Fatalf("life gained = %d on the plain cast, want exactly 3 (the creature-spell watcher)", got)
		}
		replayCheck(t, e, cfg)
	})
}

// countNameOnBattlefield counts p's battlefield permanents whose printed name
// is name -- a precondition helper for the watcher fixtures.
func countNameOnBattlefield(e *Engine, p state.PlayerID, name string) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}
