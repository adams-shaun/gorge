package rules

// The rv2c cost-modifier parity task: RaiseCost/ReduceCost/SetCost read the
// same parameters Forge's CostAdjustment reads (ValidSpell$, SVar Amount$,
// Color$, IgnoreGeneric$, MinMana$, EffectZone$ host-self statics, IsPresent$,
// RaiseTo$), and ParseCost prices the monocolour-hybrid (twobrid),
// hybrid-Phyrexian and snow symbols as real alternative payments. Every
// fixture below embeds the real Forge script line(s) of the card it probes,
// copied from the corpus (never a committed .txt — the licensing rule).

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

const steelshaperSrc = "Name:Auriok Steelshaper\nManaCost:1 W\nTypes:Creature Human Soldier\nPT:1/1\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 1 | Description$ Equip costs you pay cost {1} less.\n" +
	"Oracle:Equip costs you pay cost {1} less.\n"

const equiperSrc = "Name:Kyoshi Battle Fan\nManaCost:3\nTypes:Artifact Creature Kitsune\nPT:1/3\n" +
	"K:Equip:2\nOracle:Equip {2}\n"

const thaliaRv2cSrc = "Name:Thalia, Guardian of Thraben\nManaCost:1 W\nTypes:Legendary Creature Human Soldier\nPT:2/1\n" +
	"S:Mode$ RaiseCost | ValidCard$ Card.nonCreature | Type$ Spell | Amount$ 1 | Description$ Noncreature spells cost {1} more to cast.\n" +
	"Oracle:Noncreature spells cost {1} more to cast.\n"

const rakdosSrc = "Name:Rakdos, Lord of Riots\nManaCost:B B R R\nTypes:Legendary Creature Demon\nPT:6/6\n" +
	"S:Mode$ ReduceCost | ValidCard$ Creature | Type$ Spell | Activator$ You | Amount$ X | Description$ Creature spells you cast cost {1} less to cast for each 1 life your opponents have lost this turn.\n" +
	"SVar:X:Count$LifeOppsLostThisTurn\n" +
	"Oracle:Creature spells you cast cost {1} less to cast for each 1 life your opponents have lost this turn.\n"

const heraldRv2cSrc = "Name:Herald of War\nManaCost:3 W W\nTypes:Creature Angel\nPT:3/3\n" +
	"S:Mode$ ReduceCost | ValidCard$ Angel,Human | Type$ Spell | Activator$ You | Amount$ X | Description$ Angels and Humans you cast cost 1 less for each +1/+1 counter on CARDNAME.\n" +
	"SVar:X:Count$CardCounters.P1P1\n" +
	"Oracle:Angel spells and Human spells you cast cost {1} less to cast for each +1/+1 counter on Herald of War.\n"

const ghaltaSrc = "Name:Ghalta, Primal Hunger\nManaCost:10 G G\nTypes:Legendary Creature Elder Dinosaur\nPT:12/12\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ X | EffectZone$ All | Description$ This spell costs {X} less to cast, where X is the total power of creatures you control.\n" +
	"SVar:X:Count$Valid Creature.YouCtrl$CardPower\n" +
	"Oracle:This spell costs {X} less to cast, where X is the total power of creatures you control.\n"

const hydraRv2cSrc = "Name:Khalni Hydra\nManaCost:G G G G G G G G\nTypes:Creature Hydra\nPT:8/8\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Color$ G | Amount$ X | EffectZone$ All | Description$ This spell costs {G} less to cast for each green creature you control.\n" +
	"SVar:X:Count$Valid Creature.Green+YouCtrl\n" +
	"Oracle:This spell costs {G} less to cast for each green creature you control.\n"

const heartstoneSrc = "Name:Heartstone\nManaCost:3\nTypes:Artifact\n" +
	"S:Mode$ ReduceCost | ValidCard$ Creature | Type$ Ability | Amount$ 1 | MinMana$ 1 | AffectedZone$ Battlefield | Description$ Activated abilities of creatures cost {1} less to activate. This effect can't reduce the mana in that cost to less than one mana.\n" +
	"Oracle:Activated abilities of creatures cost {1} less to activate. This effect can't reduce the mana in that cost to less than one mana.\n"

