package rules

// edh-staticsA: behaviour tests for the six player/cost statics registered by
// the edh-staticsA class ticket -- GainLifeRadiation (Strong, the Brutish
// Thespian), PlayerMustAttack (Seeker of Slaanesh, Trove of Temptation),
// Devotion (Altar of the Pantheon), SetCost (Trinisphere), FlipCoinDoubler
// (Krark's Thumb) and DrawFromBottom (River Song). Each test plays the printed
// text through the real engine and asserts on state; each also pins its
// stat: key as registered, so a revert of the registration fails it.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func edhARequireRegistered(t *testing.T, key string) {
	t.Helper()
	if !effects.Supported()[key] {
		t.Fatalf("%s is not a registered primitive", key)
	}
}

// edhADrainRadiation runs seat 0's CR 728.1 rad drain over the library
// [Land, Spell, Spell] with three counters and returns the life change and the
// rad counters left.
func edhADrainRadiation(t *testing.T, strongSeat int) (lifeDelta int32, rad int32) {
	t.Helper()
	e := newSeats(t, 2)
	onBoardCard(t, e, state.PlayerID(strongSeat), corpusAlternativeCard(t, "Strong, the Brutish Thespian"))
	radiationLibrary(t, e, 0, radiationLand(t), radiationSpell(t, "Drain One"), radiationSpell(t, "Drain Two"))
	seedPlayerCounter(t, e, 0, "RAD", 3)
	before := e.G.Players[0].Life
	e.G.Step = state.StepMain1
	e.finishEnteredStep()
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("drain not on the stack: %v", e.G.Stack)
	}
	e.resolveTop()
	return e.G.Players[0].Life - before, e.G.Players[0].Counter("RAD")
}

// TestEDHStaticsAStrongGainsLifeFromRadiation: with Strong out, the two
// nonland cards the drain mills gain 1 life each instead of losing it (the
// counters still go); Strong under the opponent's control changes nothing.
func TestEDHStaticsAStrongGainsLifeFromRadiation(t *testing.T) {
	t.Parallel()
	edhARequireRegistered(t, "stat:GainLifeRadiation")
	if delta, rad := edhADrainRadiation(t, 1); delta != -2 || rad != 1 {
		t.Fatalf("opponent's Strong: life delta %d rad %d, want -2 and 1 (ValidPlayer$ You)", delta, rad)
	}
	if delta, rad := edhADrainRadiation(t, 0); delta != 2 || rad != 1 {
		t.Fatalf("own Strong: life delta %d rad %d, want +2 and 1", delta, rad)
	}
}

// TestEDHStaticsAAltarOfThePantheonDevotion: Gray Merchant of Asphodel's
// "X is your devotion to black" reads BB from itself, plus one from the Altar.
func TestEDHStaticsAAltarOfThePantheonDevotion(t *testing.T) {
	t.Parallel()
	edhARequireRegistered(t, "stat:Devotion")
	run := func(board string, p0, p1 int) {
		runInlineOracle(t, `{
	  "name": "gray-merchant-devotion",
	  "setup": {"p0": {"hand": ["Gray Merchant of Asphodel"], "battlefield": [`+board+`]}},
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Gray Merchant of Asphodel", "mana": "CCCBB"},
	    {"op": "resolve"}
	  ],
	  "expect": [{"life": {"p0": `+itoa(p0)+`, "p1": `+itoa(p1)+`}}]
	}`)
	}
	run(``, 22, 18)
	run(`"Altar of the Pantheon"`, 23, 17)
}

