// altcast_test.go — one proof test per alternative-cost keyword, each driven
// by a real card script from the ticket's table (the corpus .cards/cardsfolder
// entry, never a name-sharing fixture): Nulldrifter and Fury for evoke (mana
// and ExileFromHand costs), Ragavan for dash, Impulsive Pilferer for encore,
// Cyclonic Rift for overload, Timeline Culler for warp, Fiery Temper
// (discarded by Mind Rot) for madness, and Redirect Lightning for the
// AlternateAdditionalCost either-or.
//
// The fixtures that are NOT the card under test (the red card Fury's evoke
// exiles, the Bear Cyclonic Rift's overloaded form bounces, the Mind Rot
// discard fuel) are freely authored fixtures, never corpus .txt, per the
// licensing rule.
//
// The encore test compiles its card FRESH from cardsfolder (cards.OpenCorpus
// on a temp copy) rather than reading the shared ir.gob.gz cache: the cache's
// staleness rule is keyed on cards.lock, which a Go-side keyword-expansion
// change (cards/keywords.go) does not touch, so a cached registry decodes
// Impulsive Pilferer without the K:Encore expansion the parser now applies.
package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

const (
	altRedSrc = "Name:Ember\nManaCost:R\nTypes:Instant\n" +
		"A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1 | SpellDescription$ Deal 1 damage.\n"
	altBearSrc = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
)

// altCostEngine builds a two-seat engine: seat 0's deck is the named CORPUS
// cards over Mountains, seat 1's is the named fixture sources over Mountains,
// and the CR 103.1 toss is driven to start seat 0 (seatZeroStart) so the test
// can address seats and turns by index. reg is the shared compiled corpus;
// cfg carries reg.Tokens so replayCheck's genesis reproduces it.
func altCostEngine(t *testing.T, seed uint64, heroCorpus []string, heroFixtures, foeFixtures []string) (*Engine, Config, *cards.Registry) {
	t.Helper()
	return altCostEngineReg(t, seed, testutil.CorpusRegistry(t), heroCorpus, heroFixtures, foeFixtures)
}

// altCostEngineReg is altCostEngine over a caller-supplied registry (the
// fresh-compile variant uses it).
func altCostEngineReg(t *testing.T, seed uint64, reg *cards.Registry, heroCorpus []string, heroFixtures, foeFixtures []string) (*Engine, Config, *cards.Registry) {
	t.Helper()
	var hero, foe []*cards.Card
	for _, name := range heroCorpus {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("registry lacks %q", name)
		}
		hero = append(hero, c)
	}
	for _, src := range heroFixtures {
		hero = append(hero, card(t, src))
	}
	for _, src := range foeFixtures {
		foe = append(foe, card(t, src))
	}
	build := func(s uint64) Config {
		hd := append(append([]*cards.Card(nil), hero...), mountainDeck(t, 40-len(hero))...)
		fd := append(append([]*cards.Card(nil), foe...), mountainDeck(t, 40-len(foe))...)
		return Config{Seed: s, Names: []string{"a", "b"},
			Decks:  [][]*cards.Card{hd, fd},
			Tokens: reg.Tokens}
	}
	cfg := seatZeroStart(build(seed))
	e := New(cfg)
	e.Advance()
	return e, cfg, reg
}

