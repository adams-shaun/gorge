package cards

import "testing"

// TestSplitCountOp pins the operator-suffix grammar the replacement
// commutation classifiers read: a name outside the vocabulary, Twice with an
// argument, or an argument form without one all read as CountOpNone.
func TestSplitCountOp(t *testing.T) {
	for _, tc := range []struct {
		op   string
		code CountOp
		arg  string
	}{
		{"/Twice", CountOpTwice, ""},
		{"/Twice.x", CountOpNone, ""},
		{"/Plus.1", CountOpPlus, "1"},
		{"/Plus.", CountOpPlus, ""},
		{"/Plus", CountOpNone, ""},
		{"/Minus.N", CountOpMinus, "N"},
		{"/Minus", CountOpNone, ""},
		{"/LimitMax.Life", CountOpLimitMax, "Life"},
		{"/Times.2", CountOpNone, ""},
		{"", CountOpNone, ""},
		{"/", CountOpNone, ""},
		{"Plus.1", CountOpNone, ""},
	} {
		code, arg := SplitCountOp(tc.op)
		if code != tc.code || (code != CountOpNone && arg != tc.arg) {
			t.Errorf("SplitCountOp(%q) = (%d, %q), want (%d, %q)", tc.op, code, arg, tc.code, tc.arg)
		}
	}
}
