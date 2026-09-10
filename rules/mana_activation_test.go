package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

const volcanicIsland = "Name:Volcanic Island\nTypes:Land Island Mountain\nOracle:x\n"

func activateOption(t *testing.T, e *Engine, id state.ObjID) int {
	t.Helper()
	for _, o := range e.Pending().Options {
		if o.Kind == "activate" && o.Obj == id {
			return o.Index
		}
	}
	t.Fatalf("no mana activation for %d: %+v", id, e.Pending().Options)
	return -1
}

func manaOption(t *testing.T, d *decision.Decision, produced string) int {
	t.Helper()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("mana choice = %+v, want KChoose", d)
	}
	for _, o := range d.Options {
		if o.Kind == "mana" && o.Label == "Add "+produced {
			return o.Index
		}
	}
	t.Fatalf("no Add %s choice: %+v", produced, d.Options)
	return -1
}

// TestManaActivationChoosesOneAbility makes Volcanic Island's two intrinsic
// basic-land abilities into one real choice. The source is not tapped until
// that choice is answered, then exactly the selected mana reaches the pool.
func TestManaActivationChoosesOneAbility(t *testing.T) {
	e := layerEngine(t)
	volcanic := onBoard(t, e, 0, volcanicIsland)
	e.askPriority(0)
	submitChoices(t, e, activateOption(t, e, volcanic))
	d := e.Pending()
	red := manaOption(t, d, "R")
	if d.Source != volcanic || len(d.Options) != 2 || e.G.Obj(volcanic).Tapped {
		t.Fatalf("Volcanic choice source=%d options=%+v tapped=%t", d.Source, d.Options, e.G.Obj(volcanic).Tapped)
	}
	submitChoices(t, e, red)
	pool := e.G.Players[0].Pool
	if !e.G.Obj(volcanic).Tapped || pool.Total() != 1 || pool[state.MR] != 1 || pool[state.MU] != 0 {
		t.Fatalf("Volcanic Island pool=%+v tapped=%t, want exactly one red", pool, e.G.Obj(volcanic).Tapped)
	}
}

// TestCloneKeepsManaAbilityChoice proves a snapshot taken while the choice is
// pending owns the same activation continuation instead of dropping it.
func TestCloneKeepsManaAbilityChoice(t *testing.T) {
	e := layerEngine(t)
	volcanic := onBoard(t, e, 0, volcanicIsland)
	e.askPriority(0)
	submitChoices(t, e, activateOption(t, e, volcanic))
	c := e.Clone()
	red := manaOption(t, c.Pending(), "R")
	submitChoices(t, c, red)
	submitChoices(t, e, manaOption(t, e.Pending(), "R"))
	if c.G.Players[0].Pool != e.G.Players[0].Pool || c.L.Head() != e.L.Head() {
		t.Fatalf("clone pool/head = %v/%s, original = %v/%s", c.G.Players[0].Pool, c.L.Head(), e.G.Players[0].Pool, e.L.Head())
	}
}

// TestManaActivationSingletonDoesNotAsk pins the unchanged common path: a
// Forest resolves directly from the priority action with no extra decision.
func TestManaActivationSingletonDoesNotAsk(t *testing.T) {
	e := layerEngine(t)
	forest := onBoard(t, e, 0, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n")
	e.askPriority(0)
	submitChoices(t, e, activateOption(t, e, forest))
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("Forest activation asked an extra decision: %+v", d)
	}
	pool := e.G.Players[0].Pool
	if pool.Total() != 1 || pool[state.MG] != 1 {
		t.Fatalf("Forest pool=%+v, want one green", pool)
	}
}

// TestManaActivationChoiceExcludesRestrictedMember preserves per-ability
// CantBeActivated gating: B is absent, while U and R remain selectable.
// TestCastManaWindowChoosesOneAbility ensures the CR 601.2g call site shares
// the same choice rather than returning to the old all-abilities bundle.
func TestCastManaWindowChoosesOneAbility(t *testing.T) {
	spell := "Name:Blue Spell\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 901, spell)
	volcanic := onBoard(t, e, 0, volcanicIsland)
	e.pending = nil // direct proposal deliberately exercises the payment window.
	e.beginCast(0, decision.Option{Kind: "cast", Obj: id})
	e.Advance()
	d := e.Pending()
	activate := -1
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == volcanic {
			activate = o.Index
		}
	}
	if activate < 0 {
		t.Fatalf("no Volcanic Island source in mana window: %+v", d)
	}
	submitChoices(t, e, activate)
	d = e.Pending()
	blue := manaOption(t, d, "U")
	if e.G.Obj(volcanic).Tapped {
		t.Fatal("Volcanic Island tapped before payment-window ability choice")
	}
	submitChoices(t, e, blue)
	if !e.G.Obj(volcanic).Tapped || len(e.G.Stack) != 1 || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("cast-time choice tapped=%t stack=%v pool=%+v, want tapped source, spell paid once", e.G.Obj(volcanic).Tapped, e.G.Stack, e.G.Players[0].Pool)
	}
}

