package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestFieryEmancipationTriplesDamage uses the unmodified compiled corpus
// replacement: ReplaceEffect changes DamageAmount before DamageDone is logged.
func TestFieryEmancipationTriplesDamage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Fiery Emancipation"))
	source := onBoard(t, e, 0, "Name:Red Source\nManaCost:R\nTypes:Creature\nPT:1/1\nOracle:x\n")

	e.damaging = source
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.damaging = 0
	if got := e.G.Players[1].Life; got != 14 {
		t.Fatalf("life = %d, want 14 after Fiery Emancipation tripled 2 damage", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Player == 1 && ev.Amount != 6 {
			t.Fatalf("logged damage = %d, want rewritten amount 6", ev.Amount)
		}
	}
}

// TestChandraCannotBeCountered uses Chandra's real R:Event$ Counter line and
// Counterspell's real effect: the Counter primitive must ask the replacement
// before it moves the target off the stack.
func TestChandraCannotBeCountered(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	chandra := e.G.AddObject(mustCorpusCard(t, reg, "Chandra, Awakened Inferno"), 0)
	chandra.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, []state.ObjID{chandra.ID})
	e.G.Stack = []state.ObjID{chandra.ID}

	counterspell := mustCorpusCard(t, reg, "Counterspell").Faces[0].SpellAbility()
	effects.Resolve(e, &effects.Ctx{Controller: 1, Targets: []state.Target{{Obj: chandra.ID}}}, counterspell)
	if got := e.G.Obj(chandra.ID).Zone; got != state.ZStack {
		t.Fatalf("Chandra zone = %s, want stack after its counter replacement", got)
	}
	if len(e.G.Stack) != 1 || e.G.Stack[0] != chandra.ID {
		t.Fatalf("stack = %v, want Chandra still present", e.G.Stack)
	}
}

// TestSpiderPunkStopsProtectionPrevention uses Spider-Punk's real global
// CantPreventDamage static. Protection is the engine's current prevention
// path, so the protected creature must still take the blue source's damage.
func TestSpiderPunkStopsProtectionPrevention(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Spider-Punk"))
	target := onBoard(t, e, 1, "Name:Protected\nTypes:Creature\nPT:1/3\nK:Protection from blue\nOracle:x\n")
	source := onBoard(t, e, 0, "Name:Blue Source\nManaCost:U\nTypes:Creature\nPT:1/1\nOracle:x\n")

	e.damaging = source
	e.emit(events.Event{Kind: events.Damage, Obj: target, Amount: 2})
	e.damaging = 0
	if got := e.G.Obj(target).Damage; got != 2 {
		t.Fatalf("damage = %d, want 2; Spider-Punk forbids protection prevention", got)
	}
}

// TestOjerAxonilRaisesSmallNoncombatRedDamage uses the unmodified compiled
// corpus replacement: DamageAmount$ LTX (less than Ojer's power),
// ValidSource$ Card.RedSource+YouCtrl (the <Colour>Source predicate family),
// IsCombat$ False, and a VarValue$ X whose SVar is Count$CardPower. Noncombat
// red damage below the power is raised to the power; combat damage is not.
func TestOjerAxonilRaisesSmallNoncombatRedDamage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Ojer Axonil, Deepest Might"))
	source := onBoard(t, e, 0, "Name:Red Source\nManaCost:R\nTypes:Creature\nPT:1/1\nOracle:x\n")

	e.damaging = source
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("life = %d, want 16: 2 noncombat red damage below power 4 becomes 4", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Player == 1 && ev.Amount != 4 {
			t.Fatalf("logged damage = %d, want rewritten amount 4", ev.Amount)
		}
	}

	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.combatDamaging = false
	if got := e.G.Players[1].Life; got != 14 {
		t.Fatalf("life = %d, want 14: combat damage is not Ojer Axonil's replacement", got)
	}
}

// TestTheSoundOfDrumsDoublesCombatDamage uses the DIRECT ReplaceCount$ form
// (VarValue$ ReplaceCount$DamageAmount/Twice, no SVar indirection) on the
// enchanted creature's combat damage.
func TestTheSoundOfDrumsDoublesCombatDamage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	aura := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "The Sound of Drums"))
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{bear}})

	e.damaging = bear
	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.combatDamaging = false
	e.damaging = 0
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("life = %d, want 16: the enchanted creature's 2 combat damage doubled", got)
	}
}

// TestHexingSquelcherProtectsYourSpells uses the battlefield-arm counter
// replacement (ValidSA$ Spell.YouCtrl): a counterspell from the opponent
// cannot counter a spell you control, while your own copy of the same
// replacement does not shield the opponent's spells.
func TestHexingSquelcherProtectsYourSpells(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Hexing Squelcher"))
	mine := e.G.AddObject(mustCorpusCard(t, reg, "Counterspell"), 0)
	mine.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, []state.ObjID{mine.ID})
	e.G.Stack = []state.ObjID{mine.ID}

	theirs := e.G.AddObject(mustCorpusCard(t, reg, "Counterspell"), 1)
	theirs.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 1, []state.ObjID{theirs.ID})
	e.G.Stack = append(e.G.Stack, theirs.ID)

	cs := mustCorpusCard(t, reg, "Counterspell").Faces[0].SpellAbility()
	effects.Resolve(e, &effects.Ctx{Controller: 1, Targets: []state.Target{{Obj: mine.ID}}}, cs)
	if got := e.G.Obj(mine.ID).Zone; got != state.ZStack {
		t.Fatalf("your spell left the stack (zone %s): Hexing Squelcher forbids countering it", got)
	}
	effects.Resolve(e, &effects.Ctx{Controller: 0, Targets: []state.Target{{Obj: theirs.ID}}}, cs)
	if got := e.G.Obj(theirs.ID).Zone; got == state.ZStack {
		t.Fatalf("the opponent's spell stayed on the stack: the replacement only shields YOUR spells")
	}
}
