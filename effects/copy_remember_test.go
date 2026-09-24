package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestCopySpellAbilityRememberCopiesAppendsMintedCopy drives the REAL
// compiled Chef's Kiss copy SA (Defined$ Targeted | RememberCopies$ True |
// SubAbility$ DBChangeTargets) and pins Forge's append semantics: the copy
// actually minted is appended to the resolution's remembered set ALONGSIDE
// the entry the chain already carried. Chef's Kiss is the corpus carrier
// whose copy clause is followed by "reselect the targets at random for the
// spell AND THE COPY", so the remembered set it reads must include the copy.
func TestCopySpellAbilityRememberCopiesAppendsMintedCopy(t *testing.T) {
	saCK := corpusCopySA(t, "Chef's Kiss")
	if !strings.EqualFold(strings.TrimSpace(saCK.Params["RememberCopies"]), "True") {
		t.Fatalf("Chef's Kiss copy SA unexpectedly lacks RememberCopies$ True: %+v", saCK.Params)
	}
	if !strings.EqualFold(strings.TrimSpace(saCK.Params["Defined"]), "Targeted") {
		t.Fatalf("Chef's Kiss copy SA is not the Defined$ Targeted carrier: %+v", saCK.Params)
	}
	h := newHost(t, 2)
	// The spell Chef's Kiss copies -- it must sit ON THE STACK, or
	// effCopySpellAbility's zone guard returns before any copy is made.
	target := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
	// The pre-existing Remembered entry (Forge's ControlSpell-remembered
	// original), distinct from both the target and the source, so "append"
	// and "replace" are genuinely different observations.
	prior := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1", 0)
	if target.Zone != state.ZStack {
		t.Fatalf("precondition: copied spell zone=%s, want the stack", target.Zone)
	}
	if prior.ID == target.ID {
		t.Fatalf("precondition: prior remembered id %d equals the copied spell", prior.ID)
	}
	// Chef's Kiss's real chain ends in `DBCleanup | ClearRemembered$ True`,
	// which empties ctx.Remembered BY DESIGN at the end of the resolution
	// (effCleanup). The chain-local set the real DBChangeTargets sub reads is
	// observable at the copy clause's own exit, so resolve the REAL compiled
	// copy SA with its follow-up chain detached -- body untouched, the
	// cleanup's erasure not yet applied.
	ck := *saCK
	ck.Sub = nil
	ctx := &Ctx{Source: target.ID, Controller: 0,
		Targets:    []state.Target{{Obj: target.ID}},
		Remembered: []state.Target{{Obj: prior.ID}}}
	Resolve(h, ctx, &ck)

	if got := copyEvents(h); got != 1 {
		t.Fatalf("StackCopy events=%d, want exactly 1", got)
	}
	var copyID state.ObjID
	for _, ob := range h.g.Objs {
		if ob.IsCopy {
			if copyID != 0 {
				t.Fatalf("more than one copy object: %d and %d", copyID, ob.ID)
			}
			copyID = ob.ID
		}
	}
	if copyID == 0 || copyID == prior.ID || copyID == target.ID {
		t.Fatalf("precondition/minted: copy id=%d, prior=%d target=%d", copyID, prior.ID, target.ID)
	}
	foundPrior, foundCopy := false, false
	for _, entry := range ctx.Remembered {
		foundPrior = foundPrior || entry.Obj == prior.ID
		foundCopy = foundCopy || entry.Obj == copyID
	}
	if !foundPrior || !foundCopy || len(ctx.Remembered) != 2 {
		t.Fatalf("Remembered=%+v, want prior %d AND appended copy %d (append, not replace)",
			ctx.Remembered, prior.ID, copyID)
	}
	// The persistent half: eventRemember's Choose/remembered event records the
	// copy on the source object's event-backed list (Forge's card.addRemembered),
	// which the chain's ClearRemembered$ Cleanup cannot erase from the log.
	remembered := 0
	for _, ev := range h.log {
		if ev.Kind == events.Choose && ev.Counter == "remembered" && len(ev.IDs) == 1 && ev.IDs[0] == copyID {
			remembered++
		}
	}
	if remembered == 0 {
		t.Fatalf("no Choose/remembered event recorded the minted copy %d: %+v", copyID, h.log)
	}
}