// TestEDHStaticsATrinisphereFloorsCosts: an untapped Trinisphere makes a {1}
// spell cost {3} and leaves a {4} spell at {4}.
func TestEDHStaticsATrinisphereFloorsCosts(t *testing.T) {
	t.Parallel()
	edhARequireRegistered(t, "stat:SetCost")
	zapSrc := "Name:Zap\nManaCost:1\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"
	fourSrc := "Name:Four Spell\nManaCost:4\nTypes:Sorcery\nOracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 71, zapSrc, fourSrc, trinisphereSrc)
	putCreature(t, e, 0, trinisphereSrc)
	four := addToHand(t, e, 0, fourSrc)
	addMana(t, e, 0, "CC")
	if castByName(t, e, 0, "Zap") != nil {
		t.Fatal("a {1} spell under Trinisphere costs {3}: two mana must not cast it")
	}
	addMana(t, e, 0, "C")
	if castByName(t, e, 0, "Zap") == nil {
		t.Fatal("three mana must cast the Trinisphere-floored {1} spell")
	}
	if castByName(t, e, 0, "Four Spell") != nil {
		t.Fatal("a {4} spell costs {4}: three mana must not cast it")
	}
	addMana(t, e, 0, "C")
	opt := castByName(t, e, 0, "Four Spell")
	if opt == nil {
		t.Fatal("four mana must cast the {4} spell")
	}
	submitChoices(t, e, opt.Index)
	if e.G.Obj(four).Zone != state.ZStack || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("Four Spell on %s with %d mana left, want the stack and an empty pool (cost stays {4})",
			e.G.Obj(four).Zone, e.G.Players[0].Pool.Total())
	}
	replayCheck(t, e, cfg)
}

// edhAMustAttackGame parks a three-seat table at seat 0's declare-attackers
// step: seat 0 has one bear that could attack seat 1 or seat 2, and seat 1
// controls the named must-attack permanent.
func edhAMustAttackGame(t *testing.T, seed uint64, permanent string) (*Engine, Config, state.ObjID) {
	t.Helper()
	pc, ok := testutil.CorpusRegistry(t).Lookup(permanent)
	if !ok {
		t.Fatalf("corpus missing %s", permanent)
	}
	bear := card(t, staticBearFixture)
	e, cfg := restrictionGame(t, seed,
		[][]*cards.Card{nil, nil, nil},
		[][]*cards.Card{{bear}, {pc}, nil})
	b := bearOnBoard(t, e, 0, bear)
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected seat 0's attackers decision, got %+v", d)
	}
	return e, cfg, b
}

// edhAPairAt is the index of the bear's pair against defender seat def.
func edhAPairAt(t *testing.T, d *decision.Decision, bear state.ObjID, def state.PlayerID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Obj == bear && o.Player == def && o.Battle == 0 {
			return o.Index
		}
	}
	t.Fatalf("no pair for the bear at seat %d: %+v", def, d.Options)
	return -1
}

// TestEDHStaticsATroveOfTemptationMustAttackController: seat 0 may not skip
// combat, nor attack only seat 2, while Trove of Temptation (seat 1) is out;
// the bot repair lands on the legal attack at seat 1.
func TestEDHStaticsATroveOfTemptationMustAttackController(t *testing.T) {
	t.Parallel()
	edhARequireRegistered(t, "stat:PlayerMustAttack")
	e, cfg, bear := edhAMustAttackGame(t, 6201, "Trove of Temptation")
	d := e.Pending()
	at1, at2 := edhAPairAt(t, d, bear, 1), edhAPairAt(t, d, bear, 2)
	if !d.Options[at1].AttackOne || d.Options[at2].AttackOne {
		t.Fatalf("AttackOne marks: at seat 1 %v, at seat 2 %v; want true/false",
			d.Options[at1].AttackOne, d.Options[at2].AttackOne)
	}
	in := func(choices ...int) decision.Intent {
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
	}
	if d.Validate(in()) == nil {
		t.Fatal("no attack at all was accepted though the bear could attack the Trove's controller")
	}
	if d.Validate(in(at2)) == nil {
		t.Fatal("attacking only seat 2 was accepted under Trove of Temptation")
	}
	if err := d.Validate(in(at1)); err != nil {
		t.Fatalf("attacking the Trove's controller rejected: %v", err)
	}
	if fit := d.FitRequired(nil); len(fit) != 1 || fit[0] != at1 || d.Validate(in(fit...)) != nil {
		t.Fatalf("FitRequired(nil) = %v, want the legal attack at seat 1 (%d)", fit, at1)
	}
	if fit := d.FitRequired([]int{at2}); d.Validate(in(fit...)) != nil {
		t.Fatalf("FitRequired([seat 2 only]) = %v is not a legal answer", fit)
	}
	// The engine's own declaration check agrees with the decision's rule.
	if err := e.Submit(in(at2)); err == nil {
		t.Fatal("engine accepted a declaration attacking only seat 2")
	}
	if err := e.Submit(in(at1)); err != nil {
		t.Fatalf("engine rejected the legal declaration: %v", err)
	}
	replayCheck(t, e, cfg)
}

