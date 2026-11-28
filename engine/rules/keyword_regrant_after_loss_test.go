package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestKeywordRegrantedAfterAbilityLoss: a line the permanent prints on its own
// face does not exist once it loses all abilities (CR 613.1f), so a newer grant
// of the same line (layer 6, timestamp order) is the only instance and works:
// the dead printed line must not own it (CR 113.2c).
func TestKeywordRegrantedAfterAbilityLoss(t *testing.T) {
	t.Parallel()
	t.Run("triggered", testRegrantedTriggeredKeyword)
	t.Run("activated", testRegrantedActivatedKeyword)
}

// regrantedKeywordPermanent puts a permanent of the given types on seat 0,
// printing kw when printed, strips every ability of the permanents removed
// names, then grants kw to the same set with a newer static.
func regrantedKeywordPermanent(t *testing.T, e *Engine, types, affected, kw string, printed bool) state.ObjID {
	t.Helper()
	line := ""
	if printed {
		line = "K:" + kw + "\n"
	}
	id := onBoard(t, e, 0, "Name:Keyword Bearer\nManaCost:1 G\nTypes:"+types+"\nPT:2/2\n"+line+"Oracle:x\n")
	if got := slices.Contains(e.Derived(id).Keywords, kw); got != printed {
		t.Fatalf("precondition: derived keywords %q hold %q = %v, want %v", e.Derived(id).Keywords, kw, got, printed)
	}
	onBoard(t, e, 0, "Name:Stripping Fog\nManaCost:2\nTypes:Enchantment\n"+
		"S:Mode$ Continuous | Affected$ "+affected+" | RemoveAllAbilities$ True | Description$ x\nOracle:x\n")
	if slices.Contains(e.Derived(id).Keywords, kw) || !e.printedAbilitiesGone(e.G.Obj(id)) {
		t.Fatalf("precondition: the permanent kept its printed abilities: %q", e.Derived(id).Keywords)
	}
	onBoard(t, e, 0, "Name:Keyword Lord\nManaCost:2\nTypes:Enchantment\n"+
		"S:Mode$ Continuous | Affected$ "+affected+".YouCtrl | AddKeyword$ "+kw+" | Description$ x\nOracle:x\n")
	n := 0
	for _, k := range e.Derived(id).Keywords {
		if k == kw {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("precondition: derived keywords %q hold %q %d times, want only the grant", e.Derived(id).Keywords, kw, n)
	}
	return id
}

// stackAbilitiesFrom counts the ability objects on the stack sourced at id.
func stackAbilitiesFrom(e *Engine, id state.ObjID) int {
	n := 0
	for _, sid := range e.G.Stack {
		if o := e.G.Obj(sid); o != nil && o.Ability != nil && o.Source == id {
			n++
		}
	}
	return n
}

func testRegrantedTriggeredKeyword(t *testing.T) {
	t.Parallel()
	attack := func(t *testing.T, e *Engine, id state.ObjID) int {
		from := len(e.L.Events)
		declareAttackersOnly(t, e, 0, 1, id)
		return from
	}
	cases := []struct {
		kw string
		// fire emits the triggering event and returns the log position before it.
		fire func(t *testing.T, e *Engine, id state.ObjID) int
	}{
		{"Firebending:1", attack},
		{"Dethrone", attack},
		{"Training", func(t *testing.T, e *Engine, id state.ObjID) int {
			big := putToken(t, e, 0, "Name:Big Ox\nManaCost:4 G\nTypes:Creature Ox\nPT:6/6\nOracle:x\n", state.ZBattlefield)
			if e.Power(big) <= e.Power(id) {
				t.Fatalf("precondition: the Ox %d is not stronger than the bearer %d", e.Power(big), e.Power(id))
			}
			from := len(e.L.Events)
			declareAttackersOnly(t, e, 0, 1, id, big)
			return from
		}},
		{"Ward:2", func(t *testing.T, e *Engine, id state.ObjID) int {
			cause := e.G.Zone(state.ZLibrary, 1)[0]
			e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
			from := len(e.L.Events)
			e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{id}})
			return from
		}},
		{"Cumulative upkeep:1", func(t *testing.T, e *Engine, id state.ObjID) int {
			e.G.Active, e.G.Priority = 0, 0
			from := len(e.L.Events)
			e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
			return from
		}},
		{"Melee", attack},
		{"Exalted", attack},
		{"Afflict:1", func(t *testing.T, e *Engine, id state.ObjID) int {
			blocker := onBoard(t, e, 1, "Name:Blocking Ox\nManaCost:G\nTypes:Creature Ox\nPT:1/4\nOracle:x\n")
			declareAttackersOnly(t, e, 0, 1, id)
			from := len(e.L.Events)
			e.emit(events.Event{Kind: events.DeclareBlockers, Player: 1, Pairs: [][2]state.ObjID{{id, blocker}}})
			return from
		}},
	}
	for i, tc := range cases {
		for j, printed := range []bool{true, false} {
			name := tc.kw + "/control"
			if printed {
				name = tc.kw + "/printed"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				e, _ := tokenReplGame(t, 4801+uint64(10*i+j))
				id := regrantedKeywordPermanent(t, e, "Creature Bear", "Creature", tc.kw, printed)
				from := tc.fire(t, e, id)
				if got := queuedSince(e, id, from); got != 1 {
					t.Fatalf("%s granted after the ability loss queued %d triggers, want 1", tc.kw, got)
				}
				e.putTriggersOnStack()
				if got := stackAbilitiesFrom(e, id); got != 1 {
					t.Fatalf("%s put %d abilities on the stack, want 1", tc.kw, got)
				}
			})
		}
	}
}

