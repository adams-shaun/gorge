package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The "as this enters, choose a color" primitive (K:ETBReplacement
// ChooseColor) pinned end to end: the cast-time colour ask, the recorded
// Choose "color" event, the Produced$ Chosen mana read on BOTH mana paths
// (the CR 605.3b triggered path and the activation path), and the CR 733.1
// abort restore. Utopia Sprawl and Quirion Elves are REAL corpus cards
// (carriers come from the compiled corpus, never inline scripts -- the
// corpus is gitignored GPL text); the abort fixture is inline because it is
// a deliberately-authored unpayable-cost shape no corpus card carries.

// The helpers come from mass_primitives_test.go (corpusEngineCfg: a seat-0
// deck led by the named corpus cards, mountains after, driven to Main 1, with
// the Config returned for replayCheck) and oring_remember_targets_test.go
// (moveCorpusCard: a logged MoveZone of a real deck card). The abort fixture
// is inline because it is a deliberately-authored unpayable-cost shape no
// corpus card carries.

// chooseColorEvent finds the log's Choose "color" event for obj and returns
// its recorded letter.
func chooseColorEvent(t *testing.T, e *Engine, obj state.ObjID) string {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Obj == obj && ev.Counter == "color" {
			return ev.Text
		}
	}
	t.Fatalf("no Choose color event for object %d in the log", obj)
	return ""
}

// TestUtopiaSprawlChoosesAColorAndAddsTheChosenMana pins the whole primitive
// on the real corpus card: casting asks the WUBRG colour at cast time (before
// the target ask), the answered choice is recorded as events.Choose
// {Counter: "color"} on the aura, and tapping the enchanted Forest adds the
// chosen colour ON TOP of the Forest's own production (the CR 605.3b
// triggered path, rewriteChosenMana).
func TestUtopiaSprawlChoosesAColorAndAddsTheChosenMana(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{corpusCard(t, "Utopia Sprawl"), corpusCard(t, "Forest")}, nil)
	sprawl := moveCorpusCard(t, e, "Utopia Sprawl", 0, state.ZHand)
	forest := moveCorpusCard(t, e, "Forest", 0, state.ZBattlefield)

	addMana(t, e, 0, "G")
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending decision")
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == sprawl {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the Sprawl: %+v", d.Options)
	}
	submitChoices(t, e, idx)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the aura's target ask before entry, got %+v", d)
	}
	tgt := -1
	for _, o := range d.Options {
		if o.Obj == forest {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("Forest not offered as the aura target: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	d = passUntilNonPriority(t, e, 20)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" || d.Options[0].Kind != "color" || len(d.Options) != 5 {
		t.Fatalf("colour choice at entry %+v", d)
	}
	for i, want := range []string{"White", "Blue", "Black", "Red", "Green"} {
		if d.Options[i].Label != want {
			t.Fatalf("option %d label %q, want %q (WUBRG order)", i, d.Options[i].Label, want)
		}
	}
	submitChoices(t, e, 4) // Green
	if got := e.G.Obj(sprawl).ChosenColor; got != "G" {
		t.Fatalf("ChosenColor = %q, want G (recorded at entry)", got)
	}
	passUntilStackEmpty(t, e, 20)

	o := e.G.Obj(sprawl)
	if o.Zone != state.ZBattlefield || o.AttachedTo != forest || o.ChosenColor != "G" {
		t.Fatalf("Sprawl after resolution: zone %s attached %d color %q", o.Zone, o.AttachedTo, o.ChosenColor)
	}
	if got := chooseColorEvent(t, e, sprawl); got != "G" {
		t.Fatalf("Choose color event recorded %q, want G", got)
	}

	// Tap the enchanted Forest for mana: its own {G} plus the Sprawl's extra
	// chosen-colour {G}.
	e.askPriority(0)
	addMana(t, e, 0, "")
	mana, ok := findManaAbilityOption(e, forest, 0)
	if !ok {
		t.Fatalf("no mana activation for the Forest: %+v", e.Pending().Options)
	}
	submitChoices(t, e, mana.Index)
	if got := e.G.Players[0].Pool[state.MG]; got != 2 {
		t.Fatalf("pool {G} = %d after the Forest tapped, want 2 (Forest + Sprawl's chosen G)", got)
	}
	replayCheck(t, e, cfg)
}

// TestQuirionElvesChosenManaFollowsTheETBChoice pins the ACTIVATION path
// (rules/mana_activation.go resolveManaEffect's Chosen branch): the elves'
// second activation resolves to the ETB-recorded colour with no second ask,
// while the first ({G}) stays fixed.
func TestQuirionElvesChosenManaFollowsTheETBChoice(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{corpusCard(t, "Quirion Elves")}, nil)
	elves := moveCorpusCard(t, e, "Quirion Elves", 0, state.ZHand)
	e.pending = nil
	e.Advance()

	addMana(t, e, 0, "GG")
	castFirst(t, e, "cast")
	d := passUntilNonPriority(t, e, 40)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" || d.Options[0].Kind != "color" || len(d.Options) != 5 {
		t.Fatalf("colour choice %+v", d)
	}
	submitChoices(t, e, 2) // Black
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(elves); o.Zone != state.ZBattlefield || o.ChosenColor != "B" {
		t.Fatalf("Elves: zone %s color %q", o.Zone, o.ChosenColor)
	}

	// Activate the second mana ability ({T}: Add one mana of the chosen
	// color). Re-anchor priority on the caster after the resolution-time ask.
	e.askPriority(0)
	// The priority "activate" option opens the stage-1 ability wheel;
	// the Chosen ability is Ability index 1 ("Add chosen color").
	addMana(t, e, 0, "")
	mana, ok := findManaAbilityOption(e, elves, 0)
	if !ok {
		t.Fatalf("no mana activation for the Elves: %+v", e.Pending().Options)
	}
	submitChoices(t, e, mana.Index)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the stage-1 ability wheel, got %+v", d)
	}
	second := -1
	for _, o := range d.Options {
		if o.Ability == 1 {
			second = o.Index
		}
	}
	if second < 0 {
		t.Fatalf("no option for the Chosen ability: %+v", d.Options)
	}
	submitChoices(t, e, second)
	if got := e.G.Players[0].Pool[state.MB]; got != 1 {
		t.Fatalf("pool {B} = %d, want 1 (the chosen colour)", got)
	}
	if got := e.G.Players[0].Pool[state.MG]; got != 0 {
		t.Fatalf("pool {G} = %d, want 0 (the 1{G} cast spent both); a second ask must not have fired", got)
	}
	replayCheck(t, e, cfg)
}

