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

// Heads whose triggering events this build does not raise (a scry event's
// ScryNum, the number looked at) stay zero -- the conservative same-as-before
// no-op, not a regression. ScryBottom is NO LONGER one of them: it is the
// completed-scry marker's bottom count (TestTriggerCountScryBottomReads...
// in temporal_anchor_test.go). Result is the RolledDie head
// (TestTriggerCountResultReadsTheDieRoll below).
func TestTriggerCountUnmodelledHeadsStayZero(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{}
	c.TriggerAmount = 6
	for _, expr := range []string{"TriggerCount$ScryNum"} {
		if got := EvalCount(h, c, expr); got != 0 {
			t.Errorf("%s = %d, want 0 (unmodelled head)", expr, got)
		}
	}
}

// TestTriggerCountResultReadsTheDieRoll pins the RolledDie head against the
// real corpus body Mr. House's DB$ Branch reads: BranchConditionSVar$
// TriggerCount$Result compared GE6. The die result is captured by rules into
// Ctx.TriggerResult when the RolledDie trigger fires (from the canonical
// roll Note), and survives to resolution through the per-stack-instance
// triggerContexts map -- not from TriggerAmount, and not re-inferred. The
// /Op suffix applies exactly as for the other heads.
func TestTriggerCountResultReadsTheDieRoll(t *testing.T) {
	h := newHost(t, 2)
	for _, tt := range []struct {
		expr string
		want int32
	}{
		{"TriggerCount$Result", 6},
		{"TriggerCount$Result/Plus.2", 8},
		{"TriggerCount$Result/Minus1", 5},
	} {
		c := &Ctx{}
		c.TriggerResult = 6
		if got := EvalCount(h, c, tt.expr); got != tt.want {
			t.Errorf("%s = %d, want %d", tt.expr, got, tt.want)
		}
	}
}

// A Result head on a resolution whose trigger was not a die roll must stay
// zero: TriggerResult is only ever set by the RolledDie capture, so a
// non-roll trigger's body reading it acts on nothing rather than inventing a
// result.
func TestTriggerCountResultEmptyWithoutARoll(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{}
	c.TriggerAmount = 5 // a damage/life trigger's magnitude, NOT a die result
	if got := EvalCount(h, c, "TriggerCount$Result"); got != 0 {
		t.Fatalf("TriggerCount$Result with no die roll = %d, want 0", got)
	}
}

// TestTriggerCountMaxResultReadsTheBatchHighRoll pins the second roll head,
// TriggerCountMax$Result, against the real corpus body Farideh, Devil's
// Chosen reads: SVar:DiceResult:TriggerCountMax$Result, gated
// ConditionSVarCompare$ GE10 ("if any of those results was 10 or higher").
// The batch's highest result is captured by rules into Ctx.TriggerResultMax
// when the RolledDieOnce trigger fires, so the head answers the MAX across a
// multi-die roll -- distinct from TriggerCount$Result, the batch's reported
// (last) result.
func TestTriggerCountMaxResultReadsTheBatchHighRoll(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{}
	c.TriggerResult = 3     // the last die
	c.TriggerResultMax = 11 // the highest die of the batch
	for _, tt := range []struct {
		expr string
		want int32
	}{
		{"TriggerCount$Result", 3},
		{"TriggerCountMax$Result", 11},
		{"TriggerCountMax$Result/Plus.2", 13},
	} {
		if got := EvalCount(h, c, tt.expr); got != tt.want {
			t.Errorf("%s = %d, want %d", tt.expr, got, tt.want)
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
