package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// A delayed-trigger registration saves the target set it captured at
// registration time (TriggerContext.DelayedRemembered) and seeds that same set
// into BOTH Ctx.Remembered and Ctx.Captured: a Mode$ Phase registration has no
// firing event, so its saved list IS the referent. resolvedRemembered must not
// treat that as a disposable firing-event referent and replace it with the
// source's later mutable remembered list.
//
// The regression this guards is Turn to Mist: the original ChangeZone remembers
// the exiled creature, the delayed `TrigReturn: ChangeZone | Defined$
// Remembered` returns it at end of turn, and recasting the same card before the
// end step clears/replaces the source's persistent memory. Before the fix the
// delayed body resolved to nothing (cleared) or to the NEW target (replaced)
// instead of the creature its own registration captured.
//
// Each case asserts its precondition -- the source's persistent list really
// differs from the delayed capture -- so a vacuous fixture fails loudly rather
// than passing on an empty list.
func TestDelayedRegistrationCaptureSurvivesSourceMemory(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:Exiled\nManaCost:3\nTypes:Creature\nPT:2/2\nOracle:x\n")
	sourceCard := mkCard(t, "Name:Turn to Mist\nManaCost:2\nTypes:Instant\nOracle:x\n")
	oldID := h.g.AddObject(card, 1).ID
	newerID := h.g.AddObject(card, 1).ID
	srcID := h.g.AddObject(sourceCard, 0).ID
	// AddObject returns a pointer into a slice that later AddObject calls may
	// reallocate, so every field write goes through a fresh Obj lookup.
	h.g.Obj(oldID).Zone = state.ZExile
	h.g.Obj(newerID).Zone = state.ZExile
	h.g.Obj(srcID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZExile, 1, []state.ObjID{oldID, newerID})

	cases := []struct {
		name string
		// persistent is the source's remembered list at resolution time.
		persistent []state.Target
		read       string
		// want is the object the read must resolve to.
		want state.ObjID
	}{
		{"cleared memory, bare read", nil, "Remembered", oldID},
		{"replaced memory, bare read", []state.Target{{Obj: newerID}}, "Remembered", oldID},
		{"cleared memory, dotted read", nil, "Remembered.Creature", oldID},
		{"replaced memory, dotted read", []state.Target{{Obj: newerID}}, "Remembered.Creature", oldID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := h.g.Obj(srcID)
			src.Remembered = append([]state.Target(nil), tc.persistent...)
			capture := []state.Target{{Obj: oldID}}
			ctx := &Ctx{
				Source:     srcID,
				Controller: 0,
				Remembered: append([]state.Target(nil), capture...),
				Captured:   append([]state.Target(nil), capture...),
				TriggerContext: TriggerContext{
					DelayedRemembered: append([]state.Target(nil), capture...),
				},
			}
			// Preconditions: the saved registration target really exists where
			// the read assumes, and the source's persistent list really differs
			// from the delayed capture (or the test proves nothing).
			if o := h.g.Obj(oldID); o == nil || o.Zone != state.ZExile {
				t.Fatalf("precondition: saved delayed target not in exile")
			}
			if containsTarget(src.Remembered, capture[0]) {
				t.Fatalf("precondition: source persistent memory still contains the saved target: %+v", src.Remembered)
			}
			got := Defined(h, ctx, sa(t, "SP$ X | Defined$ "+tc.read))
			if len(got) != 1 || got[0].IsPlayer || got[0].Obj != tc.want {
				t.Fatalf("Defined$ %s -> %+v, want the registration's saved target %d", tc.read, got, tc.want)
			}
		})
	}
}

// The delayed exception keys on Captured aliasing the registration capture.
// An event-matched delayed registration seeds Ctx.Captured with the firing
// EVENT's object and carries the registration's own set in DelayedRemembered --
// a different list -- so it keeps the ordinary referent treatment: a firing
// referent the source never remembered is still dropped.
func TestEventMatchedDelayedReferentIsStillFiltered(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:Creature\nManaCost:3\nTypes:Creature\nPT:2/2\nOracle:x\n")
	sourceCard := mkCard(t, "Name:Watcher\nManaCost:2\nTypes:Enchantment\nOracle:x\n")
	referentID := h.g.AddObject(card, 0).ID
	savedID := h.g.AddObject(card, 0).ID
	srcID := h.g.AddObject(sourceCard, 0).ID
	for _, id := range []state.ObjID{referentID, savedID, srcID} {
		h.g.Obj(id).Zone = state.ZBattlefield
	}

	// The firing referent was never remembered by the source; the registration
	// captured a different object. The referent must not resolve.
	ctx := &Ctx{
		Source:     srcID,
		Controller: 0,
		Remembered: []state.Target{{Obj: referentID}},
		Captured:   []state.Target{{Obj: referentID}},
		TriggerContext: TriggerContext{
			DelayedRemembered: []state.Target{{Obj: savedID}},
		},
	}
	if sameTargets(ctx.Captured, ctx.DelayedRemembered) {
		t.Fatal("precondition: event-matched firing referent aliases the registration capture")
	}
	if containsTarget(h.g.Obj(srcID).Remembered, state.Target{Obj: referentID}) {
		t.Fatal("precondition: source remembers the firing referent")
	}
	got := Defined(h, ctx, sa(t, "SP$ X | Defined$ Remembered"))
	if len(got) != 0 {
		t.Fatalf("Defined$ Remembered -> %+v, want the unremembered firing referent dropped", got)
	}
}