// freshEncoreRegistry compiles a minimal corpus fresh (no ir.gob.gz cache):
// the real Impulsive Pilferer script and the Treasure token its dies trigger
// mints, copied out of the worktree's .cards corpus into a temp dir. This is
// the only way a test can see the K:Encore expansion before the shared cache
// is recompiled (see the file header).
func freshEncoreRegistry(t *testing.T) *cards.Registry {
	t.Helper()
	dir := t.TempDir()
	cf := filepath.Join(dir, "cardsfolder")
	tk := filepath.Join(dir, "tokenscripts")
	if err := os.MkdirAll(cf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tk, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{"../.cards/cardsfolder/i/impulsive_pilferer.txt", filepath.Join(cf, "impulsive_pilferer.txt")},
		{"../.cards/tokenscripts/c_a_treasure_sac.txt", filepath.Join(tk, "c_a_treasure_sac.txt")},
	} {
		b, err := os.ReadFile(pair[0])
		if err != nil {
			t.Fatalf("corpus copy: %v", err)
		}
		if err := os.WriteFile(pair[1], b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reg, _, err := cards.CompileDir(cf)
	if err != nil {
		t.Fatalf("fresh compile: %v", err)
	}
	return reg
}

// findCardObj locates the seeded card by face name in seat p's zones, moving
// it to `to` with a logged MoveZone when genesis shuffled it elsewhere, and
// returning its id. It drives to seat 0's Main1 first (the move happens with
// the game parked at a real priority point, exactly as moveSeeded does) and
// re-runs priorityRound after any move, so the caller's next helper sees a
// fresh decision over the moved card.
func findCardObj(t *testing.T, e *Engine, p state.PlayerID, name string, to state.Zone) state.ObjID {
	t.Helper()
	if e.G.Active == 0 {
		toMain1(t, e)
	} else {
		driveToStep(t, e, e.G.Turn+1, 0, state.StepMain1)
	}
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary, state.ZBattlefield, state.ZGraveyard} {
		for _, id := range e.G.Zone(z, p) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				if o.Zone != to {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: to})
					// The emit invalidated the parked priority decision; a
					// fresh round re-derives it so the caller's next helper
					// (addMana, the Pending() readers) sees a real decision.
					e.pending = nil
					e.priorityRound()
				}
				return id
			}
		}
	}
	t.Fatalf("card %q not found in seat %d's zones", name, p)
	return 0
}

// castModeOption returns the index of the pending priority decision's cast
// option for id with the given mode, failing the test when absent.
func castModeOption(t *testing.T, e *Engine, id state.ObjID, mode string) int {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision pending: %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == mode {
			return o.Index
		}
	}
	t.Fatalf("no (%s) cast option for %s in %+v", mode, e.G.Obj(id).Face().Name, d.Options)
	return -1
}

// passOnce answers one pending priority decision with its pass option.
func passOnceP(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("passOnce: non-priority decision %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "pass" {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
				t.Fatalf("submit pass: %v", err)
			}
			return
		}
	}
	t.Fatalf("priority decision with no pass option: %+v", d.Options)
}

func TestEvokeCastPaysTheEvokeCostAndSacrifices(t *testing.T) {
	e, cfg, _ := altCostEngine(t, 911, []string{"Nulldrifter"}, nil, nil)
	id := findCardObj(t, e, 0, "Nulldrifter", state.ZHand)
	// The pool added here is exactly the EVOKE cost {2}{U}, not the printed
	// {7}: the (evoked) option exists only because beginCast's "evoked" mode
	// charged the keyword cost in place of the mana cost.
	addMana(t, e, 0, "CCU")
	submitChoices(t, e, castModeOption(t, e, id, "evoked"))
	passOnceP(t, e) // seat 0 passes
	passOnceP(t, e) // seat 1 passes; the evoked spell resolves onto the battlefield
	// CR 702.79a: the entry queues the evoke sacrifice follow-up -- a
	// mandatory trigger with no question, whose placement inside the drain
	// emits the sacrifice. Nothing is on the stack, so the drain is already
	// finished by the time the second pass returns.
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("evoked Nulldrifter in %s, want graveyard (CR 702.79a sacrifices it)", o.Zone)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("pool after evoke %d, want 0", e.G.Players[0].Pool.Total())
	}
	sacrificed := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.From == state.ZBattlefield &&
			ev.To == state.ZGraveyard && ev.Text == "sacrificed (evoke)" {
			sacrificed = true
		}
	}
	if !sacrificed {
		t.Fatal("no sacrificed-(evoke) MoveZone in the log")
	}
	replayCheck(t, e, cfg)
}