// TestEDHStaticsASeekerOfSlaaneshMustAttackWithOne: Seeker of Slaanesh makes
// the opponent attack with at least one creature, at anyone.
func TestEDHStaticsASeekerOfSlaaneshMustAttackWithOne(t *testing.T) {
	t.Parallel()
	edhARequireRegistered(t, "stat:PlayerMustAttack")
	e, cfg, bear := edhAMustAttackGame(t, 6202, "Seeker of Slaanesh")
	d := e.Pending()
	at1, at2 := edhAPairAt(t, d, bear, 1), edhAPairAt(t, d, bear, 2)
	if !d.Options[at1].AttackOne || !d.Options[at2].AttackOne {
		t.Fatalf("AttackOne marks: at seat 1 %v, at seat 2 %v; want true/true",
			d.Options[at1].AttackOne, d.Options[at2].AttackOne)
	}
	in := func(choices ...int) decision.Intent {
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
	}
	if err := e.Submit(in()); err == nil {
		t.Fatal("engine accepted no attack though Seeker of Slaanesh requires one")
	}
	if err := e.Submit(in(at2)); err != nil {
		t.Fatalf("attacking seat 2 rejected under Seeker of Slaanesh: %v", err)
	}
	replayCheck(t, e, cfg)
}

const edhAFlipSrc = "Name:Thumb Flip Test\nManaCost:R\nTypes:Sorcery\n" +
	"A:SP$ FlipCoin | WinSubAbility$ DBWin | LoseSubAbility$ DBLose | SpellDescription$ Flip a coin.\n" +
	"SVar:DBWin:DB$ GainLife | Defined$ You | LifeAmount$ 3\n" +
	"SVar:DBLose:DB$ LoseLife | Defined$ You | LifeAmount$ 3\n" +
	"Oracle:Flip a coin. If you win the flip, you gain 3 life. If you lose the flip, you lose 3 life.\n"

const edhAThumbSrc = "Name:Krark's Thumb\nManaCost:2\nTypes:Legendary Artifact\n" +
	"S:Mode$ FlipCoinDoubler | ValidPlayer$ You\n" +
	"Oracle:x\n"

// edhAFlip casts the flip sorcery at seed, with Krark's Thumb out or not,
// answering the keep-which-coin ask (when posed) with answer (0 heads, 1
// tails), and reports whether the ask was posed and the life change.
func edhAFlip(t *testing.T, seed uint64, thumb bool, answer int) (asked bool, delta int32) {
	t.Helper()
	e, cfg, _ := newFixtureDeck(t, seed, edhAFlipSrc, edhAThumbSrc)
	if thumb {
		putCreature(t, e, 0, edhAThumbSrc)
	}
	addMana(t, e, 0, "R")
	opt := castByName(t, e, 0, "Thumb Flip Test")
	if opt == nil {
		t.Fatal("flip sorcery not castable")
	}
	before := e.G.Players[0].Life
	submitChoices(t, e, opt.Index)
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while resolving the flip")
		}
		pick := 0
		if d.Kind == decision.KChoose && len(d.Options) == 2 && d.Options[0].Kind == "coin" {
			asked, pick = true, answer
		} else if d.Kind == decision.KPriority {
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pick = o.Index
					break
				}
			}
		}
		submitChoices(t, e, pick)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("flip sorcery did not resolve (stack %v)", e.G.Stack)
	}
	replayCheck(t, e, cfg)
	return asked, e.G.Players[0].Life - before
}