func TestManaActivationChoiceExcludesRestrictedMember(t *testing.T) {
	e := layerEngine(t)
	mint := onBoard(t, e, 0, "Name:Mint\nTypes:Artifact\n"+
		"A:AB$ Mana | Cost$ T | Produced$ B\n"+
		"A:AB$ Mana | Cost$ T | Produced$ U\n"+
		"A:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	needle := onBoard(t, e, 0, "Name:NeedleB\nTypes:Artifact\n"+
		"S:Mode$ CantBeActivated | ValidCard$ Card.NamedCard | ValidSA$ Activated.ManaAbility<Produce:B>\nOracle:x\n")
	e.emit(events.Event{Kind: events.Choose, Obj: needle, Counter: "name", Text: "Mint"})
	e.askPriority(0)
	submitChoices(t, e, activateOption(t, e, mint))
	d := e.Pending()
	if d == nil || len(d.Options) != 2 {
		t.Fatalf("restricted Mint choices = %+v, want U and R only", d)
	}
	for _, o := range d.Options {
		if o.Label == "Add B" {
			t.Fatalf("restricted B ability offered: %+v", d.Options)
		}
	}
	submitChoices(t, e, manaOption(t, d, "U"))
	pool := e.G.Players[0].Pool
	if pool.Total() != 1 || pool[state.MU] != 1 || pool[state.MB] != 0 || pool[state.MR] != 0 {
		t.Fatalf("Mint pool=%+v, want selected U only", pool)
	}
}

func manaEventsFor(e *Engine, kind events.Kind, id state.ObjID) int {
	count := 0
	for _, ev := range e.L.Events {
		if ev.Kind == kind && ev.Obj == id {
			count++
		}
	}
	return count
}

func manaSourceEngine(t *testing.T, src string) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg, id := newFixtureDeck(t, 93, src)
	moveSeeded(t, e, 0, src, state.ZBattlefield)
	e.Advance()
	return e, cfg, id
}

func TestLotusPetalPaysSacrificeAndChoosesColor(t *testing.T) {
	const petal = "Name:Lotus Petal\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ Sac<1/CARDNAME> | Produced$ Any\nOracle:x\n"
	e, cfg, id := manaSourceEngine(t, petal)
	activateMana(t, e, id)
	if got := manaEventsFor(e, events.Tap, id); got != 0 {
		t.Fatalf("Lotus Petal Tap events = %d, want 0", got)
	}
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("Lotus Petal zone = %s, want Graveyard", got)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 || len(d.Options) != 5 {
		t.Fatalf("colour decision = %+v", d)
	}
	for i, opt := range d.Options {
		if opt.Obj != id {
			t.Fatalf("colour option %d Obj = %d, want Lotus Petal %d", i, opt.Obj, id)
		}
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("choose white: %v", err)
	}
	if got := e.G.Players[0].Pool[state.MW]; got != 1 {
		t.Fatalf("white pool = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

func TestCloneKeepsManaColorChoice(t *testing.T) {
	const petal = "Name:Lotus Petal\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ Sac<1/CARDNAME> | Produced$ Any\nOracle:x\n"
	e, _, id := manaSourceEngine(t, petal)
	activateMana(t, e, id)
	c := e.Clone()
	red := manaOption(t, c.Pending(), "R")
	submitChoices(t, c, red)
	submitChoices(t, e, manaOption(t, e.Pending(), "R"))
	if c.G.Players[0].Pool != e.G.Players[0].Pool || c.L.Head() != e.L.Head() {
		t.Fatalf("clone pool/head = %v/%s, original = %v/%s", c.G.Players[0].Pool, c.L.Head(), e.G.Players[0].Pool, e.L.Head())
	}
}

func TestLionsEyeDiamondPaysTapAndSacrifice(t *testing.T) {
	const led = "Name:Lion's Eye Diamond\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ C | Amount$ 3\nOracle:x\n"
	e, cfg, id := manaSourceEngine(t, led)
	activateMana(t, e, id)
	if got := manaEventsFor(e, events.Tap, id); got != 1 {
		t.Fatalf("Lion's Eye Diamond Tap events = %d, want 1", got)
	}
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("Lion's Eye Diamond zone = %s, want Graveyard", got)
	}
	if got := e.G.Players[0].Pool[state.MC]; got != 3 {
		t.Fatalf("colourless pool = %d, want 3", got)
	}
	replayCheck(t, e, cfg)
}

func TestManaActivationTapOnlyStillTapsAndAddsMana(t *testing.T) {
	const land = "Name:Plain Mana Land\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ U\nOracle:x\n"
	e, cfg, id := manaSourceEngine(t, land)
	activateMana(t, e, id)
	if got := manaEventsFor(e, events.Tap, id); got != 1 || !e.G.Obj(id).Tapped {
		t.Fatalf("tap-only mana ability Tap events/tapped = %d/%v, want 1/true", got, e.G.Obj(id).Tapped)
	}
	if got := e.G.Players[0].Pool[state.MU]; got != 1 {
		t.Fatalf("blue pool = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

func TestManaAbilityChoiceOptionsMarkSource(t *testing.T) {
	const source = "Name:Split Mana Rock\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T | Produced$ W\n" +
		"A:AB$ Mana | Cost$ T | Produced$ U\nOracle:x\n"
	e, _, id := manaSourceEngine(t, source)
	activateMana(t, e, id)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 {
		t.Fatalf("mana ability decision = %+v", d)
	}
	for i, opt := range d.Options {
		if opt.Obj != id {
			t.Fatalf("mana option %d Obj = %d, want source %d", i, opt.Obj, id)
		}
	}
}