func TestEvokeExileCostExilesTheChosenCard(t *testing.T) {
	e, cfg, _ := altCostEngine(t, 912, []string{"Fury"}, []string{altRedSrc}, nil)
	id := findCardObj(t, e, 0, "Fury", state.ZHand)
	ember := findCardObj(t, e, 0, "Ember", state.ZHand)
	addMana(t, e, 0, "") // refresh the offer list with both cards in hand
	submitChoices(t, e, castModeOption(t, e, id, "evoked"))
	// The evoked cost is ExileFromHand<1/Card.Red+Other>: an exile-cost ask
	// whose only candidate is the fixture Ember (Fury itself is excluded by
	// the +Other self-reference, and the Mountains are not red).
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 ||
		len(d.Options) != 1 || d.Options[0].Kind != "exilecost" || d.Options[0].Obj != ember {
		t.Fatalf("exile-cost ask: %+v", d)
	}
	submitChoices(t, e, 0)
	passOnceP(t, e) // seat 0 passes
	passOnceP(t, e) // seat 1 passes; the evoked spell resolves onto the battlefield
	// Fury's own ETB (4 damage divided, no legal targets) and the evoke
	// sacrifice follow-up queue from the same entry: one same-controller
	// ordering ask.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder {
		t.Fatalf("ordering ask: %+v", d)
	}
	submitChoices(t, e, 0, 1)
	// The ordered-first ETB trigger resolves into a division ask with no
	// legal candidates (TargetMin$ 0): an empty answer divides nothing.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 0 {
		t.Fatalf("Fury ETB division ask: %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
		t.Fatalf("empty division answer: %v", err)
	}
	// CR 702.79a: the evoked creature is sacrificed -- unconditionally.
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("evoked Fury in %s, want graveyard", o.Zone)
	}
	if o := e.G.Obj(ember); o.Zone != state.ZExile {
		t.Fatalf("Ember zone %s, want exile", o.Zone)
	}
	replayCheck(t, e, cfg)
}

func TestDashCastsGainHasteAndReturnAtTheEndStep(t *testing.T) {
	e, cfg, _ := altCostEngine(t, 914, []string{"Ragavan, Nimble Pilferer"}, nil, nil)
	id := findCardObj(t, e, 0, "Ragavan, Nimble Pilferer", state.ZHand)
	addMana(t, e, 0, "CR")
	submitChoices(t, e, castModeOption(t, e, id, "dashed"))
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.CastFlags&state.FlagDashed == 0 {
		t.Fatalf("Ragavan zone %s flags %d", o.Zone, o.CastFlags)
	}
	if !e.HasKeyword(id, "Haste") {
		t.Fatal("dashed Ragavan has no haste")
	}
	// The end step's delayed trigger returns it to hand.
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEnd)
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZHand {
		t.Fatalf("Ragavan after the end step: %s, want hand", o.Zone)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("dash registration not consumed: %d", len(e.G.Delayed))
	}
	replayCheck(t, e, cfg)
}

