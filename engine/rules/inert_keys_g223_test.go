package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g223: the two corpus behaviours whose census classification this ticket
// relies on. Both already hold at the brief's base -- the tests pin them so a
// later change to the mover or the replacement order cannot silently make the
// classification wrong. They also pin the runtime half: the ChangeZone
// unread Note must go silent for ETB$ (the census fix alone would leave it).

// g223GainControlFind returns the decision option whose object is id.
func g223OptionFor(t *testing.T, d *decision.Decision, id state.ObjID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Obj == id {
			return o.Index
		}
	}
	return -1
}

// TestG223MimeoplasmXIsTheEnteringX casts Mimeoplasm, Revered One for X=2
// with two creature cards in its owner's graveyard, picks both for the
// as-enters exile, and asserts the OLD probe's result: six +1/+1 counters
// (three per exiled card) and no "ignores unread parameter(s) ETB$" Note.
func TestG223MimeoplasmXIsTheEnteringX(t *testing.T) {
	t.Parallel()
	mimeo := tokenReplCorpusCard(t, "Mimeoplasm, Revered One")
	bears := corpusCard(t, "Grizzly Bears")
	giant := corpusCard(t, "Hill Giant")

	// Script precondition: the ETB replacement body is the one that carries
	// ETB$ and ChangeNum$ X, so the Note this test forbids really could fire.
	var changeSA *cards.SA
	for _, f := range mimeo.Faces {
		if sa := cards.ResolveSVar(f.SVars, "DBChangeZone"); sa != nil {
			changeSA = sa
		}
	}
	if changeSA == nil {
		t.Fatal("Mimeoplasm, Revered One no longer carries the DBChangeZone SVar")
	}
	if changeSA.Params["ETB"] != "True" || changeSA.Params["ChangeNum"] != "X" {
		t.Fatalf("precondition: DBChangeZone params moved: %+v", changeSA.Params)
	}

	e, cfg := tokenReplGame(t, 5, mimeo, bears, giant)
	mimeoID := moveSeededCard(t, e, 0, mimeo, state.ZHand)
	bearsID := moveSeededCard(t, e, 0, bears, state.ZGraveyard)
	giantID := moveSeededCard(t, e, 0, giant, state.ZGraveyard)
	if e.G.Obj(bearsID).Zone != state.ZGraveyard || e.G.Obj(giantID).Zone != state.ZGraveyard {
		t.Fatalf("precondition: the two yard creatures must be in the graveyard (%s, %s)", e.G.Obj(bearsID).Zone, e.G.Obj(giantID).Zone)
	}

	addMana(t, e, 0, "BBGGUU") // {2}{B}{G}{U} with X=2 -> five mana
	submitChoices(t, e, castCardOption(t, e, mimeoID).Index)

	// CR 601.2b: the cast's {X} is asked before the spell resolves.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("X decision for the Mimeoplasm cast: %+v", d)
	}
	xIdx := -1
	for _, o := range d.Options {
		if o.Amount == 2 {
			xIdx = o.Index
		}
	}
	if xIdx < 0 {
		t.Fatalf("precondition: X=2 was not offered: %+v", d.Options)
	}
	submitChoices(t, e, xIdx)

	// Pass priority through Submit so the spell resolves inside a served
	// run: the as-enters replacement then poses its KChoose instead of
	// taking the no-host default.
	for i := 0; i < 10; i++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		passPriorityOnce(t, e)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("as-enters exile ask = %+v, want a KChoose", d)
	}
	idxBears := g223OptionFor(t, d, bearsID)
	idxGiant := g223OptionFor(t, d, giantID)
	if idxBears < 0 || idxGiant < 0 {
		t.Fatalf("precondition: both graveyard creatures were not offered: %+v", d.Options)
	}
	submitChoices(t, e, idxBears, idxGiant)

	o := e.G.Obj(mimeoID)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Mimeoplasm zone = %s, want battlefield", o.Zone)
	}
	if got := o.Counter("P1P1"); got != 6 {
		t.Fatalf("Mimeoplasm with two cards exiled for X=2 has %d +1/+1 counters, want 6 (three per card)", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "ETB$") {
			t.Errorf("ChangeZone loud-degraded ETB$: %+v -- the scoped classification must reach the runtime known-key table", ev)
		}
	}
	replayCheck(t, e, cfg)
}

