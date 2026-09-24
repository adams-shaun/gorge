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
// PresentDefined$ TriggeredSourceLKICopy enumerates. faceDown puts the
// object face down (a manifested/cloaked permanent) so the turn-face-up
// filter can match. It returns the host, the object and the Ctx.
func presentDefinedCtx(t *testing.T, types string, faceDown bool) (*fakeHost, *state.Object, *Ctx) {
	t.Helper()
	h := newHost(t, 2)
	card := mkCard(t, "Name:PresentHit\nTypes:"+types+"\nPT:2/2\nOracle:x\n")
	o := h.g.AddObject(card, 0)
	o.Zone = state.ZBattlefield
	o.FaceDown = faceDown
	if o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: trigger source is not on the battlefield")
	}
	return h, o, &Ctx{Controller: 0, Source: o.ID, TriggerContext: TriggerContext{TriggerSource: o.ID}}
}

// TestPresentDefinedGateDeniesWithZeroMatchingReferents pins the brief's
// zero-referent outcome on the real group (TriggeredSourceLKICopy): the
// resolving source is a LAND, the gate's filter is IsPresent$ Creature, so
// the count over the group is 0 and the gate RESOLVES to not-met -- the body
// is skipped. Both halves of the precondition are asserted (the source is on
// the battlefield, and the filter really excludes it), so a setup that
// accidentally made the land a creature would fail loudly.
func TestPresentDefinedGateDeniesWithZeroMatchingReferents(t *testing.T) {
	h, o, ctx := presentDefinedCtx(t, "Land", false)
	if o.EffectiveIsCreature() {
		t.Fatal("precondition: filter fixture must be a non-creature")
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
	h, o, ctx := presentDefinedCtx(t, "Creature", false)
	if !o.EffectiveIsCreature() {
		t.Fatal("precondition: filter fixture must be a creature")
	}
	gate := sa(t, "DB$ PutCounter | Defined$ TriggeredSourceLKICopy | CounterType$ P1P1 | PresentDefined$ TriggeredSourceLKICopy | IsPresent$ Creature")
	if met, resolved := conditionMet(h, ctx, gate); !met || !resolved {
		t.Fatalf("one matching referent: met=%v resolved=%v, want true true (run)", met, resolved)
	}
}

// TestPresentDefinedGateReadsPresentCompare pins the PresentCompare$ half:
// over the same one-referent creature group, EQ0 denies and EQ1 runs.
func TestPresentDefinedGateReadsPresentCompare(t *testing.T) {
	h, _, ctx := presentDefinedCtx(t, "Creature", false)
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
// before the fix it was read as one and the gate denied.
func TestPresentDefinedGateIgnoresThePhantomPresentKey(t *testing.T) {
	h, _, ctx := presentDefinedCtx(t, "Creature", false)
	gate := sa(t, "DB$ PutCounter | Defined$ TriggeredSourceLKICopy | CounterType$ P1P1 | PresentDefined$ TriggeredSourceLKICopy | Present$ Land | PresentCompare$ EQ1")
	if met, resolved := conditionMet(h, ctx, gate); !met || !resolved {
		t.Fatalf("phantom Present$ Land over a creature: met=%v resolved=%v, want true true (the phantom key is not read)", met, resolved)
	}
}

// TestPresentDefinedGateOverTheRealCorpusBodies drives the gate over the
// REAL SVar bodies of Experimental Lab // Staff Room's DBPutCounter and
// DBTurnFaceUp (the corpus's only two `DB$ ... PresentDefined$` lines),
// with their OWN filter specs:
//
//   - DBPutCounter: `IsPresent$ Card.canReceiveCounters P1P1` over the
//     TriggeredSourceLKICopy group -- a creature source counts 1 (run), a
//     land source counts 0 (deny).
//   - DBTurnFaceUp: `IsPresent$ Card.canBeTurnedFaceUp+faceDown` -- a
//     face-down source counts 1 (run), a face-up source counts 0 (deny).
//
// Each assertion asserts its precondition (the source's creature-ness /
// face-down state really differs between the two cases).
func TestPresentDefinedGateOverTheRealCorpusBodies(t *testing.T) {
	for _, tc := range []struct {
		svar     string
		runTypes string
		runDown  bool
		noTypes  string
		noDown   bool
	}{
		{"DBPutCounter", "Creature", false, "Land", false},
		{"DBTurnFaceUp", "Creature", true, "Creature", false},
	} {
		t.Run(tc.svar, func(t *testing.T) {
			body := corpusStaffRoomBody(t, tc.svar)
			gate := sa(t, body)
			if strings.TrimSpace(gate.Params["PresentDefined"]) != "TriggeredSourceLKICopy" {
				t.Fatalf("PresentDefined = %q, want TriggeredSourceLKICopy", gate.Params["PresentDefined"])
			}
			if strings.TrimSpace(gate.Params["IsPresent"]) == "" {
				t.Fatal("real body carries no IsPresent$ filter")
			}

			// Run case: the real filter matches the real group member.
			h, o, ctx := presentDefinedCtx(t, tc.runTypes, tc.runDown)
			if got := o.EffectiveIsCreature(); got != strings.Contains(tc.runTypes, "Creature") {
				t.Fatalf("precondition: run fixture creature-ness = %v", got)
			}
			if got := o.FaceDown && o.Zone == state.ZBattlefield; got != tc.runDown {
				t.Fatalf("precondition: run fixture face-down = %v, want %v", got, tc.runDown)
			}
			if met, resolved := conditionMet(h, ctx, gate); !met || !resolved {
				t.Fatalf("real filter over a matching source: met=%v resolved=%v, want true true (run)", met, resolved)
			}

			// Deny case: the same real filter, a source it excludes.
			h2, o2, ctx2 := presentDefinedCtx(t, tc.noTypes, tc.noDown)
			if got := o2.FaceDown && o2.Zone == state.ZBattlefield; got != tc.noDown {
				t.Fatalf("precondition: deny fixture face-down = %v, want %v", got, tc.noDown)
			}
			if met, resolved := conditionMet(h2, ctx2, gate); met || !resolved {
				t.Fatalf("real filter over an excluded source: met=%v resolved=%v, want false true (deny)", met, resolved)
			}
		})
	}
}

// TestPresentDefinedGateWithoutTriggerSourceIsUnresolved pins the absent
// group binding: with no TriggerSource there is no referent to count, so the
// gate is unresolved and the body runs unconditionally -- the convention
// every unenumerable defined group in conditionMet follows.
func TestPresentDefinedGateWithoutTriggerSourceIsUnresolved(t *testing.T) {
	h, o, _ := presentDefinedCtx(t, "Creature", false)
	gate := sa(t, "DB$ PutCounter | Defined$ TriggeredSourceLKICopy | CounterType$ P1P1 | PresentDefined$ TriggeredSourceLKICopy | IsPresent$ Creature")
	ctx := &Ctx{Controller: 0, Source: o.ID} // no TriggerContext.TriggerSource
	if _, resolved := conditionMet(h, ctx, gate); resolved {
		t.Fatal("gate resolved with no TriggerSource binding -- would count a phantom zero")
	}
}