const trinisphereSrc = "Name:Trinisphere\nManaCost:3\nTypes:Artifact\n" +
	"S:Mode$ SetCost | ValidCard$ Card | Type$ Spell | Amount$ 3 | RaiseTo$ True | IsPresent$ Card.Self+untapped | Description$ As long as CARDNAME is untapped, each spell that would cost less than three mana to cast costs three mana to cast.\n" +
	"Oracle:As long as Trinisphere is untapped, each spell that would cost less than three mana to cast costs three mana to cast.\n"

const spectralProcessionSrc = "Name:Spectral Procession\nManaCost:2W 2W 2W\nTypes:Sorcery\n" +
	"A:SP$ Token | TokenAmount$ 3 | TokenScript$ w_1_1_spirit_flying | TokenOwner$ You | SpellDescription$ Create three 1/1 white Spirit creature tokens with flying.\n" +
	"Oracle:Create three 1/1 white Spirit creature tokens with flying.\n"

const icehideGolemSrc = "Name:Icehide Golem\nManaCost:S\nTypes:Snow Artifact Creature Golem\nPT:2/2\n" +
	"Oracle:({S} can be paid with one mana from a snow source.)\n"

const snowWastesSrc = "Name:Snow-Covered Wastes\nManaCost:no cost\nTypes:Basic Snow Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\n" +
	"Oracle:{T}: Add {C}.\n"

const borealCentaurSrc = "Name:Boreal Centaur\nManaCost:1 G\nTypes:Snow Creature Centaur Warrior\nPT:2/2\n" +
	"A:AB$ Pump | Cost$ S | Defined$ Self | NumAtt$ +1 | NumDef$ +1 | ActivationLimit$ 1 | SpellDescription$ CARDNAME gets +1/+1 until end of turn. Activate only once each turn.\n" +
	"Oracle:{S}: Boreal Centaur gets +1/+1 until end of turn. Activate only once each turn.\n"

const ajaniSleeperSrc = "Name:Ajani Sleeper Agent\nManaCost:1 G GWP W\nTypes:Legendary Planeswalker Ajani\nLoyalty:4\n" +
	"Oracle:Compleated\n"

const testBearSrc = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// castByName returns seat p's cast option for the named card.
func castByName(t *testing.T, e *Engine, p state.PlayerID, name string) *decision.Option {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority for %s: %+v", name, d)
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj != 0 && e.G.Obj(o.Obj) != nil && e.G.Obj(o.Obj).Face() != nil &&
			e.G.Obj(o.Obj).Face().Name == name {
			return &o
		}
	}
	return nil
}

// reduceOf returns the single evaluated ReduceCost amount a cast of id by p
// draws, failing if there is not exactly one reduction.
func reduceOf(t *testing.T, e *Engine, p state.PlayerID, id state.ObjID) int32 {
	t.Helper()
	mods := e.costModifiers(p, id, spellScope(""))
	var n int32
	for _, red := range mods.reduces {
		n += red.generic + red.colored.Total()
	}
	return n
}

// TestAuriokSteelshaperDiscountsOnlyEquip pins ValidSpell$: the reduction
// applies to the Equip activation (whose expansion carries Keyword$ Equip)
// and to nothing else — a {1}{W} creature cast is NOT discounted even though
// the static's ValidCard$ Card would admit every card.
func TestAuriokSteelshaperDiscountsOnlyEquip(t *testing.T) {
	e, cfg, fan := newFixtureDeck(t, 61, equiperSrc, thaliaRv2cSrc, steelshaperSrc)
	if got := e.AbilityCosts(0, fan); len(got) != 1 || got[0] != "2" {
		t.Fatalf("equip costs before the Steelshaper: %v, want [2]", got)
	}
	putCreature(t, e, 0, equiperSrc)
	putCreature(t, e, 0, steelshaperSrc)
	if got := e.AbilityCosts(0, fan); len(got) != 1 || got[0] != "1" {
		t.Fatalf("equip costs with the Steelshaper out: %v, want [1]", got)
	}
	// And the discount is real money: one colourless mana activates the
	// two-mana equip.
	addMana(t, e, 0, "C")
	if kinds(e.legalActions(0))["ability"] != 1 {
		t.Fatal("the discounted equip should be activatable with one mana")
	}
	replayCheck(t, e, cfg)

	// The probe: Thalia {1}{W} from a pool of exactly {W} — no discount.
	e2, _, _ := newFixtureDeck(t, 62, thaliaRv2cSrc, steelshaperSrc)
	putCreature(t, e2, 0, steelshaperSrc)
	addMana(t, e2, 0, "W")
	if castByName(t, e2, 0, "Thalia, Guardian of Thraben") != nil {
		t.Fatal("Steelshaper must not discount a plain creature cast (ValidSpell$ Activated.Equip)")
	}
}

