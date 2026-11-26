package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedPersistAndUndyingReturn: a layer-6 grant of Undying (Mikaeus,
// the Unhallowed's static AddKeyword$ Undying, CR 702.93a) or Persist
// (Cauldron of Souls' Pump KW$ Persist, CR 702.79a) gives the creature the
// printed keyword's dies trigger (613.1f). The granted walk must queue the
// printed expansion's look-back trigger: the creature returns once with the
// keyword's counter, and a second death (now carrying that counter) leaves it
// in the graveyard. An ungranted creature stays dead, and the log replays to
// the identical game.
func TestGrantedPersistAndUndyingReturn(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, granter, counter string
		pump                   bool
	}{
		{"undying", "Name:Lord\nManaCost:3 B\nTypes:Creature Zombie\nPT:5/5\n" +
			"S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl+namedBear | AddKeyword$ Undying | Description$ x\nOracle:x\n", "P1P1", false},
		{"persist", "Name:Cauldron\nManaCost:5\nTypes:Artifact\n" +
			"A:AB$ Pump | Cost$ 0 | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 1 | KW$ Persist | SpellDescription$ x\nOracle:x\n", "M1M1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bear := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:3/3\nOracle:x\n"
			wolf := "Name:Wolf\nManaCost:1 G\nTypes:Creature Wolf\nPT:2/2\nOracle:x\n"
			e, cfg, _ := newFixtureDeck(t, 91, tc.granter, bear, wolf)
			src := putCreature(t, e, 0, tc.granter)
			b := putCreature(t, e, 0, bear)
			w := putCreature(t, e, 0, wolf)
			if tc.pump {
				f := e.G.Obj(src).Face()
				e.resolveAbility(src, 0, []state.Target{{Obj: b}}, f.Abilities[0], f.SVars)
			}
			want := "Undying"
			if tc.pump {
				want = "Persist"
			}
			if !e.HasKeyword(b, want) || e.HasKeyword(w, want) {
				t.Fatalf("precondition: the grant reaches the bear only: bear %v wolf %v", e.Derived(b).Keywords, e.Derived(w).Keywords)
			}
			// Control: the ungranted wolf stays dead.
			e.emit(events.Event{Kind: events.Damage, Obj: w, Amount: 5})
			e.checkStateBased()
			e.priorityRound()
			passUntilStackEmpty(t, e, 20)
			if e.G.Obj(w).Zone != state.ZGraveyard {
				t.Fatalf("an ungranted creature is in %s after dying, want the graveyard", e.G.Obj(w).Zone)
			}
			e.emit(events.Event{Kind: events.Damage, Obj: b, Amount: 5})
			e.checkStateBased()
			e.priorityRound()
			passUntilStackEmpty(t, e, 20)
			if o := e.G.Obj(b); o.Zone != state.ZBattlefield || o.Counter(tc.counter) != 1 {
				t.Fatalf("granted %s: after the first death the bear is in %s with %d %s counters, want the battlefield with 1",
					want, o.Zone, o.Counter(tc.counter), tc.counter)
			}
			pushes := 0
			for _, ev := range e.L.Events {
				if ev.Kind == events.GrantTriggerPush {
					pushes++
				}
			}
			if pushes != 1 {
				t.Fatalf("granted %s pushed %d triggers, want 1 (read once, in the look-back pass)", want, pushes)
			}
			if !tc.pump {
				// The static still grants Undying; a creature carrying its
				// +1/+1 counter does not return (Persist's until-end-of-turn
				// grant ended with the object that left).
				e.emit(events.Event{Kind: events.Damage, Obj: b, Amount: 9})
				e.checkStateBased()
				e.priorityRound()
				passUntilStackEmpty(t, e, 20)
				if e.G.Obj(b).Zone != state.ZGraveyard {
					t.Fatal("granted undying returned a creature that had a +1/+1 counter")
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}
