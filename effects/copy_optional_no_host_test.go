package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The Optional$ True CopySpellAbility election's no-host stand-in (R-9): the
// double has no decision channel, so the deterministic pre-ask behaviour --
// THE COPY IS MADE -- applies, exactly as effCopySpellAbility's doc comment
// records. findings-r2 found the round-1 code ignoring Ask's outcome and
// silently declining instead; this test pins the documented contract.
//
// The same no-host ask on the unless PAY gate resolves to a deterministic
// DECLINE (poseUnlessAsk), so the Optional$+UnlessCost$ carriers keep their
// two-level fallback: gate declines (body runs), election no-hosts (copy
// runs) -- the second test pins that composition end to end on the double.
func TestCopySpellAbilityOptionalNoHostMakesTheCopy(t *testing.T) {
	h := newHost(t, 2)
	if h.askResult {
		t.Fatal("precondition: the fresh double must have no decision channel (Ask reports false)")
	}
	o := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 0)
	Resolve(h, &Ctx{Source: o.ID, Controller: 0},
		sa(t, "DB$ CopySpellAbility | Defined$ Parent | Optional$ True"))
	if h.askCount == 0 {
		t.Fatal("no may-copy election was even attempted: the Optional$ read did not run")
	}
	if got := copyEvents(h); got != 1 {
		t.Fatalf("%d StackCopy events on a no-host election, want 1 (the R-9 stand-in makes the copy)", got)
	}
	copies := 0
	for _, ob := range h.g.Objs {
		if ob.IsCopy && ob.Zone == state.ZStack {
			copies++
		}
	}
	if copies != 1 {
		t.Fatalf("%d copy objects on the stack, want 1", copies)
	}
	if o.Zone != state.ZStack {
		t.Fatalf("source left the stack on the no-host path: %s", o.Zone)
	}
}

// The combined Optional$ + UnlessCost$ carrier on a no-host double: the
// unless gate no-hosts into its deterministic decline (with its Note), which
// RUNS the body on the unswitched shape, and the body's election no-hosts
// into the copy stand-in. Two no-hosts in sequence, one copy.
func TestCopySpellAbilityOptionalPlusUnlessNoHostStillCopies(t *testing.T) {
	h := newHost(t, 2)
	o := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 0)
	Resolve(h, &Ctx{Source: o.ID, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}},
		sa(t, "DB$ CopySpellAbility | Defined$ Parent | Controller$ TargetedController | UnlessPayer$ TargetedController | UnlessCost$ 2 | Optional$ True"))
	if h.askCount == 0 {
		t.Fatal("no ask was attempted: neither the unless gate nor the election ran")
	}
	declined := false
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "declined") {
			declined = true
		}
	}
	if !declined {
		t.Fatal("the unless gate's no-host deterministic decline recorded no Note")
	}
	if got := copyEvents(h); got != 1 {
		t.Fatalf("%d StackCopy events, want 1 (gate decline runs the body, election no-host makes the copy)", got)
	}
}
