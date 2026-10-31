package rules

import (
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Underbridge Warlock's boon: the "if" gate reads at each of its controller's end
// steps, a failed gate leaves the boon in place, the boon outlives the Warlock, and
// its one firing ends it.
func TestOneTimeBoonWaitsForItsConditionAndFiresOnce(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bears := lookup(t, reg, "Grizzly Bears")
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Underbridge Warlock"), bears, bears, bears}, nil)
	if e.G.Active != 0 {
		passUntil(t, e, func() bool { return e.G.Active == 0 && e.G.Step == state.StepMain1 })
	}
	warlock := moveByName(t, e, 0, "Underbridge Warlock", state.ZBattlefield)
	e.pending = nil
	e.priorityRound()
	passUntilStackEmpty(t, e, 10)
	boons := func() int {
		n := 0
		for i := range e.G.Delayed {
			if e.G.Delayed[i].Source == warlock && e.G.Delayed[i].CheckSVar != "" {
				n++
			}
		}
		return n
	}
	if boons() != 1 {
		t.Fatalf("%d boons after the Warlock entered", boons())
	}
	turn := e.G.Turn
	driveToStepAll(t, e, turn+1, 1, state.StepUpkeep)
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("no creature died, but the opponent is at %d", got)
	}
	if boons() != 1 {
		t.Fatal("a failed condition ended the boon")
	}
	driveToStepAll(t, e, turn+2, 0, state.StepMain1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: warlock, From: state.ZBattlefield, To: state.ZGraveyard})
	for range 3 {
		id := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	}
	e.pending = nil
	e.priorityRound()
	driveToStepAll(t, e, turn+3, 1, state.StepUpkeep)
	if got := e.G.Players[1].Life; got != 15 {
		t.Fatalf("after three deaths the opponent is at %d, want 15", got)
	}
	if boons() != 0 {
		t.Fatal("the boon survived its firing")
	}
}

// BoonAmount$ counts the firings a boon's recurring trigger lasts.
func TestBoonAmountCountsItsFirings(t *testing.T) {
	t.Parallel()
	const boon = "Name:Twice Boon Maker\nManaCost:0\nTypes:Sorcery\n" +
		"A:SP$ Effect | Boon$ True | Duration$ Permanent | Triggers$ OnCast\n" +
		"SVar:OnCast:Mode$ SpellCast | ValidCard$ Card | ValidActivatingPlayer$ You | BoonAmount$ 2 | TriggerZones$ Command | Execute$ Gain | TriggerDescription$ Gain one, twice.\n" +
		"SVar:Gain:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"
	const zero = "Name:Zero Cost Sorcery\nManaCost:0\nTypes:Sorcery\nA:SP$ GainLife | LifeAmount$ 0\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 7, boon, zero, zero, zero)
	for _, lib := range append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...) {
		if e.G.Obj(lib).Face().Name == "Zero Cost Sorcery" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: lib, From: state.ZLibrary, To: state.ZHand})
		}
	}
	if o := e.G.Obj(id); o.Zone != state.ZHand {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZHand})
	}
	toMain1(t, e)
	e.pending = nil
	e.priorityRound()
	cast := func(name string) {
		t.Helper()
		opt := castByName(t, e, 0, name)
		if opt == nil {
			t.Fatalf("no cast of %s", name)
		}
		submitChoices(t, e, opt.Index)
		passUntilStackEmpty(t, e, 20)
	}
	cast("Twice Boon Maker")
	life := e.G.Players[0].Life
	for _, want := range []int32{1, 2, 2} {
		cast("Zero Cost Sorcery")
		if got := e.G.Players[0].Life - life; got != want {
			t.Fatalf("life gained %d, want %d", got, want)
		}
	}
}

// Every parameter on an Effect's Mode$ Phase trigger is one the registration carries
// or one that has no meaning for it.
func TestEffectPhaseTriggerParamsAreRead(t *testing.T) {
	t.Parallel()
	known := map[string]bool{
		"Mode": true, "Phase": true, "Execute": true, "ValidPlayer": true, "OneOff": true, "Static": true,
		"OptionalDecider": true, "FirstCombat": true, "IsPresent": true, "PresentZone": true, "PresentCompare": true,
		"CheckSVar": true, "SVarCompare": true, "BoonAmount": true,
		// A command-zone registration fires wherever its source is; the rest is text.
		"TriggerZones": true, "TriggerDescription": true, "StackDescription": true, "Secondary": true,
	}
	key := regexp.MustCompile(`(\w+)\$`)
	ref := regexp.MustCompile(`\bTriggers\$ ([\w, ]+)`)
	reg := testutil.CorpusRegistry(t)
	carriers := 0
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			var lines []string
			for _, a := range f.Abilities {
				if a != nil {
					lines = append(lines, a.Params["Triggers"])
				}
			}
			for _, body := range f.SVars {
				for _, m := range ref.FindAllStringSubmatch(body, -1) {
					lines = append(lines, m[1])
				}
			}
			for _, names := range lines {
				for name := range strings.SplitSeq(names, ",") {
					body := f.SVars[strings.TrimSpace(name)]
					if !strings.Contains(body, "Mode$ Phase") {
						continue
					}
					carriers++
					for _, m := range key.FindAllStringSubmatch(body, -1) {
						if !known[m[1]] {
							t.Errorf("%s: an Effect's Phase trigger carries unread %s$", f.Name, m[1])
						}
					}
				}
			}
		}
	}
	if carriers == 0 {
		t.Fatal("no carrier found")
	}
}

// "If you have a boon" reads the boon itself, which has no object in the command zone.
func TestUnderbridgeWarlockSeesItsBoon(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bears := lookup(t, reg, "Grizzly Bears")
	deck := []*cards.Card{lookup(t, reg, "Underbridge Warlock")}
	for range 30 {
		deck = append(deck, bears)
	}
	e := corpusEngine(t, reg, deck, nil)
	if e.G.Active != 0 {
		passUntil(t, e, func() bool { return e.G.Active == 0 && e.G.Step == state.StepMain1 })
	}
	moveByName(t, e, 0, "Underbridge Warlock", state.ZBattlefield)
	e.pending = nil
	e.priorityRound()
	passUntilStackEmpty(t, e, 10)
	if e.boonsOf(0) != 1 || e.boonsOf(1) != 0 {
		t.Fatalf("boons %d and %d, want one for the Warlock's controller", e.boonsOf(0), e.boonsOf(1))
	}
	life := e.G.Players[0].Life
	turn := e.G.Turn
	driveToStepAll(t, e, turn+1, 1, state.StepUpkeep)
	if got := e.G.Players[0].Life; got != life-2 {
		t.Errorf("life %d after the end step, want %d: the Warlock's trigger reads the boon", got, life-2)
	}
}
