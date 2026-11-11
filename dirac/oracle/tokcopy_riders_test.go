package oracle

import (
	"strings"
	"testing"
)

// TestCopyInsteadTokenTakesTheCreationRiders: a "the first time you would
// create tokens each turn, create copies instead" replacement accepted on a
// creation that carries riders, tokens that enter tapped (pair 87) and tokens
// sacrificed at the next end step (pair 282). gorge lands every rider on each
// token the replaced creation mints, the copy included (applyTokenMintRiders),
// and the kernel mints the copy with them; it once stopped there.
func TestCopyInsteadTokenTakesTheCreationRiders(t *testing.T) {
	reg := synthReg(t)
	for _, name := range []string{"copy_instead_token_enters_tapped", "copy_instead_token_sacrificed_at_end"} {
		t.Run(name, func(t *testing.T) {
			res := replayStdReproResult(t, reg, "std-eoe", name, true) // recorded on the full surface
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("the kernel stopped: %q", res.Stall)
			}
		})
	}
}
