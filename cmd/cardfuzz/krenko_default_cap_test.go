package main

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
)

// TestKrenkoBoardDefaultCapFiresMidResolution is the DEFAULT budget's storm
// proof: the shared drive loop (testkit/bench) and the host arm
// LoopGuard.MaxObjs = rules.DefaultMaxObjs on every config that set no
// explicit budget, and the value must sit far above every ordinary game's
// arena (measured high-water 554) while still truncating the
// documented Krenko, Mob Boss storm: a staged board 320 objects under the
// default cap whose very next Krenko resolution (X = the number of Goblins
// you control, so it mints far more than the remaining headroom) aborts
// mid-resolution at exactly cap+1, with the watcher's own reason and the
// minting event kind -- the same shape the explicit -max-objects proof
// (TestKrenkoBoardCapFiresMidResolution) pins, at the default budget.
func TestKrenkoBoardDefaultCapFiresMidResolution(t *testing.T) {
	// Precondition: the default arming the harness and host paths share
	// produces exactly this budget.
	if rules.WithDefaultObjectCap(nil).MaxObjs != rules.DefaultMaxObjs {
		t.Fatalf("default arming produced %d, want %d", rules.WithDefaultObjectCap(nil).MaxObjs, rules.DefaultMaxObjs)
	}
	const goblins = 9600 // arena after staging 80 + 9600 = 9680 < 10000
	maxObjs := rules.DefaultMaxObjs
	e, _, _, abilityIdx := krenkoFixture(t, goblins, maxObjs)
	// Preconditions: the staged board is what the assertion depends on.
	if got := goblinTokens(t, e, 0); got != goblins {
		t.Fatalf("staged %d Goblin tokens, board holds %d", goblins, got)
	}
	if arena := len(e.G.Objs); arena != 80+goblins || arena >= maxObjs {
		t.Fatalf("arena %d, want 80+%d, strictly under the default cap %d", arena, goblins, maxObjs)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority ask at the fixture park point: %+v", d)
	}
	// One Krenko activation aborts mid-resolution at the default cap, exactly
	// the way the explicit-budget storm test recovers it.
	var abort *rules.LivelockError
	func() {
		defer func() {
			if r := recover(); r != nil {
				l, ok := r.(*rules.LivelockError)
				if !ok {
					t.Fatalf("activation panicked with %T, want *rules.LivelockError", r)
				}
				abort = l
			}
		}()
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{abilityIdx}}); err != nil {
			t.Fatalf("submit Krenko activation: %v", err)
		}
		passUntilStackDrained(t, e, 20000)
	}()
	if abort == nil {
		t.Fatal("the default cap did not abort the doubling resolution")
	}
	if abort.Reason != rules.ObjectCapReason {
		t.Fatalf("abort reason = %q, want %q", abort.Reason, rules.ObjectCapReason)
	}
	if abort.Cap != maxObjs || abort.Count != maxObjs+1 {
		t.Fatalf("abort cap/count = %d/%d, want %d/%d", abort.Cap, abort.Count, maxObjs, maxObjs+1)
	}
	if !strings.Contains(abort.Error(), "kind token_create") {
		t.Fatalf("abort diagnostic %q lacks the aborting kind", abort.Error())
	}
}