// TestRakdosReduceCostEvaluatesTheLifeSVar pins the SVar Amount$: the
// reduction is Count$LifeOppsLostThisTurn — 0 with no life lost (the old
// parseAmount fallback made it 1), 2 after an opponent lost 2 life.
func TestRakdosReduceCostEvaluatesTheLifeSVar(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 63, testBearSrc, rakdosSrc)
	_ = putCreature(t, e, 0, rakdosSrc)
	var bear state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(id).Face().Name == "Bear" {
			bear = id
		}
	}
	if bear == 0 {
		t.Fatal("bear not in hand")
	}
	addMana(t, e, 0, "G")
	if got := reduceOf(t, e, 0, bear); got != 0 {
		t.Fatalf("reduction with no life lost = %d, want 0", got)
	}
	if castByName(t, e, 0, "Bear") != nil {
		t.Fatal("Bear {1}{G} with only {G} must not be castable when the reduction is 0")
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -2})
	e.priorityRound()
	if got := reduceOf(t, e, 0, bear); got != 2 {
		t.Fatalf("reduction after 2 life lost = %d, want 2", got)
	}
	if castByName(t, e, 0, "Bear") == nil {
		t.Fatal("Bear should be castable now: the reduction 2 covers the generic")
	}
	replayCheck(t, e, cfg)
}

// TestHeraldOfWarReducesByItsCounters pins Count$CardCounters: the reduction
// tracks the +1/+1 counters ON the herald itself, live as they accumulate.
func TestHeraldOfWarReducesByItsCounters(t *testing.T) {
	angelSrc := "Name:Angel\nManaCost:3 W\nTypes:Creature Angel\nPT:2/2\nOracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 64, angelSrc, heraldRv2cSrc)
	herald := putCreature(t, e, 0, heraldRv2cSrc)
	var angel state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(id).Face().Name == "Angel" {
			angel = id
		}
	}
	if angel == 0 {
		t.Fatal("angel not in hand")
	}
	addMana(t, e, 0, "W")
	if got := reduceOf(t, e, 0, angel); got != 0 {
		t.Fatalf("reduction with no counters = %d, want 0", got)
	}
	if castByName(t, e, 0, "Angel") != nil {
		t.Fatal("Angel {3}{W} with only {W} must not be castable at 0 counters")
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: herald, Counter: "P1P1", Amount: 3})
	e.priorityRound()
	if got := reduceOf(t, e, 0, angel); got != 3 {
		t.Fatalf("reduction with 3 counters = %d, want 3", got)
	}
	if castByName(t, e, 0, "Angel") == nil {
		t.Fatal("Angel should be castable: three counters cover the generic 3")
	}
	replayCheck(t, e, cfg)
}

// TestGhaltaSelfReductionFromHand pins the host-self static: Ghalta's own
// EffectZone$ All reduction is live while the card is still IN HAND, so 18
// power of creatures turns {10}{G}{G} into {G}{G}.
func TestGhaltaSelfReductionFromHand(t *testing.T) {
	bigSrc := "Name:Big\nManaCost:6 G\nTypes:Creature Beast\nPT:9/9\nOracle:x\n"
	e, cfg, ghalta := newFixtureDeck(t, 65, ghaltaSrc, bigSrc, bigSrc)
	putCreature(t, e, 0, bigSrc)
	putCreature(t, e, 0, bigSrc)
	addMana(t, e, 0, "GG")
	if got := reduceOf(t, e, 0, ghalta); got != 18 {
		t.Fatalf("Ghalta reduction with 18 power out = %d, want 18", got)
	}
	opt := castByName(t, e, 0, "Ghalta, Primal Hunger")
	if opt == nil {
		t.Fatal("Ghalta must be castable from hand with GG and 18 power out")
	}
	submitChoices(t, e, opt.Index)
	if e.G.Obj(ghalta).Zone != state.ZStack {
		t.Fatalf("Ghalta on %s, want stack", e.G.Obj(ghalta).Zone)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("pool after the cast = %d, want 0", e.G.Players[0].Pool.Total())
	}
	replayCheck(t, e, cfg)
}