func TestEncoreActivatesFromTheGraveyardIntoHastedTokenCopies(t *testing.T) {
	reg := freshEncoreRegistry(t)
	e, cfg, _ := altCostEngineReg(t, 915, reg, []string{"Impulsive Pilferer"}, nil, nil)
	id := findCardObj(t, e, 0, "Impulsive Pilferer", state.ZGraveyard)
	// The encore cost is {3}{R} plus exiling the card itself: four mana.
	addMana(t, e, 0, "CCCR")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no encore ability option: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 40)
	// One opponent, one hasted token copy; the card itself is exiled.
	if o := e.G.Obj(id); o.Zone != state.ZExile {
		t.Fatalf("Impulsive Pilferer zone %s, want exile", o.Zone)
	}
	tokens := 0
	var tokID state.ObjID
	for _, tid := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(tid)
		if o.IsToken && o.Face() != nil && o.Face().Name == "Impulsive Pilferer" {
			tokens++
			tokID = tid
		}
	}
	if tokens != 1 {
		t.Fatalf("encore tokens %d, want 1", tokens)
	}
	if !e.HasKeyword(tokID, "Haste") {
		t.Fatal("encore token has no haste")
	}
	// Sacrificed at the beginning of the next end step; a token that leaves
	// the battlefield ceases to exist (CR 704.5d), so the object's tombstone
	// zone is ZCeased, not the graveyard.
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEnd)
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(tokID); o.Zone != state.ZCeased {
		t.Fatalf("encore token after the end step: %s, want ceased", o.Zone)
	}
	replayCheck(t, e, cfg)
}

func TestOverloadedCastTargetsEachNotOne(t *testing.T) {
	e, cfg, _ := altCostEngine(t, 916, []string{"Cyclonic Rift"}, nil, []string{altBearSrc, altBearSrc})
	id := findCardObj(t, e, 0, "Cyclonic Rift", state.ZHand)
	b1 := findCardObj(t, e, 1, "Bear", state.ZBattlefield)
	b2 := findCardObj(t, e, 1, "Bear", state.ZBattlefield)
	if b1 == b2 {
		t.Fatal("the two Bears resolved to one object")
	}
	addMana(t, e, 0, "CCCCCCCU")
	submitChoices(t, e, castModeOption(t, e, id, "overloaded"))
	// No target decision at all: the overloaded spell targets "each".
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("Cyclonic Rift zone %s", o.Zone)
	}
	for _, bear := range []state.ObjID{b1, b2} {
		if o := e.G.Obj(bear); o.Zone != state.ZHand {
			t.Fatalf("Bear %d in %s, want bounced to hand", bear, o.Zone)
		}
	}
	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != 0 {
		t.Fatalf("seat 0 lost %d permanents to its own overloaded Rift", got)
	}
	replayCheck(t, e, cfg)
}

func TestWarpCastsFromTheGraveyardExileAtEndStepAndRecastFromExile(t *testing.T) {
	e, cfg, _ := altCostEngine(t, 917, []string{"Timeline Culler"}, nil, nil)
	id := findCardObj(t, e, 0, "Timeline Culler", state.ZGraveyard)
	addMana(t, e, 0, "B")
	submitChoices(t, e, castModeOption(t, e, id, "warped"))
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWarped == 0 {
		t.Fatalf("Timeline Culler zone %s flags %d", o.Zone, o.CastFlags)
	}
	// The warp cost's two life.
	if life := e.G.Players[0].Life; life != 18 {
		t.Fatalf("life after warp %d, want 18", life)
	}
	// Exiled at the beginning of the next end step.
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEnd)
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZExile {
		t.Fatalf("Timeline Culler after the end step: %s, want exile", o.Zone)
	}
	// On a later turn, the recast from exile is offered and works.
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepMain1)
	addMana(t, e, 0, "B")
	submitChoices(t, e, castModeOption(t, e, id, "warped"))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("Timeline Culler after recast: %s, want battlefield", o.Zone)
	}
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEnd)
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZExile {
		t.Fatalf("Timeline Culler after the second cycle: %s, want exile", o.Zone)
	}
	replayCheck(t, e, cfg)
}

