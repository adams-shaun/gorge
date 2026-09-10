package effects

// Task trigcount1: the TriggerCount$ heads (SVar bodies) were unimplemented
// and degraded to zero. These pin the head resolution in EvalCount against a
// Ctx whose TriggerContext.TriggerAmount carries the magnitude the triggering
// event dealt (captured by rules into the per-stack-instance triggerContexts
// map and read back at resolution). The head names are the real corpus ones:
// TriggerCount$DamageAmount (Kjeldoran Gargoyle), TriggerCount$LifeAmount
// (Vito, Thorn of the Dusk Rose) and TriggerCount$Amount (Chandra, Fire
// Artisan).

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

func TestTriggerCountHeadsReadTriggerAmount(t *testing.T) {
	h := newHost(t, 2)
	for _, tt := range []struct {
		expr string
		want int32
	}{
		{"TriggerCount$DamageAmount", 5},
		{"TriggerCount$LifeAmount", 5},
		{"TriggerCount$Amount", 5},
		{"TriggerCount$DamageAmount/Twice", 10},
		{"TriggerCount$Amount/Plus.2", 7},
		{"TriggerCount$Amount/Minus1", 4},
		{"TriggerCount$Amount/Times.3", 15},
		{"TriggerCount$Amount/HalfDown", 2},
	} {
		c := &Ctx{}
		c.TriggerAmount = 5
		if got := EvalCount(h, c, tt.expr); got != tt.want {
			t.Errorf("%s = %d, want %d", tt.expr, got, tt.want)
		}
	}
}

// An empty TriggerAmount must stay zero: a context with no triggering-event
// magnitude degrades to 0, never to a default, the same totality convention
// every other head here follows.
func TestTriggerCountHeadsEmptyTriggerAmountIsZero(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{}
	if got := EvalCount(h, c, "TriggerCount$DamageAmount"); got != 0 {
		t.Fatalf("TriggerCount$DamageAmount with no trigger amount = %d, want 0", got)
	}
}

// Heads whose triggering events this build does not raise (a die/dice roll's
// Result, a scry event's ScryNum/ScryBottom) stay zero -- the conservative
// same-as-before no-op, not a regression.
func TestTriggerCountUnmodelledHeadsStayZero(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{}
	c.TriggerAmount = 6
	for _, expr := range []string{"TriggerCount$Result", "TriggerCount$ScryNum", "TriggerCount$ScryBottom"} {
		if got := EvalCount(h, c, expr); got != 0 {
			t.Errorf("%s = %d, want 0 (unmodelled head)", expr, got)
		}
	}
}

// TestVitoTriggerCountLifeAmount pins the second head the brief names
// (LifeAmount) with the exact SVar body of a real corpus card: Vito, Thorn of
// the Dusk Rose declares `SVar:X:TriggerCount$LifeAmount` and its trigger
// loses the target opponent `that much` life (DB$ LoseLife | LifeAmount$ X).
// The value has to be the life the triggering event changed by, carried in
// TriggerAmount, and it must be N -- not the zero the unmodelled prefix used
// to produce.
func TestVitoTriggerCountLifeAmount(t *testing.T) {
	h := newHost(t, 2)
	for _, n := range []int32{2, 5} {
		c := &Ctx{}
		c.TriggerAmount = n
		if got := EvalCount(h, c, "TriggerCount$LifeAmount"); got != n {
			t.Errorf("TriggerCount$LifeAmount = %d, want %d (Vito's that-much)", got, n)
		}
	}
}

// TestChandraTriggerCountAmount pins the generic Amount head with a real
// corpus card's SVar: Chandra, Fire Artisan's trigger reads
// `SVar:X:TriggerCount$Amount` to size the damage it deals once loyalty
// counters are removed.
func TestChandraTriggerCountAmount(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{}
	c.TriggerAmount = 3
	if got := EvalCount(h, c, "TriggerCount$Amount"); got != 3 {
		t.Fatalf("TriggerCount$Amount = %d, want 3 (Chandra's that-much)", got)
	}
}

// TestNumSvarIndirectionResolvesTriggerCount pins the path a real card uses:
// the effect reads a named parameter (LifeAmount$ X), Num resolves X through
// the SVar table to a TriggerCount$ body, and EvalCount turns that into the
// event's amount. This mirrors the Sacrificed$ path's TestNumSvarIndirection.
func TestNumSvarIndirectionResolvesTriggerCount(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{}
	c.SVars = map[string]string{"X": "TriggerCount$DamageAmount"}
	c.TriggerAmount = 3
	sa := &cards.SA{Kind: "SP", API: "GainLife", Params: map[string]string{"LifeAmount": "X"}}
	if got := Num(h, c, sa, "LifeAmount", 1); got != 3 {
		t.Fatalf("Num resolved LifeAmount$ X = %d, want 3", got)
	}
}
