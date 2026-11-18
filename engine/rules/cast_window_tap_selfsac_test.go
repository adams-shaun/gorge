package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// cwTapSacLand is Crystal Vein's printed shape: a free {T} mana ability and a
// second "{T}, Sacrifice this" ability making {C}{C}. The two are alternatives
// of one tap (CR 602.2b, 107.5): the sacrifice ability cannot be activated
// after the source has already tapped, so the most the land makes is 2.
const cwTapSacLand = "Name:Fixture Crystal Vein\nTypes:Land\n" +
	"A:AB$ Mana | Produced$ C | Cost$ T | SpellDescription$ Add one colourless mana.\n" +
	"A:AB$ Mana | Produced$ C | Amount$ 2 | Cost$ T Sac<1/CARDNAME> | SpellDescription$ Add two colourless mana.\n" +
	"Oracle:x\n"

// cwTapSacExtraArtifact carries a no-tap self-sacrifice extra beside both a
// free tap and a paid tap+sacrifice alternative. The extra may follow the free
// tap (1+1 = 2), never the "{1}, {T}, Sacrifice this" activation, which has
// already consumed the source (CR 602.2b).
const cwTapSacExtraArtifact = "Name:Fixture Double Sac\nTypes:Artifact\n" +
	"A:AB$ Mana | Produced$ C | Amount$ 3 | Cost$ 1 T Sac<1/CARDNAME> | SpellDescription$ Add three colourless mana.\n" +
	"A:AB$ Mana | Produced$ C | Cost$ T | SpellDescription$ Add one colourless mana.\n" +
	"A:AB$ Mana | Produced$ C | Cost$ Sac<1/CARDNAME> | SpellDescription$ Add one colourless mana.\n" +
	"Oracle:x\n"

// cwPlainLand funds the artifact's {1} fee without contributing a second
// activation of its own.
const cwPlainLand = "Name:Fixture Forest\nTypes:Basic Land Forest\n" +
	"A:AB$ Mana | Produced$ G | Cost$ T | SpellDescription$ Add one green mana.\nOracle:x\n"

// cwXSpell is a bare {X} sink: its announcement ceiling is exactly what the
// sources can pay, so an over-counted source shows up as a too-large X offer.
const cwXSpell = "Name:Fixture Endless\nManaCost:X\nTypes:Creature Eldrazi\nPT:0/0\nOracle:x\n"

// castWindowLargestX seats each source on the battlefield beside cwXSpell,
// casts the spell, and returns the largest X the announcement offers.
func castWindowLargestX(t *testing.T, seed uint64, sources ...string) int {
	t.Helper()
	e, _, _ := newFixtureDeck(t, seed, cwXSpell, sources...)
	for _, source := range sources {
		id := moveSeeded(t, e, 0, source, state.ZBattlefield)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: mana source %q must be an untapped battlefield permanent: %+v", source, o)
		}
	}
	toMain1(t, e)
	e.pending = nil
	e.priorityRound()
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("want the X ask, got %+v", d)
	}
	return d.Options[len(d.Options)-1].Amount
}

// TestCastWindowTapSelfSacIsOneAlternative: "{T}" and "{T}, Sacrifice this"
// are two alternatives of one tap, not a tap plus a second activation. The
// cast window must not price Crystal Vein's two abilities as 1+2.
func TestCastWindowTapSelfSacIsOneAlternative(t *testing.T) {
	t.Parallel()
	if got := castWindowLargestX(t, 915, cwTapSacLand); got != 2 {
		t.Fatalf("largest X offered = %d, want 2: {T} and {T}, Sacrifice this are one tap's alternatives (CR 602.2b, 107.5), not a tap plus a second activation", got)
	}
}

// TestCastWindowSacExtraDoesNotFollowSelfSacMain: the no-tap sacrifice extra
// may follow the free tap, but the walk must not add it after the paid
// tap+sacrifice main alternative, which already sacrificed the source. The
// plain land funds the artifact's {1} fee, so the buggy walk can take the paid
// activation and then the extra: 1 - 1 + 3 + 1 = 4, where the legal maximum is
// 1 (land) + 1 (free tap) + 1 (extra) = 3.
func TestCastWindowSacExtraDoesNotFollowSelfSacMain(t *testing.T) {
	t.Parallel()
	if got := castWindowLargestX(t, 916, cwTapSacExtraArtifact, cwPlainLand); got != 3 {
		t.Fatalf("largest X offered = %d, want 3: the sacrifice extra may follow the free tap, never the {1}, {T}, Sacrifice main alternative, which already consumed the source", got)
	}
}
