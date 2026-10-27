package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// UseImprinted$ on a RepeatEach imprints the loop subject INSTEAD of
// remembering it (Forge's RepeatEachEffect): the body's "Remembered" stays the
// host's own list for the whole loop, and the subject is reachable only as
// "Imprinted". Loops without the key keep binding the subject as Remembered.

// useImprintedEngine is a two-seat corpus game at Main 1 whose seat 0 deck is
// the named cards followed by Grizzly Bears, and whose seat 1 deck is all
// Grizzly Bears.
func useImprintedEngine(t *testing.T, seed uint64, seat0 ...string) (*Engine, Config) {
	t.Helper()
	reg := searchTestRegistry(t)
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	var d0, d1 []*cards.Card
	for _, n := range seat0 {
		d0 = append(d0, mustCorpusCard(t, reg, n))
	}
	for len(d0) < 40 {
		d0 = append(d0, bear)
	}
	for len(d1) < 40 {
		d1 = append(d1, bear)
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{d0, d1}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// useImprintedDrive answers every pending decision until done reports true:
// priority passes, a target ask takes the options whose object is in want,
// and any other ask takes its "yes" option, else its first.
func useImprintedDrive(t *testing.T, e *Engine, want map[state.ObjID]bool, done func() bool) {
	t.Helper()
	for i := 0; i < 200 && !e.G.Over; i++ {
		if done() {
			return
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision pending before the scenario finished (stack depth %d)", len(e.G.Stack))
		}
		var choices []int
		switch {
		case d.Kind == decision.KPriority:
			for _, o := range d.Options {
				if o.Kind == "pass" {
					choices = []int{o.Index}
				}
			}
		case d.Kind == decision.KTarget:
			for _, o := range d.Options {
				if want[o.Obj] {
					choices = append(choices, o.Index)
				}
			}
		default:
			for _, o := range d.Options {
				if o.Kind == "yes" {
					choices = []int{o.Index}
				}
			}
			if len(choices) == 0 && len(d.Options) > 0 && d.Min > 0 {
				choices = []int{d.Options[0].Index}
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
			t.Fatalf("submit %s: %v", d.Kind, err)
		}
	}
	t.Fatal("scenario did not finish within the decision budget")
}

// TestUseImprintedDeceiverOfFormCopiesRevealedCard: the loop's body reads
// Remembered (the revealed card) beside Imprinted (the creature to become the
// copy). With the subject also bound as Remembered, each creature would clone
// itself.
func TestUseImprintedDeceiverOfFormCopiesRevealedCard(t *testing.T) {
	t.Parallel()
	e, cfg := useImprintedEngine(t, 8901, "Deceiver of Form", "Hill Giant", "Hill Giant")
	deceiver := moveByName(t, e, 0, "Deceiver of Form", state.ZBattlefield)
	giants := []state.ObjID{
		moveByName(t, e, 0, "Hill Giant", state.ZBattlefield),
		moveByName(t, e, 0, "Hill Giant", state.ZBattlefield),
	}
	if top := e.G.Zone(state.ZLibrary, 0); len(top) == 0 || e.Name(top[0]) != "Grizzly Bears" {
		t.Fatalf("precondition: library top is not a Grizzly Bears: %v", top)
	}
	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepBeginCombat)
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %v, want Deceiver's begin-combat trigger", e.G.Stack)
	}
	useImprintedDrive(t, e, nil, func() bool { return len(e.G.Stack) == 0 })
	for _, id := range giants {
		if got := e.Name(id); got != "Grizzly Bears" {
			t.Fatalf("creature %d is %q, want a copy of the revealed Grizzly Bears", id, got)
		}
	}
	if got := e.Name(deceiver); got != "Deceiver of Form" {
		t.Fatalf("Deceiver itself became %q; it is excluded by Creature.YouCtrl+Other", got)
	}
	replayCheck(t, e, cfg)
}

// TestUseImprintedHelmOfKaldraAttachesEquipment: Attach's Object$ Imprinted is
// the Equipment, Defined$ Remembered the Kaldra token the outer ability
// remembered. With the subject also bound as Remembered the Equipment would be
// attached to itself.
func TestUseImprintedHelmOfKaldraAttachesEquipment(t *testing.T) {
	t.Parallel()
	e, cfg := useImprintedEngine(t, 8902, "Helm of Kaldra", "Sword of Kaldra", "Shield of Kaldra")
	var equip []state.ObjID
	for _, n := range []string{"Helm of Kaldra", "Sword of Kaldra", "Shield of Kaldra"} {
		equip = append(equip, moveByName(t, e, 0, n, state.ZBattlefield))
	}
	addMana(t, e, 0, "C")
	e.pending = nil
	e.priorityRound()
	opt := abilityOptionByLabel(t, e, equip[0], "Kaldra")
	submitChoices(t, e, opt.Index)
	useImprintedDrive(t, e, nil, func() bool { return len(e.G.Stack) == 0 })
	var germ state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.Name(id) == "Kaldra" {
			germ = id
		}
	}
	if germ == 0 {
		t.Fatal("precondition: no Kaldra token on the battlefield")
	}
	for _, id := range equip {
		if got := e.G.Obj(id).AttachedTo; got != germ {
			t.Fatalf("%s attached to %d, want the Kaldra token %d", e.Name(id), got, germ)
		}
	}
	replayCheck(t, e, cfg)
}