// TestEDHStaticsAKrarksThumbFlipsTwoAndChooses: under Krark's Thumb a flip is
// two coins; when they differ the controller picks which to keep (heads wins
// the sorcery's +3, tails loses 3); with no Thumb the ask never appears.
func TestEDHStaticsAKrarksThumbFlipsTwoAndChooses(t *testing.T) {
	t.Parallel()
	edhARequireRegistered(t, "stat:FlipCoinDoubler")
	found := false
	for seed := uint64(1); seed <= 60; seed++ {
		if asked, _ := edhAFlip(t, seed, false, 0); asked {
			t.Fatalf("seed %d: the keep-a-coin ask appeared without Krark's Thumb", seed)
		}
		asked, heads := edhAFlip(t, seed, true, 0)
		if !asked {
			continue
		}
		found = true
		if _, tails := edhAFlip(t, seed, true, 1); heads != 3 || tails != -3 {
			t.Fatalf("seed %d: keeping heads gave %+d and keeping tails %+d, want +3 and -3", seed, heads, tails)
		}
		break
	}
	if !found {
		t.Fatal("no seed in 1..60 flipped two differing coins under Krark's Thumb")
	}
	// Two agreeing coins need no choice: a seed with no ask still resolves to
	// whichever side both showed, never a third outcome.
	for seed := uint64(1); seed <= 60; seed++ {
		if asked, d := edhAFlip(t, seed, true, 0); !asked && d != 3 && d != -3 {
			t.Fatalf("seed %d: agreeing coins gave %+d, want +3 or -3", seed, d)
		}
	}
}

// TestEDHStaticsARiverSongDrawsFromTheBottom: with River Song out the draw
// takes the library's last card; without her it takes the first.
func TestEDHStaticsARiverSongDrawsFromTheBottom(t *testing.T) {
	t.Parallel()
	edhARequireRegistered(t, "stat:DrawFromBottom")
	draw := func(riverSong bool) (drawn state.ObjID, lib []state.ObjID) {
		e := newSeats(t, 2)
		if riverSong {
			onBoardCard(t, e, 0, corpusAlternativeCard(t, "River Song"))
		}
		lib = radiationLibrary(t, e, 0, radiationSpell(t, "Top"), radiationSpell(t, "Middle"), radiationSpell(t, "Bottom"))
		effects.DrawFor(e, 0)
		hand := e.G.Zone(state.ZHand, 0)
		for _, id := range lib {
			if e.G.Obj(id).Zone == state.ZHand {
				drawn = id
			}
		}
		if drawn == 0 || len(e.G.Zone(state.ZLibrary, 0)) != 2 {
			t.Fatalf("draw moved nothing from the library: hand %v library %v", hand, e.G.Zone(state.ZLibrary, 0))
		}
		return drawn, lib
	}
	if drawn, lib := draw(false); drawn != lib[0] {
		t.Fatalf("without River Song the draw took %d, want the top card %d", drawn, lib[0])
	}
	if drawn, lib := draw(true); drawn != lib[2] {
		t.Fatalf("with River Song the draw took %d, want the bottom card %d", drawn, lib[2])
	}
}

// edhAAttackCeilingSrc is a whole-declaration attacker ceiling (CR 508.1j,
// Silent Arbiter's MaxAttackers$ 1 shape): no ValidDefender$, so
// combat.MaxAttackers reads it as the global cap on seat 0's declaration.
const edhAAttackCeilingSrc = "Name:Arbiter\nManaCost:4\nTypes:Artifact\n" +
	"S:Mode$ AttackRestrict | MaxAttackers$ 1\nOracle:x\n"