// TestG223MeliraCapsPoisonAtOneAfterReplacement casts two fixture Poison
// spells (A:SP$ Poison | Defined$ You | Num$ 3) with Melira, the Living Cure
// on the battlefield. Her ReplaceCounter body caps the first at one; the
// ImmediateTrigger with AfterReplacement$ installs the this-turn lock after
// the replacement finishes, so the second places nothing. Poison stays 1.
func TestG223MeliraCapsPoisonAtOneAfterReplacement(t *testing.T) {
	t.Parallel()
	melira := tokenReplCorpusCard(t, "Melira, the Living Cure")
	poisonOne := card(t, "Name:G223 Poison One\nManaCost:0\nTypes:Sorcery\n"+
		"A:SP$ Poison | Defined$ You | Num$ 3 | SpellDescription$ You get three poison counters.\n"+
		"Oracle:x\n")
	poisonTwo := card(t, "Name:G223 Poison Two\nManaCost:0\nTypes:Sorcery\n"+
		"A:SP$ Poison | Defined$ You | Num$ 3 | SpellDescription$ You get three poison counters.\n"+
		"Oracle:x\n")

	// Script precondition: the fixture really is the Poison source the
	// replacement sees, and Melira's rider is the AfterReplacement shape the
	// census scopes.
	if len(poisonOne.Faces[0].Abilities) != 1 || poisonOne.Faces[0].Abilities[0].API != "Poison" ||
		poisonOne.Faces[0].Abilities[0].Params["Num"] != "3" {
		t.Fatalf("precondition: the Poison fixture moved: %+v", poisonOne.Faces[0].Abilities)
	}
	var rider *cards.SA
	for _, f := range melira.Faces {
		if sa := cards.ResolveSVar(f.SVars, "DBImmediateTrigger"); sa != nil {
			rider = sa
		}
	}
	if rider == nil || rider.Params["AfterReplacement"] != "True" {
		t.Fatalf("precondition: Melira's AfterReplacement rider moved: %+v", rider)
	}

	e, cfg := tokenReplGame(t, 5, melira, poisonOne, poisonTwo)
	meliraID := moveSeededCard(t, e, 0, melira, state.ZBattlefield)
	oneID := moveSeededCard(t, e, 0, poisonOne, state.ZHand)
	twoID := moveSeededCard(t, e, 0, poisonTwo, state.ZHand)
	if e.G.Obj(meliraID).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Melira zone = %s, want battlefield", e.G.Obj(meliraID).Zone)
	}
	if got := e.G.Players[0].Counter("POISON"); got != 0 {
		t.Fatalf("precondition: seat 0 starts at %d poison, want 0", got)
	}

	addMana(t, e, 0, "")
	submitChoices(t, e, castCardOption(t, e, oneID).Index)
	e.resolveTop()
	if got := e.G.Players[0].Counter("POISON"); got != 1 {
		t.Fatalf("Melira: the first 3-poison source -> %d, want 1 (Amount$ 1)", got)
	}

	addMana(t, e, 0, "")
	submitChoices(t, e, castCardOption(t, e, twoID).Index)
	e.resolveTop()
	if got := e.G.Players[0].Counter("POISON"); got != 1 {
		t.Fatalf("Melira: the second 3-poison source -> %d, want 1 (the AfterReplacement lock landed)", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "AfterReplacement$") {
			t.Errorf("ImmediateTrigger loud-degraded AfterReplacement$: %+v -- the scoped classification must cover it", ev)
		}
	}
	replayCheck(t, e, cfg)
}
