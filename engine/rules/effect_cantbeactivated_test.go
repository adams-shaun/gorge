package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const cbaArtifactName = "CBA Relic"

// cbaArtifactSrc is a nonland permanent with one activated ability whose cost
// is mana, not {T}: it stays activatable while Braided Net has it tapped, so
// the offer assertion measures the restriction rather than the tap cost.
const cbaArtifactSrc = "Name:" + cbaArtifactName + "\nManaCost:0\nTypes:Artifact\n" +
	"A:AB$ GainLife | Cost$ 1 | Defined$ You | LifeAmount$ 2 | SpellDescription$ You gain 2 life.\nOracle:x\n"

const cbaPowerUpCreatureName = "CBA PowerUp Beast"

// cbaPowerUpCreatureSrc carries the three ability shapes the ValidSA$
// Activated.PowerUp term must separate: a PowerUp$ True ability, an ordinary
// activated ability, and a mana ability.
const cbaPowerUpCreatureSrc = "Name:" + cbaPowerUpCreatureName + "\nManaCost:0\nTypes:Creature Beast\nPT:2/2\n" +
	"A:AB$ PutCounter | Cost$ 1 | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1 | PowerUp$ True | SpellDescription$ Put a +1/+1 counter on this creature.\n" +
	"A:AB$ Draw | Cost$ 1 | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\nOracle:x\n"

// cbaEngine builds a two-seat, seat-0-start game at turn 1's main phase with
// both decks seeded from compiled *cards.Card values (so replayCheck can
// reconstruct every object) and the rest filled with Mountains.
func cbaEngine(t *testing.T, seed uint64, seat0, seat1 []*cards.Card) (*Engine, Config) {
	t.Helper()
	fill := func(deck []*cards.Card) []*cards.Card {
		if len(deck) > 40 {
			t.Fatalf("fixture deck too large: %d", len(deck))
		}
		out := append([]*cards.Card(nil), deck...)
		return append(out, mountainDeck(t, 40-len(out))...)
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{fill(seat0), fill(seat1)}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// cbaResolveEffect resolves src's DBEffect SVar for seat 0.
func cbaResolveEffect(t *testing.T, e *Engine, src state.ObjID, targets []state.Target) {
	t.Helper()
	o := e.G.Obj(src)
	if o == nil || o.Face() == nil {
		t.Fatalf("precondition: Effect source %d has no face", src)
	}
	sa := cards.ResolveSVar(o.Face().SVars, "DBEffect")
	if sa == nil {
		t.Fatalf("precondition: source %d has no DBEffect SVar", src)
	}
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0, SVars: o.Face().SVars, Targets: targets}, sa)
}

// cbaNotes counts the unimplemented Notes this change must eliminate on the
// supported bodies.
func cbaNotes(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "CantBeActivated unimplemented") {
			n++
		}
	}
	return n
}

func cbaAbility(t *testing.T, e *Engine, id state.ObjID) *cards.SA {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || len(o.Face().Abilities) == 0 {
		t.Fatalf("precondition: object %d has no printed ability", id)
	}
	return o.Face().Abilities[0]
}

// cbaClassifyAbilities splits the fixture's PowerUp, ordinary activated and
// mana abilities, failing when the fixture does not actually carry all three
// distinct shapes the assertions compare.
func cbaClassifyAbilities(t *testing.T, e *Engine, id state.ObjID) (powerUp, ordinary, mana *cards.SA) {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		t.Fatalf("precondition: object %d has no face", id)
	}
	for _, sa := range o.Face().Abilities {
		switch {
		case strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKPowerUp)), "True"):
			powerUp = sa
		case cards.IsManaAbilitySA(sa):
			mana = sa
		default:
			ordinary = sa
		}
	}
	if powerUp == nil || ordinary == nil || mana == nil {
		t.Fatalf("precondition: fixture lacks three distinct ability shapes: %+v", o.Face().Abilities)
	}
	if powerUp == ordinary || ordinary == mana || powerUp == mana {
		t.Fatal("precondition: ability classes overlap")
	}
	return powerUp, ordinary, mana
}

