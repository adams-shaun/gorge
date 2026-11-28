package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// mutateGrantLordSrc grants kw to every creature its controller controls
// (layer 6), the pile included.
func mutateGrantLordSrc(kw string) string {
	return "Name:Keyword Lord\nManaCost:2\nTypes:Enchantment\n" +
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ " + kw + " | Description$ x\nOracle:x\n"
}

// TestMutatedPileUnderCardAndGrantedKeywordTriggerTwice: a keyword printed on a
// card beneath the top AND granted by an effect is two instances (CR 113.2c,
// 702.105b, 702.149b, 702.24b), so it triggers twice: the under card's own
// expansion queues one, the granted walk the other. The under card's line
// accounts for exactly one derived entry, not every equal one.
func TestMutatedPileUnderCardAndGrantedKeywordTriggerTwice(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, kw string
		fire     func(t *testing.T, e *Engine, pile state.ObjID)
	}{
		{"dethrone", "Dethrone", func(t *testing.T, e *Engine, pile state.ObjID) {
			grantedPileAttack(t, e, pile)
		}},
		{"training", "Training", func(t *testing.T, e *Engine, pile state.ObjID) {
			big := putToken(t, e, 0, "Name:Big Ox\nManaCost:4 G\nTypes:Creature Ox\nPT:6/6\nOracle:x\n", state.ZBattlefield)
			if e.Power(big) <= e.Power(pile) {
				t.Fatalf("precondition: the Ox %d is not stronger than the pile %d", e.Power(big), e.Power(pile))
			}
			grantedPileAttack(t, e, pile, big)
		}},
		{"ward", "Ward:2", func(t *testing.T, e *Engine, pile state.ObjID) {
			cause := e.G.Zone(state.ZLibrary, 1)[0]
			e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
			e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{pile}})
		}},
		{"cumulative upkeep", "Cumulative upkeep:1", func(t *testing.T, e *Engine, pile state.ObjID) {
			e.G.Active, e.G.Priority = 0, 0
			e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
		}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, pile := mutatedKeywordPile(t, 461+uint64(i), "K:"+tc.kw+"\n")
			onBoard(t, e, 0, mutateGrantLordSrc(tc.kw))
			n := 0
			for _, k := range e.Derived(pile).Keywords {
				if k == tc.kw {
					n++
				}
			}
			if n != 2 {
				t.Fatalf("precondition: derived keywords %q hold %q %d times, want 2 (printed beneath + granted)",
					e.Derived(pile).Keywords, tc.kw, n)
			}
			if slices.Contains(e.G.Obj(pile).Face().Keywords, tc.kw) {
				t.Fatalf("precondition: the top card prints %q", tc.kw)
			}
			from := len(e.L.Events)
			tc.fire(t, e, pile)
			if got := queuedSince(e, pile, from); got != 2 {
				t.Fatalf("%s printed beneath the top and granted queued %d triggers, want 2", tc.kw, got)
			}
		})
	}
}

func grantedPileAttack(t *testing.T, e *Engine, atk ...state.ObjID) {
	t.Helper()
	declareAttackersOnly(t, e, 0, 1, atk...)
}

// TestMutatedPileUnderCardKeywordGrantedAfterAbilityLoss: once a pile loses
// all abilities (CR 613.1f) the line printed beneath the top is gone, so a
// later grant of the same keyword is the only instance and triggers once: the
// dead under line must not account for it.
func TestMutatedPileUnderCardKeywordGrantedAfterAbilityLoss(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, kw string
		fire     func(t *testing.T, e *Engine, pile state.ObjID)
	}{
		{"firebending", "Firebending:1", func(t *testing.T, e *Engine, pile state.ObjID) {
			grantedPileAttack(t, e, pile)
		}},
		{"melee", "Melee", func(t *testing.T, e *Engine, pile state.ObjID) {
			grantedPileAttack(t, e, pile)
		}},
		{"exalted", "Exalted", func(t *testing.T, e *Engine, pile state.ObjID) {
			grantedPileAttack(t, e, pile)
		}},
		{"dethrone", "Dethrone", func(t *testing.T, e *Engine, pile state.ObjID) {
			grantedPileAttack(t, e, pile)
		}},
		{"training", "Training", func(t *testing.T, e *Engine, pile state.ObjID) {
			big := putToken(t, e, 0, "Name:Big Ox\nManaCost:4 G\nTypes:Creature Ox\nPT:6/6\nOracle:x\n", state.ZBattlefield)
			if e.Power(big) <= e.Power(pile) {
				t.Fatalf("precondition: the Ox %d is not stronger than the pile %d", e.Power(big), e.Power(pile))
			}
			grantedPileAttack(t, e, pile, big)
		}},
		{"ward", "Ward:2", func(t *testing.T, e *Engine, pile state.ObjID) {
			cause := e.G.Zone(state.ZLibrary, 1)[0]
			e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
			e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{pile}})
		}},
		{"cumulative upkeep", "Cumulative upkeep:1", func(t *testing.T, e *Engine, pile state.ObjID) {
			e.G.Active, e.G.Priority = 0, 0
			e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
		}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, pile := mutatedKeywordPile(t, 471+uint64(i), "K:"+tc.kw+"\n")
			if !slices.Contains(e.Derived(pile).Keywords, tc.kw) {
				t.Fatalf("precondition: derived keywords %q lack %q printed beneath the top", e.Derived(pile).Keywords, tc.kw)
			}
			onBoard(t, e, 0, mutateAbilityLossSrc)
			if slices.Contains(e.Derived(pile).Keywords, tc.kw) || !e.printedAbilitiesGone(e.G.Obj(pile)) {
				t.Fatalf("precondition: the pile kept its printed abilities: %q", e.Derived(pile).Keywords)
			}
			// The newer grant survives the older removal (layer 6, timestamps).
			onBoard(t, e, 0, mutateGrantLordSrc(tc.kw))
			n := 0
			for _, k := range e.Derived(pile).Keywords {
				if k == tc.kw {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("precondition: derived keywords %q hold %q %d times, want only the grant", e.Derived(pile).Keywords, tc.kw, n)
			}
			from := len(e.L.Events)
			tc.fire(t, e, pile)
			if got := queuedSince(e, pile, from); got != 1 {
				t.Fatalf("%s granted after the pile lost its abilities queued %d triggers, want 1", tc.kw, got)
			}
		})
	}
}

// mutateAbilityLossSrc removes all abilities from every creature (layer 6).
const mutateAbilityLossSrc = "Name:Stripping Fog\nManaCost:2\nTypes:Enchantment\n" +
	"S:Mode$ Continuous | Affected$ Creature | RemoveAllAbilities$ True | Description$ x\nOracle:x\n"