func TestMadnessDiscardExilesAndOffersTheCast(t *testing.T) {
	e, cfg, _ := altCostEngine(t, 918, []string{"Fiery Temper", "Mind Rot"}, []string{altRedSrc}, nil)
	temper := findCardObj(t, e, 0, "Fiery Temper", state.ZHand)
	ember := findCardObj(t, e, 0, "Ember", state.ZHand)
	addMana(t, e, 0, "CCBR")
	// Cast Mind Rot targeting seat 0, the discarder of the Temper.
	mr := findCardObj(t, e, 0, "Mind Rot", state.ZHand)
	mrIdx := castModeOption(t, e, mr, "")
	submitChoices(t, e, mrIdx)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Mind Rot target ask: %+v", d)
	}
	seat0 := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 0 {
			seat0 = o.Index
		}
	}
	submitChoices(t, e, seat0)
	passOnceP(t, e) // seat 0 passes
	passOnceP(t, e) // seat 1 passes; Mind Rot resolves and its TgtChoose discard ask suspends it
	// The discard choice: the Temper plus the Ember.
	d = e.Pending()
	if d == nil || d.Kind != decision.KModes || len(d.Options) < 2 {
		t.Fatalf("discard choice: %+v", d)
	}
	temperIdx, emberIdx := -1, -1
	for _, o := range d.Options {
		if o.Obj == temper {
			temperIdx = o.Index
		}
		if o.Obj == ember {
			emberIdx = o.Index
		}
	}
	if temperIdx < 0 || emberIdx < 0 {
		t.Fatalf("discard options missing the seeded cards: %+v", d.Options)
	}
	submitChoices(t, e, temperIdx, emberIdx)
	// CR 702.35a: the discarded madness card is in exile, and the owner is
	// asked to cast it for its madness cost.
	if o := e.G.Obj(temper); o.Zone != state.ZExile {
		t.Fatalf("Fiery Temper zone %s, want exile", o.Zone)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional || d.Player != 0 {
		t.Fatalf("madness offer: %+v", d)
	}
	submitChoices(t, e, 0) // yes
	// The cast flow from exile at the madness cost {R}, then its damage
	// target ask.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("madness cast target ask: %+v", d)
	}
	foe := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			foe = o.Index
		}
	}
	submitChoices(t, e, foe)
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(temper); o.Zone != state.ZGraveyard {
		t.Fatalf("Fiery Temper after resolving: %s", o.Zone)
	}
	if life := e.G.Players[1].Life; life != 17 {
		t.Fatalf("opponent life %d, want 17", life)
	}
	if life := e.G.Players[0].Life; life != 20 {
		t.Fatalf("caster life %d, want 20 (the Ember discard was not a cost)", life)
	}
	replayCheck(t, e, cfg)
}

func TestMadnessExileByNonDiscardDoesNotOfferTheCast(t *testing.T) {
	// CR 702.35a opens the cast window on a DISCARD only. Evoking Fury exiles
	// a red card as a COST -- here the real madness card Fiery Temper -- and
	// that exile must not offer the Temper's madness cast.
	e, cfg, _ := altCostEngine(t, 922, []string{"Fury", "Fiery Temper"}, nil, nil)
	id := findCardObj(t, e, 0, "Fury", state.ZHand)
	temper := findCardObj(t, e, 0, "Fiery Temper", state.ZHand)
	// Mind Rot is in the deck to keep Fury from being the only eligible red
	// card: the exile-cost ask's options must include the Temper.
	addMana(t, e, 0, "")
	submitChoices(t, e, castModeOption(t, e, id, "evoked"))
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 {
		t.Fatalf("exile-cost ask: %+v", d)
	}
	temperIdx := -1
	for _, o := range d.Options {
		if o.Obj == temper {
			temperIdx = o.Index
		}
	}
	if temperIdx < 0 {
		t.Fatalf("exile options missing the Temper: %+v", d.Options)
	}
	submitChoices(t, e, temperIdx) // exile the Temper as the evoke cost
	passOnceP(t, e)
	passOnceP(t, e) // the evoked Fury resolves onto the battlefield
	// The evoke sacrifice and Fury's ETB share one ordering ask; the
	// sacrifice lands when the follow-up is placed (no question of its own).
	d = e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder {
		t.Fatalf("ordering ask: %+v", d)
	}
	submitChoices(t, e, 0, 1)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 0 {
		t.Fatalf("Fury ETB division ask: %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
		t.Fatalf("empty division answer: %v", err)
	}
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("evoked Fury in %s, want graveyard", o.Zone)
	}
	if o := e.G.Obj(temper); o.Zone != state.ZExile {
		t.Fatalf("Temper zone %s, want exile", o.Zone)
	}
	// And no madness cast offer was ever posed: the only asks after the
	// cost exile are the ordering and division asks and priority.
	for _, ev := range e.L.Events {
		if ev.Kind == events.DecisionAsk && ev.Text == string(decision.KTriggerOptional) {
			t.Fatalf("bogus madness cast offer in the log: %+v", ev)
		}
	}
	replayCheck(t, e, cfg)
}

