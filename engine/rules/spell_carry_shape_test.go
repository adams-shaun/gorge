package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A static SpellCast rider's Animate is mirrored by dirac only for the timed,
// keyword-only grant (dirac/kgen/lower_statictrigger_spellcast.go's
// staticSpellCastKeywordAnimate -> kop.OvSpellCarry). The engine's CR 400.7a
// stack-to-battlefield carry must stay on that same shape: a wider carry
// animates the permanent while dirac never applies the rider (Wickerwing
// Effigy).
const spellCarryShapeBears = "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

func spellCarryShapeWatcher(name, body string) string {
	return "Name:" + name + "\nTypes:Creature Dragon\nPT:4/4\n" +
		"T:Mode$ SpellCast | ValidCard$ Creature | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Static$ True | Execute$ Body\n" +
		"SVar:Body:DB$ Animate | Defined$ TriggeredCardLKICopy | " + body + "\n" +
		"Oracle:x\n"
}

// castSpellCarryShape casts the Bears under watcher and returns the engine,
// the card id and the spell's object id while it is still on the stack.
func castSpellCarryShape(t *testing.T, watcher string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := handEngine(t, card(t, spellCarryShapeBears))
	onBoard(t, e, 0, watcher)
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MG] = 2
	e.priorityRound()
	submitChoices(t, e, passToCast(t, e, id))
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %v, want the Grizzly Bears spell", e.G.Stack)
	}
	return e, id, e.G.Stack[0]
}

func spellCarryShapeHasType(types []string, want string) bool {
	for _, w := range types {
		if strings.EqualFold(w, want) {
			return true
		}
	}
	return false
}

// TestSpellCarryOnlyTimedKeyword pins the CR 400.7a stack-to-battlefield carry
// to the one Animate shape dirac mirrors: the default until-end-of-turn,
// Keywords$-only grant on the spell (Thundermane Dragon's static SpellCast
// rider). Every other Animate on a spell still ends at the stack -- a
// duration rider, a colour/type/P-T rider, or a permanent keyword grant.
// Widening the engine gate without a matching dirac lowering diverges on
// Wickerwing Effigy (a permanent bird/colour/1-1 Animate), so each wider
// shape is asserted here not to reach the entering permanent at all.
func TestSpellCarryOnlyTimedKeyword(t *testing.T) {
	t.Parallel()
	t.Run("timed keyword grants the permanent", func(t *testing.T) {
		e, id, spell := castSpellCarryShape(t, spellCarryShapeWatcher("Timed Keyword Watcher", "Keywords$ Haste"))
		if !e.HasKeyword(spell, "Haste") {
			t.Fatalf("precondition: the rider did not grant the spell haste (derived %v)", e.Derived(spell).Keywords)
		}
		passUntilStackEmpty(t, e, 40)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Grizzly Bears = %+v, want on the battlefield", e.G.Obj(id))
		}
		if !e.HasKeyword(id, "Haste") {
			t.Fatalf("the timed keyword grant did not carry onto the permanent (derived %v)", e.Derived(id).Keywords)
		}
		akBeginNextTurn(t, e)
		if e.HasKeyword(id, "Haste") {
			t.Fatalf("the carried haste survived the turn boundary (derived %v)", e.Derived(id).Keywords)
		}
	})
	t.Run("timed characteristics stay at the stack", func(t *testing.T) {
		e, id, spell := castSpellCarryShape(t, spellCarryShapeWatcher("Timed Characteristics Watcher",
			"Colors$ Black | Types$ Bird | Keywords$ Flying | Power$ 1 | Toughness$ 1"))
		if d := e.Derived(spell); d.Power != 1 || d.Toughness != 1 || !e.HasKeyword(spell, "Flying") {
			t.Fatalf("precondition: the rider did not animate the spell (%+v)", d)
		}
		passUntilStackEmpty(t, e, 40)
		d := e.Derived(id)
		if d.Power != 2 || d.Toughness != 2 || e.HasKeyword(id, "Flying") || d.Colors != "G" ||
			spellCarryShapeHasType(d.Types, "Bird") {
			t.Fatalf("the spell's colour/type/P-T animation reached the permanent: %+v", d)
		}
	})
	t.Run("permanent keyword stays at the stack", func(t *testing.T) {
		e, id, spell := castSpellCarryShape(t, spellCarryShapeWatcher("Permanent Keyword Watcher",
			"Keywords$ Haste | Duration$ Permanent"))
		if !e.HasKeyword(spell, "Haste") {
			t.Fatalf("precondition: the rider did not grant the spell haste (derived %v)", e.Derived(spell).Keywords)
		}
		passUntilStackEmpty(t, e, 40)
		if e.HasKeyword(id, "Haste") {
			t.Fatalf("the permanent keyword grant carried onto the permanent (derived %v)", e.Derived(id).Keywords)
		}
	})
}