// TestColorReduceRemovesColoredPips pins Color$: Khalni Hydra's reduction is
// one GREEN pip per green creature (SVar count), not a generic discount.
func TestColorReduceRemovesColoredPips(t *testing.T) {
	greenSrc := "Name:Elvish\nManaCost:G\nTypes:Creature Elf Druid\nPT:1/1\nOracle:x\n"
	e, cfg, hydra := newFixtureDeck(t, 66, hydraRv2cSrc, greenSrc, greenSrc)
	putCreature(t, e, 0, greenSrc)
	putCreature(t, e, 0, greenSrc)
	addMana(t, e, 0, "GGGGGG")
	if got := reduceOf(t, e, 0, hydra); got != 2 {
		t.Fatalf("Khalni Hydra reduction with 2 green creatures = %d, want 2 (green pips)", got)
	}
	mods := e.costModifiers(0, hydra, spellScope(""))
	if mods.reduces[0].colored[state.MG] != 2 || mods.reduces[0].generic != 0 {
		t.Fatalf("the reduction must name 2 green pips, got %+v", mods.reduces[0])
	}
	opt := castByName(t, e, 0, "Khalni Hydra")
	if opt == nil {
		t.Fatal("Khalni Hydra must be castable with 6 green after a 2-pip colour reduction")
	}
	submitChoices(t, e, opt.Index)
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("pool after the cast = %d, want 0", e.G.Players[0].Pool.Total())
	}
	replayCheck(t, e, cfg)
}

// TestMinManaFloorKeepsOneMana pins MinMana$: Heartstone's reduction can't
// reduce a creature ability's activation cost below one mana.
func TestMinManaFloorKeepsOneMana(t *testing.T) {
	pumperSrc := "Name:Pumper\nManaCost:1 G\nTypes:Creature Beast\nPT:1/1\n" +
		"A:AB$ Pump | Cost$ 1 | Defined$ Self | NumAtt$ +1 | SpellDescription$ +1/+0.\n" +
		"Oracle:x\n"
	e, _, pumper := newFixtureDeck(t, 67, pumperSrc, heartstoneSrc)
	putCreature(t, e, 0, pumperSrc)
	putCreature(t, e, 0, heartstoneSrc)
	if got := e.AbilityCosts(0, pumper); len(got) != 1 || got[0] != "1" {
		t.Fatalf("Heartstone's reduction with MinMana$ 1 leaves %v, want [1]", got)
	}
	e.G.Players[0].Pool[state.MC] = 0
	e.priorityRound()
	if kinds(e.legalActions(0))["ability"] != 0 {
		t.Fatal("the floored ability still costs one mana; with an empty pool it must not be offered")
	}
	e.G.Players[0].Pool[state.MC] = 1
	e.priorityRound()
	if kinds(e.legalActions(0))["ability"] != 1 {
		t.Fatal("the floored ability should be activatable with one mana")
	}
}

