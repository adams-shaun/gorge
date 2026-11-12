package cards

import (
	"strings"
	"testing"
)

// TestScavengeExpandsToAGraveyardPutCounter pins the CR 702.97a expansion:
// one graveyard activation whose cost exiles the card itself and whose
// PutCounter body sizes the +1/+1 counters by the exiled card's power.
func TestScavengeExpandsToAGraveyardPutCounter(t *testing.T) {
	f := &Face{Keywords: []string{"Scavenge:2 G"}}
	f.expandKeywords()
	if len(f.Abilities) != 1 {
		t.Fatalf("Scavenge expanded to %d abilities, want 1", len(f.Abilities))
	}
	sa := f.Abilities[0]
	if sa.API != "PutCounter" {
		t.Fatalf("API = %q, want PutCounter", sa.API)
	}
	cost := sa.ParamStr(PKCost)
	if !strings.Contains(cost, "2 G") || !strings.Contains(cost, "ExileFromGrave<1/CARDNAME>") {
		t.Fatalf("Cost$ = %q, want the printed 2 G plus ExileFromGrave<1/CARDNAME>", cost)
	}
	for _, c := range []struct {
		key  ParamKey
		want string
	}{
		{PKActivationZone, "Graveyard"},
		{PKCounterType, "P1P1"},
		{PKCounterNum, "__kwScavengeX"},
		{PKSorcerySpeed, "True"},
		{PKValidTgts, "Creature"},
	} {
		if got := sa.ParamStr(c.key); got != c.want {
			t.Errorf("param %d = %q, want %q", c.key, got, c.want)
		}
	}
	if got := f.SVars["__kwScavengeX"]; got != "Exiled$CardPower" {
		t.Fatalf("SVar __kwScavengeX = %q, want Exiled$CardPower", got)
	}
	// Idempotent: a second expansion must not mint a second ability.
	f.expandKeywords()
	if len(f.Abilities) != 1 {
		t.Fatalf("re-expansion minted %d abilities, want 1", len(f.Abilities))
	}
}