// testRegrantedActivatedKeyword is the activated half: the offer walk must not
// read the dead printed line as covering the granted one.
func testRegrantedActivatedKeyword(t *testing.T) {
	t.Parallel()
	cases := []struct{ kw, types, affected string }{
		{"Crew:1", "Artifact Vehicle", "Artifact"},
		{"Saddle:1", "Creature Horse Mount", "Creature"},
	}
	for i, tc := range cases {
		for j, printed := range []bool{true, false} {
			name := tc.kw + "/control"
			if printed {
				name = tc.kw + "/printed"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				e, _ := tokenReplGame(t, 4901+uint64(10*i+j))
				toMain1(t, e)
				id := regrantedKeywordPermanent(t, e, tc.types, tc.affected, tc.kw, printed)
				rider := onBoard(t, e, 0, "Name:Keyword Rider\nManaCost:G\nTypes:Creature Elf\nPT:2/2\nOracle:x\n")
				e.pending = nil
				e.priorityRound()
				d := e.Pending()
				if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
					t.Fatalf("precondition: want seat 0's priority, got %+v", d)
				}
				var offered []decision.Option
				for _, op := range d.Options {
					if op.Kind == "ability" && op.Obj == id {
						offered = append(offered, op)
					}
				}
				if len(offered) != 1 {
					t.Fatalf("%s granted after the ability loss is offered %d times, want 1: %+v", tc.kw, len(offered), d.Options)
				}
				submitChoices(t, e, offered[0].Index)
				d = e.Pending()
				if d == nil || d.Kind != decision.KChoose {
					t.Fatalf("pending decision %+v, want the tap election", d)
				}
				pick := slices.IndexFunc(d.Options, func(op decision.Option) bool { return op.Obj == rider })
				if pick < 0 {
					t.Fatalf("the tap election does not offer the rider %d: %+v", rider, d.Options)
				}
				submitChoices(t, e, d.Options[pick].Index)
				if got := stackAbilitiesFrom(e, id); got != 1 {
					t.Fatalf("%s put %d abilities on the stack, want 1", tc.kw, got)
				}
			})
		}
	}
}
