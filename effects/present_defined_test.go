package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// corpusStaffRoomBody returns the REAL SVar body text of one DB effect on
// Staff Room's face (the split half of Experimental Lab // Staff Room), the
// corpus's ONLY `DB$ ... PresentDefined$` carrier. It asserts the body
// really carries both the group (PresentDefined$) and the filter
// (IsPresent$) under test -- a caller can never be handed a body that only
// looks like the shape. The body is fed to sa() so it is parsed by the same
// corpus parser the engine uses.
func corpusStaffRoomBody(t *testing.T, svar string) string {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup("Experimental Lab // Staff Room")
	if !ok {
		t.Fatal("corpus has no Experimental Lab // Staff Room")
	}
	for _, f := range c.Faces {
		body, ok := f.SVars[svar]
		if !ok {
			continue
		}
		if !strings.Contains(body, "PresentDefined$ TriggeredSourceLKICopy") {
			t.Fatalf("%s body lost its PresentDefined$ group: %q", svar, body)
		}
		if !strings.Contains(body, "IsPresent$ ") {
			t.Fatalf("%s body lost its IsPresent$ filter: %q", svar, body)
		}
		return body
	}
	t.Fatalf("no face of Experimental Lab // Staff Room defines SVar %q", svar)
	return ""
}

// presentDefinedCtx builds a two-seat host with one object on seat 0's
// battlefield and a Ctx whose TriggerSource names it, the group
// PresentDefined$ TriggeredSourceLKICopy enumerates. It returns the host,
// the id and the Ctx.
func presentDefinedCtx(t *testing.T, types string) (*fakeHost, state.ObjID, *Ctx) {
	t.Helper()
	h := newHost(t, 2)
	card := mkCard(t, "Name:PresentHit\nTypes:"+types+"\nPT:2/2\nOracle:x\n")
	o := h.g.AddObject(card, 0)
	o.Zone = state.ZBattlefield
	if o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: trigger source is not on the battlefield")
	}
	return h, o.ID, &Ctx{Controller: 0, Source: o.ID, TriggerContext: TriggerContext{TriggerSource: o.ID}}
}

// TestPresentDefinedGateDeniesWithZeroMatchingReferents pins the brief's
// zero-referent outcome on the real group (TriggeredSourceLKICopy): the
// resolving source is a LAND, the gate's filter is IsPresent$ Creature, so
// the count over the group is 0 and the gate RESOLVES to not-met -- the body
// is skipped. Both halves of the precondition are asserted (the source is on
// the battlefield, and the filter really excludes it), so a setup that
// accidentally made the land a creature would fail loudly.
func TestPresentDefinedGateDeniesWithZeroMatchingReferents(t *testing.T) {
	h, src, ctx := presentDefinedCtx(t, "Land")
	if o := h.g.Obj(src); o == nil || o.EffectiveIsCreature() {
		t.Fatalf("precondition: filter fixture must be a non-creature on the battlefield")
	}
	gate := sa(t, "DB$ PutCounter | Defined$ TriggeredSourceLKICopy | CounterType$ P1P1 | PresentDefined$ TriggeredSourceLKICopy | IsPresent$ Creature")
	if met, resolved := conditionMet(h, ctx, gate); met || !resolved {
		t.Fatalf("zero matching referents: met=%v resolved=%v, want false true (deny)", met, resolved)
	}
}

// TestPresentDefinedGateRunsWithOneMatchingReferent is the one-referent
// twin: the source IS a creature, so IsPresent$ Creature counts 1 and the
// gate resolves to met -- the body runs.
func TestPresentDefinedGateRunsWithOneMatchingReferent(t *testing.T) {
	h, src, ctx := presentDefinedCtx(t, "Creature")
	if o := h.g.Obj(src); o == nil || !o.EffectiveIsCreature() {
		t.Fatalf("precondition: filter fixture must be a creature on the battlefield")
	}
	gate := sa(t, "DB$ PutCounter | Defined$ TriggeredSourceLKICopy | CounterType$ P1P1 | PresentDefined$ TriggeredSourceLKICopy | IsPresent$ Creature")
	if met, resolved := conditionMet(h, ctx, gate); !met || !resolved {
		t.Fatalf("one matching referent: met=%v resolved=%v, want true true (run)", met, resolved)
	}
}

// TestPresentDefinedGateReadsPresentCompare pins the PresentCompare$ half:
// over the same one-referent creature group, EQ0 denies and EQ1 runs. The
// two compare values differ by construction, so the assertion is
// discriminative.
func TestPresentDefinedGateReadsPresentCompare(t *testing.T) {
	h, _, ctx := presentDefinedCtx(t, "Creature")
	eq0 := sa(t, "DB$ SetState | Defined$ TriggeredSourceLKICopy | Mode$ TurnFaceUp | PresentDefined$ TriggeredSourceLKICopy | IsPresent$ Creature | PresentCompare$ EQ0")
	if met, resolved := conditionMet(h, ctx, eq0); met || !resolved {
		t.Fatalf("EQ0 over one referent: met=%v resolved=%v, want false true", met, resolved)
	}
	eq1 := sa(t, "DB$ SetState | Defined$ TriggeredSourceLKICopy | Mode$ TurnFaceUp | PresentDefined$ TriggeredSourceLKICopy | IsPresent$ Creature | PresentCompare$ EQ1")
	if met, resolved := conditionMet(h, ctx, eq1); !met || !resolved {
		t.Fatalf("EQ1 over one referent: met=%v resolved=%v, want true true", met, resolved)
	}
}

