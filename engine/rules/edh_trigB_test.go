package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edh_trigB_test.go plays the printed text of the EDH trigger-modes class
// ticket's three finished cards through a real game: Toralf, God of Fury
// (trig:ExcessDamage), Font of Agonies (trig:PayLife) and Widespread Panic
// (trig:Shuffled).

// trigBDrive resolves a resolution's own decisions the way a player would:
// pass on priority, take the offered order on a trigger-order ask, and stop
// at the first ask stop accepts (returning it) or at a stack with no pending
// decision (returning nil). Anything else is a fatal: these fixtures pose
// no other kind. before, when non-nil, runs at every ask so a caller can
// observe the queue at the trigger-order boundary.
func trigBDrive(t *testing.T, e *Engine, limit int, stop func(d *decision.Decision) bool,
	before func(d *decision.Decision)) *decision.Decision {
	t.Helper()
	for i := 0; i < limit; i++ {
		d := e.Pending()
		if d == nil {
			return nil
		}
		if stop != nil && stop(d) {
			return d
		}
		if before != nil {
			before(d)
		}
		switch d.Kind {
		case decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			submitChoices(t, e, idx)
		case decision.KTriggerOrder:
			choices := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				choices = append(choices, o.Index)
			}
			submitChoices(t, e, choices...)
		default:
			t.Fatalf("unexpected %v decision: %+v", d.Kind, d)
		}
	}
	t.Fatal("trigBDrive exceeded its decision budget")
	return nil
}

// trigBStackIdle stops a trigBDrive at the first priority ask once the stack
// has drained: the resolution under test is over, and passing further would
// run the next turn's steps.
func trigBStackIdle(e *Engine) func(d *decision.Decision) bool {
	return func(d *decision.Decision) bool {
		return len(e.G.Stack) == 0 && d.Kind == decision.KPriority
	}
}

// toralfTriggerLine asserts the printed Mode$ ExcessDamage line exists on
// Toralf's front face.
func toralfTriggerLine(t *testing.T, reg *cards.Registry) {
	t.Helper()
	card := searchCorpusCard(t, reg, "Toralf, God of Fury")
	if unsupported := reg.Unsupported(card, effects.Supported()); len(unsupported) != 0 {
		t.Fatalf("precondition: Toralf unsupported: %v", unsupported)
	}
	for _, tr := range card.Faces[0].Triggers {
		if tr.Mode == "ExcessDamage" && tr.ParamStr(cards.PKCombatDamage) == "False" {
			return
		}
	}
	t.Fatal("precondition: Toralf has no printed ExcessDamage/CombatDamage$ False line")
}

// TestEDHTriggerModesBToralfExcessDamage plays Toralf's printed line: cast
// Lightning Strike at an opponent's 2/2, the excess 1 is noncombat, so
// Toralf's trigger queues with X=1 and resolves dealing that damage to the
// chosen other target (the opponent), who loses 1 life.
func TestEDHTriggerModesBToralfExcessDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	toralfTriggerLine(t, reg)
	e, _, battle, _ := effectGrantEngine(t, 9402,
		[]string{"Toralf, God of Fury", "Grizzly Bears"}, []string{"Lightning Strike"})
	toralf := battle[0]
	victim := putOnBattlefield(t, e, 1, searchCorpusCard(t, reg, "Grizzly Bears"))
	if e.Toughness(victim) != 2 {
		t.Fatalf("precondition: victim toughness %d, want 2", e.Toughness(victim))
	}
	before := e.G.Players[1].Life

	addMana(t, e, 0, "CR")
	castNamed(t, e, "Lightning Strike")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the strike's target ask, got %+v", d)
	}
	submitChoices(t, e, targetOptionIdx(t, e, victim))
	// Drive the strike's resolution to Toralf's payout ask: exactly one
	// trigger (the excess-1 hit) must have queued, carrying the excess as
	// its amount (the body's TriggerCount$DamageAmount) -- the opponent
	// losing exactly 1 life below is the same assertion at resolution.
	sawQueue := false
	payout := trigBDrive(t, e, 80, func(d *decision.Decision) bool { return d.Kind == decision.KTarget },
		func(*decision.Decision) {
			if sawQueue || len(e.pendingTriggers) == 0 {
				return
			}
			sawQueue = true
			if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != toralf {
				t.Fatalf("strike queued %d triggers, want exactly Toralf's: %+v",
					len(e.pendingTriggers), e.pendingTriggers)
			}
			if amt := e.pendingTriggers[0].Ctx.TriggerContext.TriggerAmount; amt != 1 {
				t.Fatalf("trigger amount %d, want the excess 1", amt)
			}
		})
	if o := e.G.Obj(victim); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the strike did not kill the 2/2: %+v", o)
	}
	if payout == nil {
		t.Fatal("Toralf's trigger resolved without asking its payout target")
	}
	opp := -1
	for _, o := range payout.Options {
		if o.Kind == "player" && o.Player == 1 {
			opp = o.Index
		}
	}
	if opp < 0 {
		t.Fatalf("payout ask offers no opponent player: %+v", payout.Options)
	}
	submitChoices(t, e, opp)
	trigBDrive(t, e, 40, trigBStackIdle(e), nil)
	if got := e.G.Players[1].Life; got != before-1 {
		t.Fatalf("opponent life %d, want %d (Toralf dealt the excess 1)", got, before-1)
	}
}

