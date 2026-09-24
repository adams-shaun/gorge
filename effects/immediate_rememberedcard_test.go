package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestImmediateTriggerRememberedCardDoesNotNote pins that a corpus
// ImmediateTrigger whose RememberObjects$ names the object-only predicate
// form "RememberedCard" is RECOGNISED by the RememberObjects$ switch in
// effects/immediate.go -- it takes the whole-parent-set arm (no Note) and
// the Execute$ body still runs against the remembered object.
//
// The corpus shape (mount_velus_manticore.txt):
//
//	SVar:DBImmediateTrigger:DB$ ImmediateTrigger | ConditionDefined$ Remembered |
//	  ConditionPresent$ Card | ConditionCompare$ GE1 | Execute$ TrigDamage |
//	  RememberObjects$ RememberedCard | SubAbility$ DBCleanup | TriggerDescription$ ...
//
// and nine further carriers (cait_sith_fortune_teller, fiery_encore,
// god_eternal_kefnet, hidetsugu_devouring_chaos, lavabrink_floodgates,
// pyroclastic_hellion, superior_spider_man,
// torrent_sculptor_flamethrower_sonata, wall_of_stolen_identity) all spell the
// value the same way. This test drives the recogniser directly (not the full
// card: mount_velus_manticore additionally needs the unread
// TriggerRemembered$CardTypes property, a separate gap).
func TestImmediateTriggerRememberedCardDoesNotNote(t *testing.T) {
	h, c := immHost(t)
	toBattlefield(h, c.Source)
	c.Remembered = []state.Target{{Obj: c.Source}}

	// Precondition: the remembered object really is on the battlefield in the
	// zone DealDamage lands on, and nothing has damaged it yet -- so the
	// assertions below cannot pass vacuously.
	if obj := h.Game().Obj(c.Source); obj == nil || obj.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source %d not on the battlefield (obj %+v)", c.Source, obj)
	}
	if n := countDamageTo(h, c.Source); n != 0 {
		t.Fatalf("precondition: %d Damage events before the resolve", n)
	}

	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Hit", "RememberObjects": "RememberedCard"}})

	// The ticket's whole claim: the value must NOT reach the loud default arm.
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			t.Fatalf("RememberedCard emitted Note %q; the value is recognised", ev.Text)
		}
	}
	// The Execute body ran against the remembered object -- so the arm was
	// taken as the whole-parent-set read, not silently ignored.
	if n := countDamageTo(h, c.Source); n != 1 {
		t.Fatalf("%d Damage events on the remembered object, want 1", n)
	}
}

// TestImmediateTriggerRememberedCardSeesWholeParentSet pins the documented
// reading for the recognised "RememberedCard" arm: every instance sees the
// whole parent remembered set (the switch's second case), matching the
// "Remembered" arm -- unlike RememberEach$.
func TestImmediateTriggerRememberedCardSeesWholeParentSet(t *testing.T) {
	h, c := immHost(t)
	toBattlefield(h, c.Source)
	src2 := h.Game().Obj(2).ID
	toBattlefield(h, src2)
	c.Remembered = []state.Target{{Obj: c.Source}, {Obj: src2}}

	if obj := h.Game().Obj(src2); obj == nil || obj.Zone != state.ZBattlefield {
		t.Fatalf("precondition: second object %d not on the battlefield (obj %+v)", src2, obj)
	}
	if countDamageTo(h, c.Source) != 0 || countDamageTo(h, src2) != 0 {
		t.Fatalf("precondition: damage before the resolve (source %d, obj2 %d)",
			countDamageTo(h, c.Source), countDamageTo(h, src2))
	}

	Resolve(h, c, &cards.SA{Kind: "DB", API: "ImmediateTrigger",
		Params: map[string]string{"Execute": "Hit", "RememberObjects": "RememberedCard"}})

	for _, ev := range h.log {
		if ev.Kind == events.Note {
			t.Fatalf("RememberedCard emitted Note %q; the value is recognised", ev.Text)
		}
	}
	if n := countDamageTo(h, c.Source); n != 1 || countDamageTo(h, src2) != 1 {
		t.Fatalf("whole-parent-set instance expected (source %d, obj2 %d), want 1 and 1",
			countDamageTo(h, c.Source), countDamageTo(h, src2))
	}
}