// TestTrinisphereSetCostFloor pins SetCost (RaiseTo$ True + IsPresent$): an
// untapped Trinisphere raises every cheaper spell's total mana to 3; a tapped
// one raises nothing.
func TestTrinisphereSetCostFloor(t *testing.T) {
	zapSrc := "Name:Zap\nManaCost:1\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"
	e, cfg, zap := newFixtureDeck(t, 68, zapSrc, trinisphereSrc)
	putCreature(t, e, 0, trinisphereSrc)
	addMana(t, e, 0, "C")
	if castByName(t, e, 0, "Zap") != nil {
		t.Fatal("a {1} spell under an untapped Trinisphere costs {3}; one mana must not be enough")
	}
	addMana(t, e, 0, "CC")
	opt := castByName(t, e, 0, "Zap")
	if opt == nil {
		t.Fatal("the Trinisphere-raised Zap should be castable with 3 mana")
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Zap's target decision: %+v", d)
	}
	submitChoices(t, e, 0)
	if e.G.Obj(zap).Zone != state.ZStack {
		t.Fatalf("Zap on %s, want stack", e.G.Obj(zap).Zone)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("pool after paying the raised cost = %d, want 0", e.G.Players[0].Pool.Total())
	}
	replayCheck(t, e, cfg)

	// A tapped Trinisphere stops applying (IsPresent$ Card.Self+untapped).
	e3, _, _ := newFixtureDeck(t, 69, zapSrc, trinisphereSrc)
	tri3 := putCreature(t, e3, 0, trinisphereSrc)
	e3.emit(events.Event{Kind: events.Tap, Obj: tri3})
	addMana(t, e3, 0, "C")
	if castByName(t, e3, 0, "Zap") == nil {
		t.Fatal("a tapped Trinisphere must not raise the cost")
	}
}

// TestSpectralProcessionTwobridPayments pins the monocolour hybrid: each
// {2/W} pip is announced as one decision, payable by one white OR two
// generic (CR 107.4e).
func TestSpectralProcessionTwobridPayments(t *testing.T) {
	// All white: three announcements, each paid white.
	e, cfg, proc := newFixtureDeck(t, 70, spectralProcessionSrc)
	addMana(t, e, 0, "WWW")
	opt := castByName(t, e, 0, "Spectral Procession")
	if opt == nil {
		t.Fatal("Spectral Procession must be payable with WWW")
	}
	submitChoices(t, e, opt.Index)
	for i := 0; i < 3; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "pay_W" {
			t.Fatalf("pip %d: %+v", i, d)
		}
		if len(d.Options) != 2 {
			t.Fatalf("pip %d: options %+v, want pay_W and pay_generic", i, d.Options)
		}
		submitChoices(t, e, 0)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("pool after WWW payment = %d, want 0", e.G.Players[0].Pool.Total())
	}
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(proc).Zone != state.ZGraveyard {
		t.Fatalf("resolved procession on %s, want graveyard", e.G.Obj(proc).Zone)
	}
	replayCheck(t, e, cfg)

	// Six generic (no white at all): each pip announced, each paid generic.
	e2, cfg2, proc2 := newFixtureDeck(t, 71, spectralProcessionSrc)
	addMana(t, e2, 0, "CCCCCC")
	opt2 := castByName(t, e2, 0, "Spectral Procession")
	if opt2 == nil {
		t.Fatal("Spectral Procession must be payable with six generic mana")
	}
	submitChoices(t, e2, opt2.Index)
	for i := 0; i < 3; i++ {
		d := e2.Pending()
		if d == nil || d.Kind != decision.KChoose || len(d.Options) != 1 || d.Options[0].Kind != "pay_generic" {
			t.Fatalf("generic pip %d: %+v", i, d)
		}
		submitChoices(t, e2, 0)
	}
	if e2.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("pool after generic payment = %d, want 0", e2.G.Players[0].Pool.Total())
	}
	passUntilStackEmpty(t, e2, 20)
	if e2.G.Obj(proc2).Zone != state.ZGraveyard {
		t.Fatalf("resolved procession on %s, want graveyard", e2.G.Obj(proc2).Zone)
	}
	replayCheck(t, e2, cfg2)

	// Two white and four generic: a mixed payment also reaches the tokens.
	e3, _, _ := newFixtureDeck(t, 72, spectralProcessionSrc)
	addMana(t, e3, 0, "WCCCC")
	if castByName(t, e3, 0, "Spectral Procession") == nil {
		t.Fatal("the mixed W+4C pool should pay one pip white and two generic")
	}
	// Three mana short of three pips: W pays one, and only two generic are
	// left for the other two {2/W} faces — not offered at all.
	e4, _, _ := newFixtureDeck(t, 73, spectralProcessionSrc)
	addMana(t, e4, 0, "WCC")
	if castByName(t, e4, 0, "Spectral Procession") != nil {
		t.Fatal("three mana cannot pay three {2/W} pips")
	}
}