func TestEffectCantBeActivatedBraidedNet(t *testing.T) {
	t.Parallel()
	netCard := corpusAlternativeCard(t, "Braided Net")
	artifactCard := card(t, cbaArtifactSrc)
	e, cfg := cbaEngine(t, 11, []*cards.Card{netCard}, []*cards.Card{artifactCard})
	net := moveByName(t, e, 0, "Braided Net", state.ZBattlefield)
	art := moveByName(t, e, 1, cbaArtifactName, state.ZBattlefield)
	e.pending = nil
	e.Advance()
	if e.G.Obj(net).Zone != state.ZBattlefield || e.G.Obj(art).Zone != state.ZBattlefield {
		t.Fatalf("precondition: zones net=%s artifact=%s", e.G.Obj(net).Zone, e.G.Obj(art).Zone)
	}
	ab := cbaAbility(t, e, art)
	addMana(t, e, 1, "C")
	if !hasAbilityOptionFor(e.legalActions(1), art) {
		t.Fatalf("precondition: artifact ability not offered before the lock: %+v", e.legalActions(1))
	}
	if e.abilityRestricted(1, art, ab) {
		t.Fatal("precondition: artifact ability restricted before Braided Net resolves")
	}
	// Drive Braided Net's real activated ability: {T}, remove a net counter,
	// tap another target nonland permanent, then its DBEffect latches the lock.
	opt := abilityOption(t, e, net, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Braided Net target ask = %+v", d)
	}
	targetIdx := -1
	for _, o := range d.Options {
		if o.Obj == art {
			targetIdx = o.Index
		}
	}
	if targetIdx < 0 {
		t.Fatalf("target artifact absent from Braided Net's targets: %+v", d.Options)
	}
	submitChoices(t, e, targetIdx)
	passUntilStackEmpty(t, e, 20)
	if !e.G.Obj(art).Tapped {
		t.Fatal("precondition: Braided Net did not tap the target artifact")
	}
	if n := cbaNotes(e); n != 0 {
		t.Fatalf("Effect-delivered CantBeActivated emitted %d unimplemented Notes", n)
	}
	if !e.abilityRestricted(1, art, ab) {
		t.Fatal("remembered artifact's activated ability is not restricted while tapped")
	}
	if hasAbilityOptionFor(e.legalActions(1), art) {
		t.Fatalf("artifact ability offered while tapped and locked: %+v", e.legalActions(1))
	}
	// Seat 1's untap step untaps the remembered object; CR 611.2b ends the
	// "for as long as it remains tapped" lock there.
	driveToStep(t, e, 2, 1, state.StepMain1)
	if e.G.Obj(art).Tapped {
		t.Fatal("precondition: artifact did not untap at seat 1's untap step")
	}
	addMana(t, e, 1, "C")
	if e.abilityRestricted(1, art, ab) {
		t.Fatal("the UntilTargetedUntaps lock survived the remembered object's untap")
	}
	if !hasAbilityOptionFor(e.legalActions(1), art) {
		t.Fatalf("artifact ability not offered after its untap: %+v", e.legalActions(1))
	}
	replayCheck(t, e, cfg)
}

func TestEffectCantBeActivatedKangPowerUp(t *testing.T) {
	t.Parallel()
	kangCard := corpusAlternativeCard(t, "Kang the Conqueror")
	creatureCard := card(t, cbaPowerUpCreatureSrc)
	e, cfg := cbaEngine(t, 12, []*cards.Card{kangCard, creatureCard}, nil)
	kang := moveByName(t, e, 0, "Kang the Conqueror", state.ZBattlefield)
	creature := moveByName(t, e, 0, cbaPowerUpCreatureName, state.ZBattlefield)
	e.pending = nil
	e.Advance()
	// Kang's real effect body: TrigEffect -> DB$ Effect | StaticAbilities$
	// STCantPowerUp, whose static is ValidSA$ Activated.PowerUp.
	ko := e.G.Obj(kang)
	effectSA := cards.ResolveSVar(ko.Face().SVars, "TrigEffect")
	if effectSA == nil {
		t.Fatalf("precondition: Kang has no TrigEffect body: %+v", ko.Face().SVars)
	}
	effects.Resolve(e, &effects.Ctx{Source: kang, Controller: 0, SVars: ko.Face().SVars}, effectSA)
	if n := cbaNotes(e); n != 0 {
		t.Fatalf("Kang's CantBeActivated body emitted %d unimplemented Notes", n)
	}
	powerUp, ordinary, mana := cbaClassifyAbilities(t, e, creature)
	if !e.abilityRestricted(0, creature, powerUp) {
		t.Fatal("PowerUp$ True ability is not restricted by the registered Kang body")
	}
	if e.abilityRestricted(0, creature, ordinary) {
		t.Fatal("ordinary activated ability restricted by Activated.PowerUp")
	}
	if e.abilityRestricted(0, creature, mana) {
		t.Fatal("mana ability restricted by Activated.PowerUp")
	}
	replayCheck(t, e, cfg)
}

func TestCantBeActivatedValidSAPowerUpPrinted(t *testing.T) {
	t.Parallel()
	lockCard := card(t, "Name:CBA PowerUp Lock\nManaCost:0\nTypes:Enchantment\n"+
		"S:Mode$ CantBeActivated | ValidCard$ Card | ValidSA$ Activated.PowerUp | Description$ Power-up abilities can't be activated.\nOracle:x\n")
	creatureCard := card(t, cbaPowerUpCreatureSrc)
	e, cfg := cbaEngine(t, 13, []*cards.Card{lockCard, creatureCard}, nil)
	lock := moveByName(t, e, 0, "CBA PowerUp Lock", state.ZBattlefield)
	creature := moveByName(t, e, 0, cbaPowerUpCreatureName, state.ZBattlefield)
	e.pending = nil
	e.Advance()
	if e.G.Obj(lock).Zone != state.ZBattlefield || e.G.Obj(creature).Zone != state.ZBattlefield {
		t.Fatalf("precondition: zones lock=%s creature=%s", e.G.Obj(lock).Zone, e.G.Obj(creature).Zone)
	}
	powerUp, ordinary, mana := cbaClassifyAbilities(t, e, creature)
	if !e.abilityRestricted(0, creature, powerUp) {
		t.Fatal("printed Activated.PowerUp does not restrict the power-up ability")
	}
	if e.abilityRestricted(0, creature, ordinary) {
		t.Fatal("printed Activated.PowerUp restricted an ordinary activated ability")
	}
	if e.abilityRestricted(0, creature, mana) {
		t.Fatal("printed Activated.PowerUp restricted a mana ability")
	}
	replayCheck(t, e, cfg)
}