// TestPresentDefinedGateIgnoresThePhantomPresentKey proves the fix replaces
// the phantom `Present$` key (zero corpus occurrences) with the corpus's
// real DB-body spelling `IsPresent$`. A gate carrying ONLY `Present$ Land`
// over the creature source must RUN (the phantom key is not a filter);
// before the fix it was read as one and the gate denied. This test FAILS
// against the pre-fix code, which is what makes it discriminative.
func TestPresentDefinedGateIgnoresThePhantomPresentKey(t *testing.T) {
	h, _, ctx := presentDefinedCtx(t, "Creature")
	gate := sa(t, "DB$ PutCounter | Defined$ TriggeredSourceLKICopy | CounterType$ P1P1 | PresentDefined$ TriggeredSourceLKICopy | Present$ Land | PresentCompare$ EQ1")
	if met, resolved := conditionMet(h, ctx, gate); !met || !resolved {
		t.Fatalf("phantom Present$ Land over a creature: met=%v resolved=%v, want true true (the phantom key is not read)", met, resolved)
	}
}

// TestPresentDefinedGateOverTheRealCorpusBodies pins the real carrier
// (Experimental Lab // Staff Room's DBPutCounter and DBTurnFaceUp, the
// corpus's only two `DB$ ... PresentDefined$` lines). It asserts:
//
//   - the real body carries the group PresentDefined$ TriggeredSourceLKICopy
//     and the real filter key IsPresent$ (the corpus spelling, NOT Present$);
//   - with the body's OWN filter spec (Card.canReceiveCounters P1P1 /
//     Card.canBeTurnedFaceUp+faceDown) the gate is UNRESOLVED, because those
//     two card properties are not modelled by this filter tier -- the
//     documented fail-open convention, so the body still runs exactly as
//     before (see the report's Issues section: implementing those predicates
//     is a separate filter-grammar task);
//   - substituting a readable filter over the SAME real body+group makes the
//     gate discriminative (Land denies, Creature runs), which is the
//     behaviour the fix provides for every future IsPresent$ carrier whose
//     spec this tier does model.
func TestPresentDefinedGateOverTheRealCorpusBodies(t *testing.T) {
	h, _, ctx := presentDefinedCtx(t, "Creature")
	for _, svar := range []string{"DBPutCounter", "DBTurnFaceUp"} {
		body := corpusStaffRoomBody(t, svar)
		base := sa(t, body)
		if strings.TrimSpace(base.Params["PresentDefined"]) != "TriggeredSourceLKICopy" {
			t.Fatalf("%s: PresentDefined = %q, want TriggeredSourceLKICopy", svar, base.Params["PresentDefined"])
		}
		if strings.TrimSpace(base.Params["IsPresent"]) == "" {
			t.Fatalf("%s: real body carries no IsPresent$ filter", svar)
		}

		// The real spec's card property is unmodelled -> unresolved, the
		// documented fail-open, unchanged from the pre-fix path.
		real := *base
		if met, resolved := conditionMet(h, ctx, &real); resolved {
			t.Fatalf("%s: unmodelled real filter resolved (met=%v) -- either the predicate landed, or the gate is misreading the spec", svar, met)
		}

		// Same real body and group, readable filter: deny vs run.
		skip := *base
		skip.Params = cloneParams(base.Params)
		skip.Params["IsPresent"] = "Land"
		if met, resolved := conditionMet(h, ctx, &skip); met || !resolved {
			t.Fatalf("%s: Land filter over a creature: met=%v resolved=%v, want false true (deny)", svar, met, resolved)
		}
		run := *base
		run.Params = cloneParams(base.Params)
		run.Params["IsPresent"] = "Creature"
		if met, resolved := conditionMet(h, ctx, &run); !met || !resolved {
			t.Fatalf("%s: Creature filter over a creature: met=%v resolved=%v, want true true (run)", svar, met, resolved)
		}
	}
}

// TestPresentDefinedGateWithoutTriggerSourceIsUnresolved pins the absent
// group binding: with no TriggerSource there is no referent to count, so the
// gate is unresolved and the body runs unconditionally -- the convention
// every unenumerable defined group in conditionMet follows.
func TestPresentDefinedGateWithoutTriggerSourceIsUnresolved(t *testing.T) {
	h, src, _ := presentDefinedCtx(t, "Creature")
	gate := sa(t, "DB$ PutCounter | Defined$ TriggeredSourceLKICopy | CounterType$ P1P1 | PresentDefined$ TriggeredSourceLKICopy | IsPresent$ Creature")
	ctx := &Ctx{Controller: 0, Source: src} // no TriggerContext.TriggerSource
	if _, resolved := conditionMet(h, ctx, gate); resolved {
		t.Fatal("gate resolved with no TriggerSource binding -- would count a phantom zero")
	}
}

// cloneParams returns a fresh copy of an SA's parameter map so a test can
// vary one key without touching the shared map.
func cloneParams(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
