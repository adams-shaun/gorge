package kgen

import (
	"fmt"
	"strings"
)

// (Shanna, Purifying Blade) a printed trigger's own "you may pay
// {X}". gorge's triggered-cost window (rules cumulative.go
// triggeredCostXAsk, the SVar:X = Count$xPaid payer-chooses arm) announces X
// before the pay election, bounded by the payer's mana and the body's XMax$;
// the paid X is the body's Count$xPaid. The kernel's PayManaX (kernel
// trig_maypayx.go) does the same, its A the XMax$ cap,
// or 64 when the body names none (Tester of the Tangential,
// Wildborn Preserver; gorge then bounds X by the payer's mana alone). Only
// exactly {X} as the whole cost, with the SVar:X Count$xPaid, lowers.

// trigXFoldCap is gorge's xFoldBoundCap (rules cumulative.go L979): the
// mana bound's cap, the whole cap when the body has no XMax$.
const trigXFoldCap = 64

// trigXCost reads the XMax$ of the trigger-own {X} cost (trigXFoldCap when
// absent) into lc.trigXMax and marks the body's Count$xPaid as the announced X.
func (lc *lowerCtx) trigXCost(ps *params) error {
	if x, ok := lc.face.SVars["X"]; !ok || strings.TrimSpace(x) != "Count$xPaid" {
		return refuse("trigger {X} cost with SVar:X other than Count$xPaid")
	}
	raw, ok := ps.get("XMax")
	if !ok {
		v := cst(trigXFoldCap)
		lc.trigXMax = &v
		return nil
	}
	v, err := lc.value(raw)
	if err != nil {
		return refuse("trigger {X} cost XMax$ %s: %v", raw, err)
	}
	lc.trigXMax = &v
	return nil
}

// payManaX lowers exactly {X} (no other mana, life or cost part) to PayManaX.
func (lc *lowerCtx) payManaX(c lCost, in lInstr) (lInstr, error) {
	m := c.Mana
	if !lc.trigOwnCost || m.XX || m.Generic != 0 || m.Pips != [5]int{} || len(m.Hybrid) > 0 || len(m.Twobrid) > 0 ||
		len(m.Phyrexian) > 0 || m.C > 0 || c.PayLife > 0 || c.Energy > 0 || c.DiscardN > 0 || c.Sac != nil || c.Return != nil ||
		c.ExileG != nil || c.Forage != nil || c.DrawN > 0 || c.SubCtr > 0 || c.TapN > 0 || c.SacSelf || c.Tap {
		return in, refuse("trigger {X} cost beside other cost parts")
	}
	in.CS, in.A = "ManaX", *lc.trigXMax
	lc.trigXPaid = true
	return in, nil
}

// trigRemXPaid marks a reflexive latent whose body reads Count$xPaid under a
// parent's announced {X} (Tester of the Tangential, Wildborn
// Preserver, "you may pay {X}. When you do, ... X ..."). gorge
// effImmediateTrigger copies the parent Ctx, its X the paid X, into the
// instance; the kernel captures it the same way as a TriggerRemembered$
// amount: OpReflexiveTrigger's B is the parent's VX, ridden as the entry's
// trigAmt and read as vTriggerAmount (one trigAmt per latent).
const trigRemXPaid = "xPaid"

// reflexXPaid lowers Count$xPaid inside a reflexive latent under a lowered
// PayManaX cost; ok false elsewhere.
func (lc *lowerCtx) reflexXPaid() (lValue, bool, error) {
	ab := lc.ab
	if !lc.trigXPaid || ab == nil || !ab.Latent || ab.Name != "ReflexiveTrigger" {
		return lValue{}, false, nil
	}
	if ab.TrigRem != "" && ab.TrigRem != trigRemXPaid {
		return lValue{}, false, fmt.Errorf("Count$xPaid beside TriggerRemembered$ in a reflexive trigger: one captured amount")
	}
	if ab.ReflexObj {
		return lValue{}, false, fmt.Errorf("Count$xPaid in a reflexive trigger remembering its target: one captured amount")
	}
	ab.TrigRem = trigRemXPaid
	return lValue{Kind: vTriggerAmount}, true, nil
}