// TestUseImprintedSoulOfEmancipationUnchanged: the body reads only
// ImprintedController, so each destroyed permanent's controller still gets
// one 3/3 Angel.
func TestUseImprintedSoulOfEmancipationUnchanged(t *testing.T) {
	t.Parallel()
	e, cfg := useImprintedEngine(t, 8903, "Soul of Emancipation", "Hill Giant")
	mine := moveByName(t, e, 0, "Hill Giant", state.ZBattlefield)
	theirs := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	if o := e.G.Obj(theirs); o.Owner != 1 || o.Controller != 1 {
		t.Fatalf("precondition: seat 1's Bears owner/controller = %d/%d, want 1/1", o.Owner, o.Controller)
	}
	moveByName(t, e, 0, "Soul of Emancipation", state.ZBattlefield)
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %v, want Soul's enters trigger", e.G.Stack)
	}
	useImprintedDrive(t, e, map[state.ObjID]bool{mine: true, theirs: true}, func() bool { return len(e.G.Stack) == 0 })
	for _, id := range []state.ObjID{mine, theirs} {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("precondition: target %d is in %v, want destroyed", id, z)
		}
	}
	angels := func(p state.PlayerID) int {
		var n int
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if e.G.Obj(id).IsToken && e.Name(id) == "Angel Token" {
				n++
			}
		}
		return n
	}
	// Seat 1's Bears is the only subject controlled by seat 1, so its Angel
	// proves the token went to the subject's controller. Seat 0 gets at least
	// the Hill Giant's: the loop today also visits the entering Soul itself
	// (the trigger's own card sits in Remembered), a separate defect filed as
	// a ticket, so seat 0's count is not pinned exactly.
	if got := angels(1); got != 1 {
		t.Fatalf("seat 1 controls %d Angel tokens, want 1", got)
	}
	if got := angels(0); got < 1 {
		t.Fatalf("seat 0 controls %d Angel tokens, want at least 1", got)
	}
	replayCheck(t, e, cfg)
}

// TestRepeatEachWithoutUseImprintedUnchanged: a loop without the key still
// binds each subject as Remembered, so the body's Defined$ Remembered pump
// reaches every creature the loop visits.
func TestRepeatEachWithoutUseImprintedUnchanged(t *testing.T) {
	t.Parallel()
	src := "Name:Rally Bearer\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
		"T:Mode$ Phase | Phase$ BeginCombat | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigEach | TriggerDescription$ x\n" +
		"SVar:TrigEach:DB$ RepeatEach | RepeatCards$ Creature.YouCtrl | RepeatSubAbility$ DBPump\n" +
		"SVar:DBPump:DB$ Pump | Defined$ Remembered | NumAtt$ +2\nOracle:x\n"
	bearSrc := "Name:Plain Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 8904, src, bearSrc)
	bearer := putCreature(t, e, 0, src)
	bear := putCreature(t, e, 0, bearSrc)
	e.priorityRound()
	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepBeginCombat)
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %v, want the begin-combat trigger", e.G.Stack)
	}
	useImprintedDrive(t, e, nil, func() bool { return len(e.G.Stack) == 0 })
	if got := e.Derived(bearer).Power; got != 3 {
		t.Fatalf("Rally Bearer power = %d, want 1+2", got)
	}
	if got := e.Derived(bear).Power; got != 4 {
		t.Fatalf("Plain Bear power = %d, want 2+2", got)
	}
	replayCheck(t, e, cfg)
}
