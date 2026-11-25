package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestDefinedPlayerQualifierUnknownInDefinedSpec pins the census boundary of
// definedSpec's base-Player fallback: a qualifier the shared player grammar
// does not read must stay UNKNOWN (ok=false), so knownDefinedTargets and the
// Defined$ census see the gap, while a modelled qualifier still resolves.
// Before this, the fallback swept the living seats and returned ok=true for
// every spec with a known base, even when an unread qualifier matched nobody;
// the support gate therefore never saw e.g. Thornbow Archer's
// `Player.Opponent+controlsElf_EQ0` (now modelled by the generic controls rule,
// so the synthetic unread qualifier below is the regression fixture).
func TestDefinedPlayerQualifierUnknownInDefinedSpec(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	// Precondition: seat 1 exists as the opponent and its life makes the
	// modelled qualifier below match a real seat.
	g.Players[1].Life = 13
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	if !MatchesPlayerSpecWithSVars(h, c, "Player.Opponent", 1, 0) ||
		MatchesPlayerSpecWithSVars(h, c, "Player.Opponent", 0, 0) {
		t.Fatal("precondition: the modelled opponent base must match seat 1 and not the controller")
	}

	for _, spec := range []string{
		"Player.Opponent+noSuchQualifier",
		"Opponent.noSuchQualifier",
		"You.noSuchQualifier",
		"Other.noSuchQualifier",
	} {
		ts, ok := definedSpec(h, c, spec)
		if ok {
			t.Errorf("definedSpec(%q) = %v, ok=true; an unread qualifier must stay unknown (ok=false)", spec, ts)
		}
		if len(ts) != 0 {
			t.Errorf("definedSpec(%q) = %v; an unknown qualifier must resolve to the empty set", spec, ts)
		}
	}

	// The public resolver keeps acting on nobody: it must not fall back to
	// the source just because definedSpec now reports the spelling unknown.
	c.Source = qualifierObject(t, g, 0, "Name:Probe Source\nTypes:Enchantment\nOracle:x\n").ID
	if ts := DefinedSpec(h, c, "Player.Opponent+noSuchQualifier"); len(ts) != 0 {
		t.Errorf("DefinedSpec(unread player qualifier) = %v; want the fail-closed empty set, never the source", ts)
	}

	// The modelled spellings keep the fallback's resolving verdict: the
	// opponent base and a state qualifier the grammar reads.
	if ts, ok := definedSpec(h, c, "Player.Opponent"); !ok || len(ts) != 1 || !ts[0].IsPlayer || ts[0].Player != 1 {
		t.Errorf("definedSpec(Player.Opponent) = %v, %v; want exactly opponent seat 1", ts, ok)
	}
	if ts, ok := definedSpec(h, c, "Player.lifeEQ13"); !ok || len(ts) != 1 || !ts[0].IsPlayer || ts[0].Player != 1 {
		t.Errorf("definedSpec(Player.lifeEQ13) = %v, %v; want exactly seat 1", ts, ok)
	}
}
