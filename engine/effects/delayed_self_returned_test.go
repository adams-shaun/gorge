package effects

// This file pins the one-home Defined$ Self resolver against CR 400.7 /
// 603.7c: a delayed triggered ability that refers to "this object" does not
// affect it once the object left its zone and returned (a new object).
// CorrectedSelf is Forge's identity-corrected source and keeps the raw id,
// because it exists to re-fetch a card across its own self-exile
// (Shorecrasher Elemental's DBReturn).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestDelayedSelfReturnedSource(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Returned Self\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	h.g.Obj(src.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
	if o := h.g.Obj(src.ID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source object zone = %v, want battlefield", o)
	}
	// The resolving ability's stack object: pushed while the source stood on
	// the battlefield (the delayed-trigger shape).
	stack := h.g.AddObject(mkCard(t, "Name:Returned Self Ability\nTypes:Instant\nOracle:x\n"), 0)
	stack.PushSourceStamped, stack.PushSourceOnBattlefield = true, true
	c := &Ctx{Source: src.ID, Controller: 0, ResolvingObj: stack.ID}

	// The same spellings that the code table maps to the Self case: a source
	// still on its battlefield incarnation names itself.
	for _, spec := range []string{"Self", "Parent", "EffectSource"} {
		ts := DefinedSpec(h, c, spec)
		if len(ts) != 1 || ts[0].Obj != src.ID {
			t.Fatalf("live %q = %v, want the raw source %d", spec, ts, src.ID)
		}
	}

	// The source left and returned, so it is a new object (CR 400.7). The
	// flag that the self-referent readers consult changes, and must change
	// this answer too.
	c.SourceReturned = true
	for _, spec := range []string{"Self", "Parent", "EffectSource"} {
		if ts := DefinedSpec(h, c, spec); len(ts) != 0 {
			t.Fatalf("returned-source %q = %v, want no target (CR 603.7c)", spec, ts)
		}
	}

	// CorrectedSelf is the identity-corrected fetch: the raw id stays legal
	// even when the source is a new object.
	if ts := DefinedSpec(h, c, "CorrectedSelf"); len(ts) != 1 || ts[0].Obj != src.ID {
		t.Fatalf("returned-source CorrectedSelf = %v, want the raw source %d", ts, src.ID)
	}

	// An ability pushed while the source was OFF the battlefield (Winter
	// Soldier's graveyard activation returning itself) did not lose a
	// battlefield incarnation: entering is the corrected identity, so Self
	// still names the card even though SourceReturned is set.
	stack.PushSourceOnBattlefield = false
	if ts := DefinedSpec(h, c, "Self"); len(ts) != 1 || ts[0].Obj != src.ID {
		t.Fatalf("off-battlefield-push Self = %v, want the raw source %d", ts, src.ID)
	}
}
