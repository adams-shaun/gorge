package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestChangeZoneAllForgetOtherRememberedFlagGating pins the absent/false
// behaviour of the ChangeZoneAll batch sweep's ForgetOtherRemembered$ rider
// (ticket: Implement ForgetOtherRemembered$ for ChangeZoneAll). The sibling
// TestChangeZoneAllForgetOtherRememberedReplacesTheSet proves the True case
// replaces the remembered set with exactly the moved cards; this one proves
// an ABSENT or explicit-False parameter does not: no clear-remembered event
// is emitted and the stale source memory survives the sweep.
//
// The stale object is remembered but sits in Exile, outside the sweep's
// Origin$ Graveyard, so it is never moved either way -- the only thing the
// flag can change is whether it survives in the remembered set. The two
// moved creatures ARE remembered and match ChangeType$ Card.IsRemembered, so
// the sweep is non-vacuous and the filter must see the pre-clear set.
func TestChangeZoneAllForgetOtherRememberedFlagGating(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flag  string
		clear bool
	}{
		{"Absent", "", false},
		{"False", " | ForgetOtherRemembered$ False", false},
		{"True", " | ForgetOtherRemembered$ True", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, src, cards := forgetFixtureHost(t, "Moved One", "Moved Two", "Stale")
			first, second, stale := cards[0], cards[1], cards[2]
			h.g.SetZone(state.ZGraveyard, 0, []state.ObjID{first.ID, second.ID})
			first.Zone, second.Zone = state.ZGraveyard, state.ZGraveyard
			h.g.SetZone(state.ZExile, 0, []state.ObjID{stale.ID})
			stale.Zone = state.ZExile
			seedRemembered(h, src, first.ID, second.ID, stale.ID)

			// Preconditions: distinct ids in the zones the rule reads, and a
			// persistent set that actually contains all three.
			if first.ID == second.ID || first.ID == stale.ID || second.ID == stale.ID {
				t.Fatal("precondition: fixture ids not distinct")
			}
			if first.Zone != state.ZGraveyard || second.Zone != state.ZGraveyard || stale.Zone != state.ZExile {
				t.Fatalf("precondition: zones %s/%s/%s, want graveyard/graveyard/exile", first.Zone, second.Zone, stale.Zone)
			}
			if got := rememberedIDs(h.g.Obj(src.ID).Remembered); !sameIDs(got, []state.ObjID{first.ID, second.ID, stale.ID}) {
				t.Fatalf("precondition: persistent remembered = %v, want all three", got)
			}
			// The sweep's selector must be non-vacuous: a remembered graveyard
			// creature matches Card.IsRemembered before the clear.
			sc := (&Ctx{Source: src.ID, Controller: 0}).SpecContext(0)
			if !MatchesSpecCtx(h.g, "Card.IsRemembered", first.ID, sc) {
				t.Fatal("precondition: remembered graveyard creature does not match the sweep selector")
			}

			body := "DB$ ChangeZoneAll | Origin$ Graveyard | Destination$ Exile | ChangeType$ Card.IsRemembered" +
				" | RememberChanged$ True" + tc.flag
			parsed := sa(t, body)
			if parsed.API != "ChangeZoneAll" {
				t.Fatalf("precondition: fixture SA parsed as %q, want ChangeZoneAll", parsed.API)
			}
			c := &Ctx{Source: src.ID, Controller: 0,
				Remembered: []state.Target{{Obj: first.ID}, {Obj: second.ID}, {Obj: stale.ID}}}

			before := len(h.log)
			Resolve(h, c, parsed)
			clears := 0
			for _, e := range h.log[before:] {
				if e.Kind == events.Choose && e.Obj == src.ID && e.Counter == "clear-remembered" {
					clears++
				}
			}
			if tc.clear && clears != 1 {
				t.Fatalf("True: clear-remembered events = %d, want exactly 1; log %v", clears, h.log[before:])
			}
			if !tc.clear && clears != 0 {
				t.Fatalf("absent/false: clear-remembered events = %d, want 0; log %v", clears, h.log[before:])
			}

			// The two remembered graveyard creatures moved regardless of the
			// flag; the sweep is not emptied.
			for _, o := range []*state.Object{first, second} {
				if o.Zone != state.ZExile {
					t.Fatalf("%q did not move: zone = %s, want exile", o.Face().Name, o.Zone)
				}
			}
			if stale.Zone != state.ZExile {
				t.Fatalf("the unmoved stale card was disturbed: zone = %s", stale.Zone)
			}

			persistent := rememberedIDs(h.g.Obj(src.ID).Remembered)
			ctxMem := rememberedIDs(c.Remembered)
			if tc.clear {
				if !sameIDs(persistent, []state.ObjID{first.ID, second.ID}) {
					t.Errorf("True: persistent remembered = %v, want exactly the moved pair", persistent)
				}
				if !sameIDs(ctxMem, []state.ObjID{first.ID, second.ID}) {
					t.Errorf("True: ctx remembered = %v, want exactly the moved pair", ctxMem)
				}
			} else {
				// Absent/False: no clear, so the stale entry survives and the
				// moved pair is appended on top of it.
				if !containsID(persistent, stale.ID) {
					t.Errorf("absent/false: stale memory was dropped from persistent set %v", persistent)
				}
				if !containsID(ctxMem, stale.ID) {
					t.Errorf("absent/false: stale memory was dropped from ctx set %v", ctxMem)
				}
				if !containsID(persistent, first.ID) || !containsID(persistent, second.ID) {
					t.Errorf("absent/false: moved members missing from persistent set %v", persistent)
				}
			}
		})
	}
}
