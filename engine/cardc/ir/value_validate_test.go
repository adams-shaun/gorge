package ir

import (
	"strings"
	"testing"
)

func TestValidateValue(t *testing.T) {
	one := func() *Value { return &Value{Kind: VConst, N: 1} }
	creature := func() *Filter { return &Filter{Op: FPred, Pred: PCardType, Word: "Creature"} }
	cmp := func() *Cond { return &Cond{Kind: CCompare, L: &Value{Kind: VX}, R: one(), Cmp: CmpGE} }
	good := map[string]*Value{
		"const":   one(),
		"x":       {Kind: VX},
		"add":     {Kind: VAdd, Args: []*Value{one(), {Kind: VX}}},
		"half-up": {Kind: VHalfUp, Args: []*Value{{Kind: VX}}},
		"min":     {Kind: VMin, Args: []*Value{one(), one()}},
		"count":   {Kind: VCount, Filter: creature()},
		"if":      {Kind: VIf, Test: cmp(), Args: []*Value{one(), {Kind: VConst}}},
		"entered": {Kind: VEnteredThisTurn, Filter: creature(), Zones: MakeZoneSet(ZoneBattlefield)},
		"power":   {Kind: VPower, Ref: &Ref{Kind: RefSelf}},
		"domain":  {Kind: VDomain},
	}
	for name, v := range good {
		if err := ValidateValue(v); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad := []struct {
		name, want string
		v          *Value
	}{
		{"nil", "nil value", nil},
		{"no kind", "no kind", &Value{}},
		{"out of range", "no kind", &Value{Kind: valueKindCount}},
		{"add one arg", "needs 2 args", &Value{Kind: VAdd, Args: []*Value{one()}}},
		{"const with arg", "needs 0 args", &Value{Kind: VConst, Args: []*Value{one()}}},
		{"half no arg", "needs 1 args", &Value{Kind: VHalfDown}},
		{"count no filter", "needs a filter", &Value{Kind: VCount}},
		{"if no test", "test goes with if", &Value{Kind: VIf, Args: []*Value{one(), one()}}},
		{"test off if", "test goes with if", &Value{Kind: VConst, Test: cmp()}},
		{"zones off entered", "no origin zones", &Value{Kind: VCount, Filter: creature(), Zones: MakeZoneSet(ZoneHand)}},
		{"bad arg", "no kind", &Value{Kind: VSub, Args: []*Value{one(), {}}}},
		{"bad filter", "card type", &Value{Kind: VCount, Filter: &Filter{Op: FPred, Pred: PCardType, Word: "Elf"}}},
		{"bad ref filter", "no op", &Value{Kind: VPower, Ref: &Ref{Kind: RefAll, Filter: &Filter{}}}},
		{"compare no cmp", "needs l, r and cmp", &Value{Kind: VIf, Test: &Cond{Kind: CCompare, L: one(), R: one()}, Args: []*Value{one(), one()}}},
		{"bad test value", "no kind", &Value{Kind: VIf, Test: &Cond{Kind: CCompare, L: &Value{}, R: one(), Cmp: CmpEQ}, Args: []*Value{one(), one()}}},
	}
	for _, c := range bad {
		err := ValidateValue(c.v)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), c.want) {
			t.Errorf("%s: error = %v, want one containing %q", c.name, err, c.want)
		}
	}
}