func TestAlternateAdditionalCostAsksWhichAlternative(t *testing.T) {
	e, cfg, _ := altCostEngine(t, 919, []string{"Redirect Lightning", "Shock"}, nil, []string{altBearSrc})
	rl := findCardObj(t, e, 0, "Redirect Lightning", state.ZHand)
	shock := findCardObj(t, e, 0, "Shock", state.ZHand)
	bear := findCardObj(t, e, 1, "Bear", state.ZBattlefield)
	addMana(t, e, 0, "RRRR")
	d := e.Pending()
	shockIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == shock && o.Mode == "" {
			shockIdx = o.Index
		}
	}
	if shockIdx < 0 {
		t.Fatalf("no Shock cast option: %+v", d.Options)
	}
	submitChoices(t, e, shockIdx)
	// Shock's target ask: the Bear.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Shock target ask: %+v", d)
	}
	bearIdx := -1
	for _, o := range d.Options {
		if o.Obj == bear {
			bearIdx = o.Index
		}
	}
	submitChoices(t, e, bearIdx)
	// Seat 0 kept priority with Shock still on the stack: cast Redirect
	// Lightning. Its either-or additional cost asks which one to pay.
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority with Shock on the stack: %+v", d)
	}
	rlIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == rl && o.Mode == "" {
			rlIdx = o.Index
		}
	}
	if rlIdx < 0 {
		t.Fatalf("no Redirect Lightning cast option: %+v", d.Options)
	}
	submitChoices(t, e, rlIdx)
	// The either-or ask: both alternatives payable, script order (5 life
	// first), then the {2} alternative.
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 ||
		d.Options[0].Kind != "altaddcost" || d.Options[0].Label != "Pay PayLife<5>" ||
		d.Options[1].Label != "Pay 2" {
		t.Fatalf("alternate-additional-cost ask: %+v", d)
	}
	submitChoices(t, e, 1) // pay {2}
	// Then Redirect Lightning's own target: the Shock on the stack.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Redirect Lightning target ask: %+v", d)
	}
	stackIdx := -1
	for _, o := range d.Options {
		if o.Obj == shock {
			stackIdx = o.Index
		}
	}
	if stackIdx < 0 {
		t.Fatalf("Redirect Lightning options missing the Shock on the stack: %+v", d.Options)
	}
	submitChoices(t, e, stackIdx)
	passUntilStackEmpty(t, e, 40)
	if pool := e.G.Players[0].Pool.Total(); pool != 0 {
		t.Fatalf("pool after the {2} alternative %d, want 0", pool)
	}
	if life := e.G.Players[0].Life; life != 20 {
		t.Fatalf("life %d, want 20 (the life alternative was not chosen)", life)
	}
	// The redirected-at Shock still resolved at its original target (CR
	// 601.2c's unchanged text: api:ChangeTargets is a separate, unimplemented
	// primitive, recorded in the ticket report's Issues) -- the Bear died to
	// it, and the cost assertion above is what this test owns.
	if o := e.G.Obj(bear); o.Zone != state.ZGraveyard {
		t.Fatalf("Bear zone %s, want graveyard (Shock resolved at it)", o.Zone)
	}
	replayCheck(t, e, cfg)
}
