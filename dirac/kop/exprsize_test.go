package kop

import (
	"testing"
	"unsafe"
)

// TestExprSizeUnchanged holds Expr to its size from before ExprKind widened to
// 16 bits: Kind took the pad byte ahead of N, so the wider kind is free.
func TestExprSizeUnchanged(t *testing.T) {
	if n := unsafe.Sizeof(Expr{}); n != 30 {
		t.Errorf("sizeof(Expr) = %d, want 30", n)
	}
	if off := unsafe.Offsetof(Expr{}.N); off != 2 {
		t.Errorf("offsetof(Expr.N) = %d, want 2", off)
	}
}