// TestSnowCostPaidOnlyBySnowMana pins {S} (CR 107.4h): the pip is payable
// only by a mana a snow permanent produced, and paying it consumes the snow
// unit (the pool slot and the parallel tally together).
func TestSnowCostPaidOnlyBySnowMana(t *testing.T) {
	e, cfg, golem := newFixtureDeck(t, 74, icehideGolemSrc, snowWastesSrc)
	wastes := moveSeeded(t, e, 0, snowWastesSrc, state.ZBattlefield)
	addMana(t, e, 0, "C")
	if castByName(t, e, 0, "Icehide Golem") != nil {
		t.Fatal("a plain colourless mana must not pay {S}")
	}
	// Tap the snow land for its colourless snow mana (the effMana tag).
	e.emit(events.Event{Kind: events.Tap, Obj: wastes})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "SC", Amount: 1})
	e.priorityRound()
	if e.G.Players[0].Pool[state.MC] != 2 || e.G.Players[0].Snow[state.MC] != 1 {
		t.Fatalf("pool %+v snow %+v after the snow tap", e.G.Players[0].Pool, e.G.Players[0].Snow)
	}
	opt := castByName(t, e, 0, "Icehide Golem")
	if opt == nil {
		t.Fatal("the snow colourless mana must pay {S}")
	}
	submitChoices(t, e, opt.Index)
	if e.G.Obj(golem).Zone != state.ZStack {
		t.Fatalf("golem on %s, want stack", e.G.Obj(golem).Zone)
	}
	// The snow unit was consumed; the plain unit remains.
	if e.G.Players[0].Pool[state.MC] != 1 || e.G.Players[0].Snow[state.MC] != 0 {
		t.Fatalf("after paying {S}: pool %+v snow %+v, want one plain colourless left",
			e.G.Players[0].Pool, e.G.Players[0].Snow)
	}
	replayCheck(t, e, cfg)

	// The same pip gates an ability activation: Boreal Centaur's {S} pump.
	e2, cfg2, _ := newFixtureDeck(t, 75, borealCentaurSrc)
	centaur := putCreature(t, e2, 0, borealCentaurSrc)
	addMana(t, e2, 0, "G")
	e2.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "SG", Amount: 1})
	e2.priorityRound()
	if e2.G.Players[0].Pool[state.MG] != 2 || e2.G.Players[0].Snow[state.MG] != 1 {
		t.Fatalf("centaur pool %+v snow %+v", e2.G.Players[0].Pool, e2.G.Players[0].Snow)
	}
	if kinds(e2.legalActions(0))["ability"] != 1 {
		t.Fatal("the {S} pump should be activatable with snow mana in the pool")
	}
	// And without the snow unit the same pool cannot pay it.
	e3, _, _ := newFixtureDeck(t, 76, borealCentaurSrc)
	addMana(t, e3, 0, "G")
	if kinds(e3.legalActions(0))["ability"] != 0 {
		t.Fatal("plain green mana must not pay a {S} activation")
	}
	_ = centaur
	replayCheck(t, e2, cfg2)
}

