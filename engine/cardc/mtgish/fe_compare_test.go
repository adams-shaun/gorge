package mtgish

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func TestCompareLowering(t *testing.T) {
	ops := map[string]ir.CmpOp{
		"EqualTo": ir.CmpEQ, "NotEqualTo": ir.CmpNE, "LessThan": ir.CmpLT,
		"LessThanOrEqualTo": ir.CmpLE, "GreaterThan": ir.CmpGT, "GreaterThanOrEqualTo": ir.CmpGE,
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			in := valNode("Comparison", name, valNode("GameNumber", "Integer", 4.0))
			got, err := (&Ctx{}).Compare(in)
			if err != nil {
				t.Fatal(err)
			}
			want := &ir.Compare{Op: op, V: &ir.Value{Kind: ir.VConst, N: 4}}
			if !ir.Equal(got, want) {
				t.Errorf("got %s want %s", got.String(), want.String())
			}
		})
	}
	if Registered("Comparison.OneOf") {
		t.Error("Comparison.OneOf must stay unregistered")
	}
}
