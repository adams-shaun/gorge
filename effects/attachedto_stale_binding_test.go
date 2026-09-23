package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// A recorded target or remembered ID is not a usable bearer if it no longer
// identifies an object in the game. In particular, negating a stale binding
// cannot turn it into a match against every other Aura.
func TestAttachedToStaleReferentFailsClosed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	live := corpusObject(t, reg, g, "Grizzly Bears")
	aura := corpusObject(t, reg, g, "Unholy Strength")
	stale := state.ObjID(999999)
	aura = g.Obj(aura.ID)
	aura.AttachedTo = stale
	if g.Obj(stale) != nil || g.Obj(live.ID) == nil || aura.Zone != state.ZBattlefield || aura.AttachedTo != stale || stale == live.ID {
		t.Fatal("precondition failed: Aura must be on the battlefield and point to a missing, not live, referent")
	}

	for _, ref := range []string{"Targeted", "ParentTarget", "TriggeredCardLKICopy", "TriggeredAttackerLKICopy"} {
		var sc SpecContext
		sc.Resolving = true
		if ref == "Targeted" || ref == "ParentTarget" {
			sc.ResolutionTargets = []state.Target{{Obj: stale}}
		} else {
			sc.Remembered = []state.Target{{Obj: stale}}
		}
		pred := "AttachedTo " + ref
		for _, token := range []string{pred, "!" + pred} {
			// The stale ID even agrees with the Aura's AttachedTo; the
			// positive path used to accept this as a valid match.
			if got, ok := matchPredicate(g, token, aura, sc); got || ok {
				t.Errorf("%s: stale binding returned (%v, %v), want (false, false)", token, got, ok)
			}
			if MatchesObjectCtx(g, "Aura."+token, aura, sc) {
				t.Errorf("Aura.%s: stale binding must match nothing", token)
			}
		}
		if un := UnknownPredicates("Aura." + pred); len(un) != 0 {
			t.Errorf("%s: grammar must still be recognized: %v", pred, un)
		}

		// A stale ID next to a live one must not silently narrow a plural
		// binding to the live object. Even if the Aura now points to that
		// live bearer, both positive and negated predicates stay unbound.
		scWithLive := sc
		if ref == "Targeted" || ref == "ParentTarget" {
			scWithLive.ResolutionTargets = []state.Target{{Obj: stale}, {Obj: live.ID}}
		} else {
			scWithLive.Remembered = []state.Target{{Obj: stale}, {Obj: live.ID}}
		}
		aura.AttachedTo = live.ID
		if g.Obj(aura.AttachedTo) == nil {
			t.Fatal("precondition failed: live bearer is missing")
		}
		for _, token := range []string{pred, "!" + pred} {
			if got, ok := matchPredicate(g, token, aura, scWithLive); got || ok {
				t.Errorf("%s: mixed stale/live binding returned (%v, %v), want (false, false)", token, got, ok)
			}
		}
		aura.AttachedTo = stale
	}
}