// TestCopySpellAbilityRememberCopiesGateFlips is the ticket's purpose: the
// exact "if you didn't copy a spell this way" shape Shiko and Narset,
// Unified's DBDraw uses --
// ConditionDefined$ Remembered | ConditionPresent$ Spell | ConditionCompare$ EQ0.
// A hand-built chain keeps the trigger's fire-time capture (Ctx.Captured and
// the trigger-seeded Remembered) out of the picture, so the gate reads ONLY
// what the copy clause itself recorded. Resolved through the real chain, the
// two directions are a real Draw event or none.
func TestCopySpellAbilityRememberCopiesGateFlips(t *testing.T) {
	chain := chainSA(t,
		"SP$ CopySpellAbility | Defined$ Parent | RememberCopies$ True | SubAbility$ Gated",
		"Gated:DB$ Draw | ConditionDefined$ Remembered | ConditionPresent$ Spell | ConditionCompare$ EQ0")
	if !strings.EqualFold(strings.TrimSpace(chain.Params["RememberCopies"]), "True") {
		t.Fatalf("precondition: chain copy SA lacks RememberCopies$ True: %+v", chain.Params)
	}
	if chain.Sub == nil || !strings.EqualFold(strings.TrimSpace(chain.Sub.Params["ConditionCompare"]), "EQ0") {
		t.Fatalf("precondition: chained gate is not the EQ0 shape: %+v", chain.Sub)
	}

	// Direction 1 -- a copy IS made: the gate is false, no card is drawn.
	h := newHost(t, 2)
	fillLibrary(h.g, 0, mkCard(t, "Name:Top\nTypes:Instant\nOracle:x\n"), 1)
	source := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 0)
	if source.Zone != state.ZStack {
		t.Fatalf("precondition: copy source zone=%s, want the stack", source.Zone)
	}
	Resolve(h, &Ctx{Source: source.ID, Controller: 0}, chain)
	if got := copyEvents(h); got != 1 {
		t.Fatalf("direction 1: %d StackCopy events, want 1 (a copy was made)", got)
	}
	if got := drawEvents(h); got != 0 {
		t.Fatalf("direction 1: %d Draw events, want 0 (a copy WAS made, so \"if you didn't copy\" is false)", got)
	}

	// Direction 2 -- no copy: the copy source is not on the stack, so
	// effCopySpellAbility returns before any StackCopy and remembers nothing.
	// The gate is true and its Draw fires.
	h2 := newHost(t, 2)
	fillLibrary(h2.g, 0, mkCard(t, "Name:Top\nTypes:Instant\nOracle:x\n"), 1)
	source2 := h2.g.AddObject(mkCard(t, "Name:Left\nTypes:Sorcery\nOracle:x\n"), 0)
	if source2.Zone == state.ZStack {
		t.Fatalf("precondition: direction-2 source is on the stack zone=%s; the early return needs it off", source2.Zone)
	}
	Resolve(h2, &Ctx{Source: source2.ID, Controller: 0}, chain)
	if got := copyEvents(h2); got != 0 {
		t.Fatalf("direction 2: %d StackCopy events, want 0 (no copy)", got)
	}
	if got := drawEvents(h2); got != 1 {
		t.Fatalf("direction 2: %d Draw events, want 1 (NO copy was made, so \"if you didn't copy\" is true)", got)
	}
}

// TestCopySpellAbilityRememberCopiesCoversEveryEmitBranch pins that the
// RememberCopies$ rider records copies minted by BOTH emit sites (the
// DefinedTarget$ branch and the ordinary Amount$ branch) and that an absent
// or False parameter changes nothing. Copy objects are counted off the board,
// not the event log, so a branch that stopped minting would fail here too.
func TestCopySpellAbilityRememberCopiesCoversEveryEmitBranch(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		wantCopy int
		wantRem  bool
	}{
		{"defined target branch", "SP$ CopySpellAbility | Defined$ Parent | DefinedTarget$ Self | RememberCopies$ True", 1, true},
		{"amount branch", "SP$ CopySpellAbility | Defined$ Parent | Amount$ 2 | RememberCopies$ True", 2, true},
		{"absent parameter", "SP$ CopySpellAbility | Defined$ Parent", 1, false},
		{"false parameter", "SP$ CopySpellAbility | Defined$ Parent | RememberCopies$ False", 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(t, 2)
			source := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 0)
			if source.Zone != state.ZStack {
				t.Fatalf("precondition: source zone=%s, want the stack", source.Zone)
			}
			ctx := &Ctx{Source: source.ID, Controller: 0}
			Resolve(h, ctx, sa(t, tc.line))
			copies := 0
			for _, ob := range h.g.Objs {
				if ob.IsCopy {
					copies++
				}
			}
			if copies != tc.wantCopy {
				t.Fatalf("minted copies=%d, want %d", copies, tc.wantCopy)
			}
			if tc.wantRem {
				if len(ctx.Remembered) != tc.wantCopy {
					t.Fatalf("Remembered=%+v, want %d minted copies recorded", ctx.Remembered, tc.wantCopy)
				}
				return
			}
			if len(ctx.Remembered) != 0 {
				t.Fatalf("Remembered=%+v, want untouched for a non-True parameter", ctx.Remembered)
			}
		})
	}
}

// chainSA parses a primary ability line plus named SVar sub-abilities and
// returns the linked primary SA, so a test can drive a real Condition* gate
// chain without a hand-built cards.SA bag (which is exactly what let the
// inverted unswitched orientation look right in copy_test.go's B2 history).
func chainSA(t *testing.T, primary string, svars ...string) *cards.SA {
	t.Helper()
	var b strings.Builder
	b.WriteString("Name:T\nTypes:Sorcery\nA:" + primary + "\n")
	for _, s := range svars {
		b.WriteString("SVar:" + s + "\n")
	}
	b.WriteString("Oracle:x\n")
	c, d := cards.ParseBytes("t.txt", []byte(b.String()))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	return c.Faces[0].Abilities[0]
}

// drawEvents counts the Draw events the fakeHost logged: the observable the
// gate's resolved-met outcome produces (effDraw's lib[0] draw).
func drawEvents(h *fakeHost) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.Draw {
			n++
		}
	}
	return n
}