// TestEDHStaticsAPlayerMustAttackUnenforcedUnderCeiling: with an attacker
// ceiling in force (the decision's Max is below the offered option count) the
// PlayerMustAttack marking leaves the duty unmarked, the unenforcement the tap
// pool and the per-defender cap already take: a Max-full declaration cannot
// make room for a marked pair on a creature it did not declare, so a repair
// that cannot hand back a legal answer would leave the deterministic bot
// re-deriving forever. The duty stays enforced whenever no ceiling binds
// (TestEDHStaticsATroveOfTemptationMustAttackController).
func TestEDHStaticsAPlayerMustAttackUnenforcedUnderCeiling(t *testing.T) {
	t.Parallel()
	edhARequireRegistered(t, "stat:PlayerMustAttack")
	pc, ok := testutil.CorpusRegistry(t).Lookup("Trove of Temptation")
	if !ok {
		t.Fatal("corpus missing Trove of Temptation")
	}
	bear := card(t, staticBearFixture)
	e, cfg := restrictionGame(t, 6203,
		[][]*cards.Card{nil, nil, nil},
		[][]*cards.Card{{bear, card(t, edhAAttackCeilingSrc)}, {pc}, nil})
	b := bearOnBoard(t, e, 0, bear)
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected seat 0's attackers decision, got %+v", d)
	}
	if d.Max != 1 || len(d.Options) != 2 {
		t.Fatalf("precondition: Max %d over %d options, want the ceiling at 1 over 2", d.Max, len(d.Options))
	}
	at1, at2 := edhAPairAt(t, d, b, 1), edhAPairAt(t, d, b, 2)
	if d.Options[at1].AttackOne || d.Options[at2].AttackOne {
		t.Fatalf("AttackOne marks under the ceiling: at seat 1 %v, at seat 2 %v; want false/false",
			d.Options[at1].AttackOne, d.Options[at2].AttackOne)
	}
	in := func(choices ...int) decision.Intent {
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
	}
	if err := e.Submit(in(at2)); err != nil {
		t.Fatalf("attacking only seat 2 rejected under the ceiling: %v", err)
	}
	if fit := d.FitRequired(nil); d.Validate(in(fit...)) != nil {
		t.Fatalf("FitRequired(nil) = %v is not a legal answer (the repair must never deadlock)", fit)
	}
	if fit := d.FitRequired([]int{at2}); d.Validate(in(fit...)) != nil {
		t.Fatalf("FitRequired([seat 2 only]) = %v is not a legal answer", fit)
	}
	replayCheck(t, e, cfg)
}

// TestEDHStaticsAValidPlayerIsYou pins the assumption effects' youStaticCount
// makes without reading it: every corpus carrier of GainLifeRadiation,
// Devotion, FlipCoinDoubler and DrawFromBottom is scoped ValidPlayer$ You, so
// the scan over the controller's own permanents is the whole rule. A card that
// names another player fails here, before the engine would apply it to You.
func TestEDHStaticsAValidPlayerIsYou(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	seen := map[string]int{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, st := range f.Statics {
				switch st.Mode {
				case effects.ModeGainLifeRadiation, effects.ModeDevotion,
					effects.ModeFlipCoinDoubler, effects.ModeDrawFromBottom:
					seen[st.Mode]++
					if got := st.ParamStr(cards.PKValidPlayer); got != "You" {
						t.Errorf("%s: %s names ValidPlayer$ %q, want You", f.Name, st.Mode, got)
					}
				}
			}
		}
	}
	for _, mode := range []string{effects.ModeGainLifeRadiation, effects.ModeDevotion,
		effects.ModeFlipCoinDoubler, effects.ModeDrawFromBottom} {
		if seen[mode] == 0 {
			t.Errorf("no corpus card carries %s: the corpus did not load or the mode is gone", mode)
		}
	}
}