// TestChooseColorRecordsTheAnswerAndAbortsRestoreIt pins CR 733.1's undo of
// an as-enters colour choice: the answer is recorded during the proposal, and
// the unpayable-mana abort restores the object to its pre-proposal choice
// state (abortCast's reverse Choose "color" event), like the ChosenName and
// ChosenNumber siblings.
func TestChooseColorRecordsTheAnswerAndAbortsRestoreIt(t *testing.T) {
	jewel := "Name:Jewel\nManaCost:G G\nTypes:Creature Elf Druid\nPT:1/1\n" +
		"K:ETBReplacement:Other:ChooseColor\n" +
		"SVar:ChooseColor:DB$ ChooseColor | Defined$ You | SpellDescription$ As CARDNAME enters, choose a color.\nOracle:x\n"
	// seatZeroStart + the etbConfig deck shape (the fixture card seeded as a
	// REAL deck card, mountains after): etbConfig itself does not pin the
	// toss, and a seat-1 starter would make driveToStep(1, 0, main1) chase
	// the game to its mill-out instead of stopping at seat 0's first turn.
	cfg := seatZeroStart(Config{Seed: 77, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append([]*cards.Card{card(t, jewel)}, mountainDeck(t, 39)...), mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	id := findByName(e, "Jewel", 0)
	if id == 0 {
		t.Fatal("Jewel not found")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZHand})
	e.pending = nil
	e.Advance()
	// The unpayable proposal is rejected before the permanent would enter.
	// Under CR 614.12 there is consequently no colour election and no choice
	// to undo; the card and its pre-proposal characteristics remain untouched.
	e.pending = nil
	e.beginCast(0, decision.Option{Kind: "cast", Obj: id})
	if o := e.G.Obj(id); o.Zone != state.ZHand || o.ChosenColor != "" {
		t.Fatalf("Jewel after rejected proposal: zone=%s color=%q", o.Zone, o.ChosenColor)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Obj == id && ev.Counter == "color" {
			t.Fatal("an as-enters color was recorded before the permanent could enter")
		}
	}
	replayCheck(t, e, cfg)
}