// TestHybridPhyrexianCostsParseAndPay pins the three-part compleated symbol:
// one pip payable by either colour or two life.
func TestHybridPhyrexianCostsParseAndPay(t *testing.T) {
	c := ParseCost("1 G GWP W")
	if c.Generic != 1 || c.Colored[state.MG] != 1 || c.Colored[state.MW] != 1 ||
		len(c.HybridPhyrexian) != 1 || c.HybridPhyrexian[0] != (HybridPhyrexian{A: 'G', B: 'W'}) {
		t.Fatalf("ParseCost(\"1 G GWP W\") = %+v", c)
	}
	if !c.payable(pool(1, 0, 0, 0, 2, 1), state.Mana{}, 0) {
		t.Fatal("W + GG + a green face + generic must pay the cost with no life")
	}
	// Two life pays the compleated face when no colour has a spare unit left
	// for it: the colour pips take their own units, the pip goes to life, and
	// the generic lands on the colourless unit.
	pay, ok := c.resolveMana(pool(1, 0, 0, 0, 1, 1), state.Mana{}, 10)
	if !ok || pay.lifeSpent != 2 {
		t.Fatalf("W+G+C with life must pay the compleated pip with two life: %+v ok=%v", pay, ok)
	}
	if !c.payable(pool(1, 0, 0, 0, 1, 1), state.Mana{}, 10) {
		t.Fatal("the same pool is payable with life offered")
	}
	if c.payable(pool(1, 0, 0, 0, 1, 1), state.Mana{}, 1) {
		t.Fatal("one life is not enough for the compleated face")
	}
	// Pool with no white at all, life offered: the pip goes to life.
	c2 := ParseCost("GWP")
	pay2, ok2 := c2.resolveMana(state.Mana{}, state.Mana{}, 2)
	if !ok2 || pay2.lifeSpent != 2 {
		t.Fatalf("GWP with an empty pool and 2 life = %+v ok=%v, want two life", pay2, ok2)
	}
	if !c2.payable(state.Mana{}, state.Mana{}, 2) {
		t.Fatal("GWP must be payable with exactly two life")
	}
	if c2.CanPay(state.Mana{}) {
		t.Fatal("GWP is not pool-payable by nothing")
	}
}

// TestAffectedZoneScopesAbilityModifier pins AffectedZone$ on a Type$
// Ability reduction: a static scoped to Graveyard abilities must not reach a
// battlefield ability.
func TestAffectedZoneScopesAbilityModifier(t *testing.T) {
	pumperSrc := "Name:Pumper\nManaCost:1 G\nTypes:Creature Beast\nPT:1/1\n" +
		"A:AB$ Pump | Cost$ 2 | Defined$ Self | NumAtt$ +1 | SpellDescription$ +1/+0.\n" +
		"Oracle:x\n"
	convergenceSrc := "Name:Convergence\nManaCost:3\nTypes:Artifact\n" +
		"S:Mode$ ReduceCost | ValidCard$ Card.YouOwn | Type$ Ability | Amount$ 2 | MinMana$ 1 | AffectedZone$ Graveyard | Description$ Activated abilities of cards in your graveyard cost {2} less to activate.\n" +
		"Oracle:x\n"
	e, _, pumper := newFixtureDeck(t, 77, pumperSrc, convergenceSrc)
	putCreature(t, e, 0, convergenceSrc)
	// The battlefield ability is untouched by the graveyard-scoped static.
	if got := e.AbilityCosts(0, pumper); len(got) != 1 || got[0] != "2" {
		t.Fatalf("battlefield ability costs under a graveyard-scoped reducer: %v, want [2]", got)
	}
}

// TestCheckSVarGatesTheReduction pins CheckSVar$/SVarCompare$ (Monk Class's
// real "second spell" line): the reduction holds only when the SVar
// evaluation matches the comparison, and the count itself is the
// Count$ThisTurnCast_Card.YouCtrl log fold.
func TestCheckSVarGatesTheReduction(t *testing.T) {
	dualcastSrc := "Name:Dualcast\nManaCost:2\nTypes:Enchantment\n" +
		"S:Mode$ ReduceCost | ValidCard$ Card | Type$ Spell | Activator$ You | Amount$ 1 | CheckSVar$ YouCastThisTurn | SVarCompare$ EQ1 | Description$ The second spell you cast each turn costs {1} less to cast.\n" +
		"SVar:YouCastThisTurn:Count$ThisTurnCast_Card.YouCtrl\n" +
		"Oracle:The second spell you cast each turn costs {1} less to cast.\n"
	boltSrc := "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 78, testBearSrc, dualcastSrc, boltSrc)
	putCreature(t, e, 0, dualcastSrc)
	var bear state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		switch e.G.Obj(id).Face().Name {
		case "Bear":
			bear = id
		}
	}
	addMana(t, e, 0, "RG")
	if got := reduceOf(t, e, 0, bear); got != 0 {
		t.Fatalf("first-spell reduction = %d, want 0 (YouCastThisTurn is 0, not EQ1)", got)
	}
	// Cast the counted first spell for real (Bolt, targeted and resolved).
	opt := castByName(t, e, 0, "Bolt")
	if opt == nil {
		t.Fatal("Bolt should be castable with {R}")
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Bolt's target decision: %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	addMana(t, e, 0, "G")
	if got := reduceOf(t, e, 0, bear); got != 1 {
		t.Fatalf("second-spell reduction = %d, want 1", got)
	}
	if castByName(t, e, 0, "Bear") == nil {
		t.Fatal("Bear should be castable: the discount 1 covers the generic")
	}
	replayCheck(t, e, cfg)
}

