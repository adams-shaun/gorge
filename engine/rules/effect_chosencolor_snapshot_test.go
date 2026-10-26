package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const chosenColorStrandsSrc = "Name:Prismatic Strands Fixture\nManaCost:2 W\nTypes:Instant\n" +
	"K:Flashback:tapXType<1/Creature.White/white creature>\n" +
	"A:SP$ ChooseColor | Defined$ You | SubAbility$ DBEffect\n" +
	"SVar:DBEffect:DB$ Effect | ReplacementEffects$ RPrevent\n" +
	"SVar:RPrevent:Event$ DamageDone | Prevent$ True | ActiveZones$ Command | ValidSource$ Card.ChosenColor\n" +
	"Oracle:x\n"

// answerStrandsColour resolves the stack, answering the ChooseColor ask with
// the labelled colour and passing every priority ask.
func answerStrandsColour(t *testing.T, e *Engine, label string) {
	t.Helper()
	answered := false
	for i := 0; i < 40 && (len(e.G.Stack) > 0 || !answered); i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision with stack depth %d", len(e.G.Stack))
		}
		switch {
		case d.Kind == decision.KChoose && d.ResumeKind == "choosecolor":
			idx := optionByLabel(d.Options, label)
			if idx < 0 {
				t.Fatalf("choosecolor ask offers no %s option: %+v", label, d.Options)
			}
			submitChoices(t, e, idx)
			answered = true
		case d.Kind == decision.KPriority:
			for _, o := range d.Options {
				if o.Kind == "pass" {
					submitChoices(t, e, o.Index)
					break
				}
			}
		default:
			t.Fatalf("unexpected %s decision (resume %q): %+v", d.Kind, d.ResumeKind, d)
		}
	}
	if !answered {
		t.Fatalf("the ChooseColor ask was never posed")
	}
}

// TestEffectChosenColorSnapshot: each resolution of a "sources of the colour
// of your choice" Effect locks to the colour chosen THEN (CR 611.2c, 607.2).
// Prismatic Strands is cast choosing red and flashed back choosing green the
// same turn: both colours stay prevented, blue is not.
func TestEffectChosenColorSnapshot(t *testing.T) {
	t.Parallel()
	whiteSrc := "Name:White Guard\nManaCost:W\nTypes:Creature Human\nPT:1/1\nOracle:x\n"
	srcOf := func(name, cost string) string {
		return "Name:" + name + "\nManaCost:" + cost + "\nTypes:Creature Imp\nPT:1/1\nOracle:x\n"
	}
	e, cfg, strands := newFixtureDeck(t, 71, chosenColorStrandsSrc, whiteSrc,
		srcOf("Red Imp", "R"), srcOf("Green Imp", "G"), srcOf("Blue Imp", "U"))
	putCreature(t, e, 0, whiteSrc)
	red := moveByName(t, e, 0, "Red Imp", state.ZBattlefield)
	green := moveByName(t, e, 0, "Green Imp", state.ZBattlefield)
	blue := moveByName(t, e, 0, "Blue Imp", state.ZBattlefield)
	for id, want := range map[state.ObjID]string{red: "R", green: "G", blue: "U"} {
		if got := e.Colors(id); got != want {
			t.Fatalf("precondition: source %d colours %q, want %q", id, got, want)
		}
	}
	addMana(t, e, 0, "WWWW")
	castObj1 := func(mode string) {
		t.Helper()
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("not at priority: %+v", d)
		}
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == strands && (mode == "" || o.Mode == mode) {
				submitChoices(t, e, o.Index)
				return
			}
		}
		t.Fatalf("no %q cast option for strands: %+v", mode, d.Options)
	}
	castObj1("")
	answerStrandsColour(t, e, "Red")
	if o := e.G.Obj(strands); o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: strands in %s after the first resolution, want graveyard", o.Zone)
	}
	castObj1("flashback")
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose && d.ResumeKind != "choosecolor" {
		submitChoices(t, e, 0)
	}
	answerStrandsColour(t, e, "Green")
	if got := e.G.Obj(strands).ChosenColor; got != "G" {
		t.Fatalf("precondition: the card's live choice = %q, want G (the second cast overwrote the first)", got)
	}
	hit := func(src state.ObjID) bool {
		before := e.G.Players[1].Life
		e.damaging, e.combatDamaging = src, false
		e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
		e.damaging = 0
		return e.G.Players[1].Life < before
	}
	if hit(red) {
		t.Fatal("damage from the red source went through: the first cast's prevention was recoloured to green")
	}
	if hit(green) {
		t.Fatal("damage from the green source went through")
	}
	if !hit(blue) {
		t.Fatal("damage from the blue source was prevented")
	}
	replayCheck(t, e, cfg)
}
