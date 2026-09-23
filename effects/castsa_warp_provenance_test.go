package effects

// The effects-side CastSa flag-token strip (task mayplay-warp): the
// ConditionPresent$ Card.CastSa Spell.Warp gate on Full Bore
// (`.cards/cardsfolder/f/full_bore.txt`) and the sibling Spell.Mayhem /
// Spell.MayPlaySource gates all route through castSaAdmitsFilter. The
// brief's premise that Spell.Mayhem fails closed was measured false (it
// landed with kw:Mayhem); this file pins Spell.Warp's newly added arm and
// the shared loop that keeps the next flag spelling from being handled at
// one read and missed at the other.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestCastSaFlagFilterStripsWarp pins the positive/negative split for
// Spell.Warp against the resolving object's pay-time CastFlags.
func TestCastSaFlagFilterStripsWarp(t *testing.T) {
	h, c := fixtureHost(t)
	o := h.g.Obj(c.Source)
	o.Zone = state.ZStack
	// Precondition: the two bits under test differ, so the negative case
	// below proves something.
	if state.FlagWarped == state.FlagMayhem {
		t.Fatal("precondition: FlagWarped and FlagMayhem must be distinct bits")
	}

	const spec = "Card.CastSa Spell.Warp"

	// Precondition: with no provenance the token must NOT be stripped out
	// from under the positive gate — the strip leaves the spec untouched and
	// the gate reports the requirement unmet.
	if out, ok := castSaAdmitsFilter(h, spec, c.Source); ok {
		t.Fatalf("no-flag cast: strip admitted %q, want the positive gate to fail", out)
	}

	o.CastFlags = state.FlagWarped
	if o.CastFlags&state.FlagWarped == 0 {
		t.Fatal("precondition: FlagWarped did not land on the object")
	}
	out, ok := castSaAdmitsFilter(h, spec, c.Source)
	if !ok || out == "" || strings.Contains(out, "Spell.Warp") {
		t.Fatalf("warp cast: strip = %q ok=%v, want the token removed and a live remainder", out, ok)
	}
	if strings.Contains(out, "CastSa") {
		t.Fatalf("warp cast: strip left a CastSa token in %q", out)
	}

	// The negated spelling is the mirror: dropped for a warp cast, admitted
	// for a plain one.
	negSpec := "Card.!" + spec[len("Card."):]
	if _, ok := castSaAdmitsFilter(h, negSpec, c.Source); ok {
		t.Fatal("warp cast: !CastSa Spell.Warp gate was admitted, want it dropped")
	}
	o.CastFlags = 0
	if _, ok := castSaAdmitsFilter(h, negSpec, c.Source); !ok {
		t.Fatal("plain cast: !CastSa Spell.Warp gate was dropped, want it admitted")
	}
}

// TestCastSaFlagFilterHandlesBothTokensInOneSpec pins the structural fix:
// one spec carrying Warp AND Mayhem is evaluated against the SAME
// CastFlags read, so the loop cannot handle the first token and miss the
// second.
func TestCastSaFlagFilterHandlesBothTokensInOneSpec(t *testing.T) {
	h, c := fixtureHost(t)
	o := h.g.Obj(c.Source)
	o.Zone = state.ZStack

	const spec = "Card.CastSa Spell.Warp+CastSa Spell.Mayhem"
	// With only Warp set, the Mayhem requirement is unmet: the whole spec is
	// dropped (not admitted), proving the second token was read.
	o.CastFlags = state.FlagWarped
	if out, ok := castSaAdmitsFilter(h, spec, c.Source); ok {
		t.Fatalf("warp-only cast: mixed spec admitted as %q, want dropped on the Mayhem leg", out)
	}
	// With only Mayhem set, the Warp requirement is unmet instead.
	o.CastFlags = state.FlagMayhem
	if out, ok := castSaAdmitsFilter(h, spec, c.Source); ok {
		t.Fatalf("mayhem-only cast: mixed spec admitted as %q, want dropped on the Warp leg", out)
	}
	// With both set the spec survives with both tokens stripped.
	o.CastFlags = state.FlagWarped | state.FlagMayhem
	out, ok := castSaAdmitsFilter(h, spec, c.Source)
	if !ok || strings.Contains(out, "CastSa") {
		t.Fatalf("both-set cast: strip = %q ok=%v, want both tokens removed", out, ok)
	}
}

// TestFullBoreConditionPresentGateReadsWarp drives the real corpus gate
// shape end to end through conditionMet: the resolving creature's warp cast
// makes `ConditionPresent$ Card.CastSa Spell.Warp` met, a plain cast leaves
// it unmet — and both resolve (not the unsupported fail-open path), which is
// the assertion that the handler actually ran.
func TestFullBoreConditionPresentGateReadsWarp(t *testing.T) {
	h, c := fixtureHost(t)
	o := h.g.Obj(c.Source)
	o.Zone = state.ZStack

	gate := sa(t, "DB$ Pump | Defined$ Self | ConditionDefined$ Self | "+
		"ConditionPresent$ Card.CastSa Spell.Warp | ConditionCompare$ EQ1")

	o.CastFlags = state.FlagWarped
	if met, resolved := conditionMet(h, c, gate); !met || !resolved {
		t.Fatalf("warp cast vs the gate: met=%v resolved=%v, want true true", met, resolved)
	}
	o.CastFlags = 0
	if met, resolved := conditionMet(h, c, gate); met || !resolved {
		t.Fatalf("plain cast vs the gate: met=%v resolved=%v, want false true", met, resolved)
	}
}