// TestConditionPlayerTurnGatesTheReduction pins Condition$ PlayerTurn /
// NotPlayerTurn (Discontinuity's real line): the reduction applies only on
// the caster's own turn.
func TestConditionPlayerTurnGatesTheReduction(t *testing.T) {
	discontinuitySrc := "Name:Discontinuity\nManaCost:2 U U\nTypes:Instant\n" +
		"S:Mode$ ReduceCost | Condition$ PlayerTurn | ValidCard$ Card.Self | Amount$ 1 | Color$ 2 U U | Type$ Spell | EffectZone$ All | Description$ During your turn, CARDNAME costs {2}{U}{U} less to cast.\n" +
		"Oracle:During your turn, CARDNAME costs {2}{U}{U} less to cast.\n"
	e, _, disc := newFixtureDeck(t, 79, discontinuitySrc)
	addMana(t, e, 0, "UU")
	e.G.Active = 0
	if got := reduceOf(t, e, 0, disc); got != 3 {
		t.Fatalf("reduction on your turn = %d, want 3 ({2}{U}{U})", got)
	}
	e.G.Active = 1
	if got := reduceOf(t, e, 0, disc); got != 0 {
		t.Fatalf("reduction on the opponent's turn = %d, want 0", got)
	}
	e.G.Active = 0
}

// TestRaiseCostManaShapeAddsPips pins the RaiseCost Cost\$ raise (Andradite
// Leech's real line): the additional cost is whole mana — the raised spell
// owes the extra {B} pip, and a non-black spell is untouched.
func TestRaiseCostManaShapeAddsPips(t *testing.T) {
	blightSrc := "Name:Blight\nManaCost:B\nTypes:Sorcery\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"
	leechSrc := "Name:Andradite Leech\nManaCost:2 B\nTypes:Creature Leech\nPT:2/2\n" +
		"A:AB$ Pump | Cost$ B | Defined$ Self | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ CARDNAME gets +1/+1 until end of turn.\n" +
		"S:Mode$ RaiseCost | ValidCard$ Card.Black | Activator$ You | Type$ Spell | Cost$ B | Description$ Black spells you cast cost {B} more to cast.\n" +
		"Oracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 80, blightSrc, leechSrc)
	putCreature(t, e, 0, leechSrc)
	addMana(t, e, 0, "B")
	if castByName(t, e, 0, "Blight") != nil {
		t.Fatal("a {B} spell raised by {B} needs two black; one must not be enough")
	}
	addMana(t, e, 0, "B")
	opt := castByName(t, e, 0, "Blight")
	if opt == nil {
		t.Fatal("the raised {B}{B} should be castable with two black")
	}
	submitChoices(t, e, opt.Index)
	if d := e.Pending(); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Blight's target decision: %+v", d)
	}
	submitChoices(t, e, 0)
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("pool after paying the raised cost = %d, want 0", e.G.Players[0].Pool.Total())
	}
	replayCheck(t, e, cfg)
	// A non-black spell is untouched by the ValidCard$ Card.Black raise.
	leech2Src := "Name:Andradite Leech\nManaCost:2 B\nTypes:Creature Leech\nPT:2/2\n" +
		"S:Mode$ RaiseCost | ValidCard$ Card.Black | Activator$ You | Type$ Spell | Cost$ B | Description$ Black spells you cast cost {B} more to cast.\n" +
		"Oracle:x\n"
	e2, _, _ := newFixtureDeck(t, 81, "Name:Filler\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n", leech2Src)
	putCreature(t, e2, 0, leech2Src)
	addMana(t, e2, 0, "R")
	if castByName(t, e2, 0, "Filler") == nil {
		t.Fatal("a red spell is not black: the {B} raise must not touch it")
	}
}