// TestEDHTriggerModesBFontPayLife plays Font of Agonies' printed line:
// activate Kozilek's Translator's "Pay 1 life: Add {C}" in a game with Font
// on the battlefield, and Font gains exactly 1 blood counter for the 1 life
// the cost paid.
func TestEDHTriggerModesBFontPayLife(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	fontCard := searchCorpusCard(t, reg, "Font of Agonies")
	if unsupported := reg.Unsupported(fontCard, effects.Supported()); len(unsupported) != 0 {
		t.Fatalf("precondition: Font of Agonies unsupported: %v", unsupported)
	}
	found := false
	for _, tr := range fontCard.Faces[0].Triggers {
		if tr.Mode == "PayLife" && tr.ParamStr(cards.PKValidPlayer) == "You" {
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: Font of Agonies has no printed PayLife/ValidPlayer$ You line")
	}
	e, _, battle, _ := effectGrantEngine(t, 9403,
		[]string{"Font of Agonies", "Kozilek's Translator"}, nil)
	fontID, tr := battle[0], battle[1]
	if e.G.Obj(fontID).Counter("BLOOD") != 0 {
		t.Fatal("precondition: Font already carries blood counters")
	}
	before := e.G.Players[0].Life

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision pending, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if (o.Kind == "activate" || o.Kind == "ability") && o.Obj == tr {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the translator's pay-1-life ability is not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	// The cost's pay-life settlement queued Font's trigger, carrying the
	// life PAID as its amount (the body's TriggerCount$LifeAmount); a single
	// trigger poses no order ask, so the queue drains into the stack before
	// the next ask surfaces it. The resolution's outcomes assert it: the
	// ability paid exactly 1 life and Font carries exactly 1 blood counter.
	trigBDrive(t, e, 80, trigBStackIdle(e), nil)
	if got := e.G.Players[0].Life; got != before-1 {
		t.Fatalf("the ability paid %d life, want exactly 1 (%d -> %d)", before-got, before, got)
	}
	if n := e.G.Obj(fontID).Counter("BLOOD"); n != 1 {
		t.Fatalf("Font of Agonies carries %d blood counters, want 1", n)
	}
}

// TestEDHTriggerModesBWidespreadShuffle plays Widespread Panic's printed
// line: activate Elixir of Immortality (an ability resolving its DB$ Shuffle
// tail), and the effect-caused shuffle makes the Panic trigger; its body has
// the shuffling player put a chosen hand card on top of their library.
func TestEDHTriggerModesBWidespreadShuffle(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	panicCard := searchCorpusCard(t, reg, "Widespread Panic")
	if unsupported := reg.Unsupported(panicCard, effects.Supported()); len(unsupported) != 0 {
		t.Fatalf("precondition: Widespread Panic unsupported: %v", unsupported)
	}
	found := false
	for _, tr := range panicCard.Faces[0].Triggers {
		if tr.Mode == "Shuffled" && tr.ParamStr(cards.PKValidPlayer) == "Player" {
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: Widespread Panic has no printed Shuffled line")
	}
	e, _, battle, _ := effectGrantEngine(t, 9404,
		[]string{"Widespread Panic", "Elixir of Immortality"}, nil)
	elixir := battle[1]
	hand := e.G.Zone(state.ZHand, 0)
	if len(hand) == 0 {
		t.Fatal("precondition: seat 0's hand is empty")
	}
	libBefore := len(e.G.Zone(state.ZLibrary, 0))

	addMana(t, e, 0, "CC")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision pending, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if (o.Kind == "activate" || o.Kind == "ability") && o.Obj == elixir {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the elixir's ability is not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	// The elixir shuffled its owner's library (it and the graveyard went
	// in), so the Panic trigger queued before its put-card ask. A single
	// trigger poses no order ask, so the queue drains into the stack before
	// the next ask surfaces it.
	put := trigBDrive(t, e, 80, func(d *decision.Decision) bool {
		return d.Kind == decision.KChoose || d.Kind == decision.KTarget
	}, nil)
	if o := e.G.Obj(elixir); o == nil || o.Zone != state.ZLibrary {
		t.Fatalf("the elixir did not shuffle itself into the library: %+v", o)
	}
	if put == nil {
		t.Fatal("the Panic trigger resolved without asking which hand card")
	}
	chosen := put.Options[0]
	if chosen.Obj == 0 {
		t.Fatalf("put-card ask offers no object: %+v", put.Options)
	}
	submitChoices(t, e, chosen.Index)
	trigBDrive(t, e, 40, trigBStackIdle(e), nil)
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) != libBefore+2 { // the elixir and the chosen hand card joined it
		t.Fatalf("library size %d, want %d", len(lib), libBefore+2)
	}
	if lib[0] != chosen.Obj {
		t.Fatalf("library top is %d, want the chosen hand card %d", lib[0], chosen.Obj)
	}
}

// TestEDHTriggerModesBWidespreadFetchShuffle plays Widespread Panic's line
// against the search shuffle every fetch land ends in: activate Windswept
// Heath's "Search your library for a Forest or Plains card, put it onto the
// battlefield, then shuffle", and the search tail's effect-caused shuffle
// (shuffleLibraryOrder, the majority of trig:Shuffled's sites -- 210 corpus
// Origin$ Library search lines) makes the Panic trigger, exactly the way the
// DB$ Shuffle verb already did.
func TestEDHTriggerModesBWidespreadFetchShuffle(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	panicCard := searchCorpusCard(t, reg, "Widespread Panic")
	if unsupported := reg.Unsupported(panicCard, effects.Supported()); len(unsupported) != 0 {
		t.Fatalf("precondition: Widespread Panic unsupported: %v", unsupported)
	}
	e, _, battle, _ := effectGrantEngine(t, 9405,
		[]string{"Widespread Panic", "Windswept Heath"}, nil)
	panicID, heath := battle[0], battle[1]
	libBefore := len(e.G.Zone(state.ZLibrary, 0))

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision pending, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if (o.Kind == "activate" || o.Kind == "ability") && o.Obj == heath {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the heath's fetch ability is not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	// The CARDNAME sacrifice cost may pose its own one-option KChoose.
	if d = e.Pending(); d != nil && d.Kind == decision.KChoose && d.ResumeKind != "search" {
		if len(d.Options) != 1 || d.Options[0].Obj != heath {
			t.Fatalf("unexpected activation cost choice: %+v", d)
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	// The search ask: fetch the first Forest it offers.
	search := trigBDrive(t, e, 80, func(d *decision.Decision) bool {
		return d.Kind == decision.KChoose && d.ResumeKind == "search"
	}, nil)
	if search == nil {
		t.Fatal("the heath's search ask never surfaced")
	}
	pick := -1
	for _, o := range search.Options {
		if obj := e.G.Obj(o.Obj); obj != nil && obj.Face() != nil && obj.Face().Name == "Forest" {
			pick = o.Index
			break
		}
	}
	if pick < 0 {
		t.Fatalf("the search ask offered no Forest: %+v", search.Options)
	}
	submitChoices(t, e, pick)
	// The search tail's shuffle queued the Panic trigger; its put-card ask
	// (the hand_move choose, whose source is the trigger's own) surfaces
	// next. A single trigger poses no order ask, so the queue drains into
	// the stack before the ask surfaces it.
	put := trigBDrive(t, e, 80, func(d *decision.Decision) bool {
		return d.Kind == decision.KChoose && d.ResumeKind == "hand_move" && d.Player == 0
	}, nil)
	if put == nil {
		t.Fatal("the fetch shuffle did not queue the Panic trigger (no put-card ask)")
	}
	if put.Source != panicID {
		t.Fatalf("put-card ask source %d, want Widespread Panic %d", put.Source, panicID)
	}
	if o := e.G.Obj(heath); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the heath did not sacrifice itself: %+v", o)
	}
	fetched := false
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Forest" {
			fetched = true // the fetch found its Forest; the shuffle tail follows
			break
		}
	}
	if !fetched {
		t.Fatal("the heath's search did not put a Forest on the battlefield")
	}
	chosen := put.Options[0]
	if chosen.Obj == 0 {
		t.Fatalf("put-card ask offers no object: %+v", put.Options)
	}
	if o := e.G.Obj(chosen.Obj); o == nil || o.Zone != state.ZHand {
		t.Fatalf("put-card ask offers a non-hand object: %+v", o)
	}
	submitChoices(t, e, chosen.Index)
	trigBDrive(t, e, 40, trigBStackIdle(e), nil)
	lib := e.G.Zone(state.ZLibrary, 0)
	// The fetched Forest left the library for the battlefield and the chosen
	// hand card joined the library on top of it.
	if len(lib) != libBefore {
		t.Fatalf("library size %d, want %d (one Forest out, one hand card in)", len(lib), libBefore)
	}
	if lib[0] != chosen.Obj {
		t.Fatalf("library top is %d, want the chosen hand card %d", lib[0], chosen.Obj)
	}
}