func TestEffectCantBeActivatedRememberedActivator(t *testing.T) {
	t.Parallel()
	srcCard := card(t, "Name:CBA Remembered Source\nManaCost:0\nTypes:Enchantment\n"+
		"SVar:STCantBeActivated:Mode$ CantBeActivated | ValidCard$ Card | ValidSA$ Activated | Activator$ Player.IsRemembered | Description$ x\n"+
		"SVar:DBEffect:DB$ Effect | StaticAbilities$ STCantBeActivated | RememberObjects$ TargetedPlayer | Duration$ Permanent\nOracle:x\n")
	artifactCard := card(t, cbaArtifactSrc)
	e, cfg := cbaEngine(t, 14, []*cards.Card{srcCard}, []*cards.Card{artifactCard})
	src := moveByName(t, e, 0, "CBA Remembered Source", state.ZBattlefield)
	art := moveByName(t, e, 1, cbaArtifactName, state.ZBattlefield)
	e.pending = nil
	e.Advance()
	cbaResolveEffect(t, e, src, []state.Target{{IsPlayer: true, Player: 1}})
	if n := cbaNotes(e); n != 0 {
		t.Fatalf("remembered-activator body emitted %d unimplemented Notes", n)
	}
	ab := cbaAbility(t, e, art)
	if !e.abilityRestricted(1, art, ab) {
		t.Fatal("Activator$ Player.IsRemembered did not bind the remembered player")
	}
	if e.abilityRestricted(0, art, ab) {
		t.Fatal("Activator$ Player.IsRemembered bound a non-remembered player")
	}
	replayCheck(t, e, cfg)
}

func TestEffectCantBeActivatedUnreadParamStaysLoud(t *testing.T) {
	t.Parallel()
	srcCard := card(t, "Name:CBA Unread Source\nManaCost:0\nTypes:Enchantment\n"+
		"SVar:STCantBeActivated:Mode$ CantBeActivated | ValidCard$ Card | ValidSA$ Activated | Bogus$ Thing\n"+
		"SVar:DBEffect:DB$ Effect | StaticAbilities$ STCantBeActivated | Duration$ Permanent\nOracle:x\n")
	artifactCard := card(t, cbaArtifactSrc)
	e, cfg := cbaEngine(t, 15, []*cards.Card{srcCard}, []*cards.Card{artifactCard})
	src := moveByName(t, e, 0, "CBA Unread Source", state.ZBattlefield)
	art := moveByName(t, e, 1, cbaArtifactName, state.ZBattlefield)
	e.pending = nil
	e.Advance()
	cbaResolveEffect(t, e, src, nil)
	if n := cbaNotes(e); n == 0 {
		t.Fatal("no unimplemented Note for an unread CantBeActivated parameter")
	}
	if e.abilityRestricted(1, art, cbaAbility(t, e, art)) {
		t.Fatal("a body with an unread key registered a restriction")
	}
	replayCheck(t, e, cfg)
}

func TestEffectCantBeActivatedUntilUntapsNeedsRemembered(t *testing.T) {
	t.Parallel()
	srcCard := card(t, "Name:CBA No Memory Source\nManaCost:0\nTypes:Enchantment\n"+
		"SVar:STCantBeActivated:Mode$ CantBeActivated | ValidCard$ Card | ValidSA$ Activated\n"+
		"SVar:DBEffect:DB$ Effect | StaticAbilities$ STCantBeActivated | Duration$ UntilTargetedUntaps\nOracle:x\n")
	artifactCard := card(t, cbaArtifactSrc)
	e, cfg := cbaEngine(t, 16, []*cards.Card{srcCard}, []*cards.Card{artifactCard})
	src := moveByName(t, e, 0, "CBA No Memory Source", state.ZBattlefield)
	art := moveByName(t, e, 1, cbaArtifactName, state.ZBattlefield)
	e.pending = nil
	e.Advance()
	cbaResolveEffect(t, e, src, nil)
	if n := cbaNotes(e); n == 0 {
		t.Fatal("UntilTargetedUntaps with no remembered object did not stay loud")
	}
	if e.abilityRestricted(1, art, cbaAbility(t, e, art)) {
		t.Fatal("UntilTargetedUntaps with no remembered object registered a permanent lock")
	}
	replayCheck(t, e, cfg)
}
