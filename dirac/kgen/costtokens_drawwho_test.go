package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDrawCostTargetedPlayerLowers: a Draw<N/Player.targetedBy> unless cost's
// drawing player is the parent's player target, so the built PayDraw carries
// it as W (gorge pay/unlessflow AdvanceUnless draws for the selected player;
// the kernel reads W in cost_restokens.go payDrawToken).
func TestDrawCostTargetedPlayerLowers(t *testing.T) {
	draw := "Name:Draw Who Probe\nManaCost:2 U\nTypes:Instant\n" +
		"A:SP$ Draw | Defined$ You | ValidTgts$ Player | NumCards$ 1 | SubAbility$ DBUnless\n" +
		"SVar:DBUnless:DB$ Draw | Defined$ You | NumCards$ 1 | UnlessCost$ Draw<1/Player.targetedBy> | UnlessPayer$ You | UnlessSwitched$ True\n"
	d, rs := resTokBuild(t, "Draw Who Probe", draw)
	if len(rs) != 0 {
		t.Fatalf("Draw<1/Player.targetedBy> refused: %v", rs)
	}
	in := resTokPay(t, d)
	if in.Op != kop.OpUnlessSwitched || in.C&0xF != kop.PayDraw || in.A.N != 1 {
		t.Fatalf("Draw<1/Player.targetedBy>: op %v C %#x A %d, want OpUnlessSwitched over PayDraw/1", in.Op, in.C, in.A.N)
	}
	if in.W != kop.RefTarget0 {
		t.Errorf("Draw<1/Player.targetedBy>: W %v, want the player target slot (RefTarget0)", in.W)
	}
}
