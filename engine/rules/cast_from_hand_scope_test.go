// The BARE wasCastFromYourHand predicate on a SpellCast trigger line that has
// no ValidActivatingPlayer$ (Baron Helmut Zemo: "Whenever you cast a black
// spell from your hand") scopes "your" to the trigger's controller, CR 109.5:
// an opponent's black spell cast from the opponent's hand does not trigger
// it, while its controller's own hand cast does. Forge (CardProperty
// wasCastFromYourHand) reads the same. Found by the cross-engine farm
// (MSH-s134-pool xe:RAND:1:15:w13 step 4).
//
// The carrier census below pins the class: every corpus script that spells
// the bare token on a SpellCast trigger carries ValidActivatingPlayer$ You
// except Zemo, so the scoping change moves exactly the ones the test lists.

package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const blackSorcery = "Name:Decoy Probe\nManaCost:B\nTypes:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"

// zemoCastStackDepth has `zemoOwner` control Baron Helmut Zemo, makes player 0
// cast a black spell from its hand, queues the triggers and returns the stack
// depth: 1 is the spell alone, 2 means Zemo's trigger is on it too.
func zemoCastStackDepth(t *testing.T, zemoOwner state.PlayerID) int {
	t.Helper()
	e := handEngine(t, card(t, blackSorcery))
	zemo := e.G.AddObject(corpusAlternativeCard(t, "Baron Helmut Zemo"), zemoOwner)
	zemo.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, zemoOwner, []state.ObjID{zemo.ID})
	spell := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MB] = 1
	castMode(t, e, spell, "")
	if len(e.pendingTriggers) > 0 {
		e.putTriggersOnStack()
	}
	return len(e.G.Stack)
}

func TestBareWasCastFromYourHandScopesToTriggerController(t *testing.T) {
	t.Parallel()
	if got := zemoCastStackDepth(t, 1); got != 1 {
		t.Fatalf("opponent's hand cast: stack depth %d, want 1 (Zemo's \"you\" is its controller)", got)
	}
	if got := zemoCastStackDepth(t, 0); got != 2 {
		t.Fatalf("controller's own hand cast: stack depth %d, want 2 (Zemo's trigger)", got)
	}
}

// TestBareWasCastFromYourHandCarrierCensus pins the corpus class: a SpellCast
// trigger line spelling the bare (or negated) token with no
// ValidActivatingPlayer$ is exactly the listed set. A new one fails here and
// must be reviewed against the controller scoping above (and dirac's lowering).
func TestBareWasCastFromYourHandCarrierCensus(t *testing.T) {
	t.Parallel()
	root := testutil.CorpusDirOrSkip(t)
	paths, _ := filepath.Glob(filepath.Join(root, "cardsfolder", "*", "*.txt"))
	var got []string
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, ln := range strings.Split(string(b), "\n") {
			if !strings.Contains(ln, "Mode$ SpellCast") || !strings.Contains(ln, "wasCastFromYourHand") ||
				strings.Contains(ln, "wasCastFromYourHandByYou") || strings.Contains(ln, "ValidActivatingPlayer$") {
				continue
			}
			got = append(got, filepath.Base(p))
			break
		}
	}
	want := []string{"baron_helmut_zemo.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unscoped bare wasCastFromYourHand SpellCast carriers = %v, want %v", got, want)
	}
}
