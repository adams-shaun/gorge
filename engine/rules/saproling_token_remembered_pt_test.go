package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// saprolingBurstSrc is the Saproling Burst shape (an inline fixture, not the
// corpus script): AB$ Token with RememberSource$ True mints a token whose
// characteristic-defining P/T is Remembered$CardCounters.FADE, the number of
// fade counters on the source it remembers.
const saprolingBurstSrc = "Name:Burst Fixture\nManaCost:4 G\nTypes:Enchantment\n" +
	"A:AB$ Token | Cost$ SubCounter<1/FADE> | TokenScript$ saproling_burst_fixture | TokenOwner$ You | ImprintTokens$ True | RememberSource$ True\n" +
	"Oracle:x\n"

const saprolingTokenSrc = "Name:Saproling Token\nManaCost:no cost\nColors:green\nTypes:Creature Saproling\nPT:*/*\n" +
	"S:Mode$ Continuous | CharacteristicDefining$ True | SetPower$ TokenX | SetToughness$ TokenX\n" +
	"SVar:TokenX:Remembered$CardCounters.FADE\nOracle:x\n"

// TestSaprolingTokenRememberedPT pins RememberSource$ True on a Token: the
// minted token remembers the creating source, and its remembered-counter P/T
// follows the fade counters on THAT object (and on no other).
func TestSaprolingTokenRememberedPT(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t)
	setFixtureToken(e, "saproling_burst_fixture", card(t, saprolingTokenSrc))
	burst := onBoard(t, e, 0, saprolingBurstSrc)
	other := onBoard(t, e, 0, "Name:Other Fader\nManaCost:1\nTypes:Enchantment\nOracle:x\n")
	e.G.Obj(burst).AddCounter("FADE", 4)
	e.G.Obj(other).AddCounter("FADE", 9)
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.Advance()

	activateFirst(t, e, burst)
	passUntilStackEmpty(t, e, 8)

	tok := atTokenWithName(t, e, 0, "Saproling Token")
	// Precondition: the cost removed one fade counter, so the read is 3 and
	// differs from the other source's 9 and from the pre-cost 4.
	if got := e.G.Obj(burst).Counter("FADE"); got != 3 {
		t.Fatalf("precondition: Burst fade counters = %d, want 3", got)
	}
	if d := e.Derived(tok); d.Power != 3 || d.Toughness != 3 {
		t.Fatalf("token P/T = %d/%d, want 3/3 (the fade counters on the remembered source)", d.Power, d.Toughness)
	}
	// The read is live: removing another fade counter shrinks the token.
	e.G.Obj(burst).AddCounter("FADE", -2)
	e.staticEpoch = -1
	e.activeEpoch = -1
	if d := e.Derived(tok); d.Power != 1 || d.Toughness != 1 {
		t.Fatalf("token P/T after the source lost counters = %d/%d, want 1/1", d.Power, d.Toughness)
	}
}
