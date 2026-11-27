package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// dealDamageValidSource lowers a DealDamage whose DamageSource$ is a Valid
// <filter> set and whose Defined$ names a referent relative to each source:
//
//	NumDmg$ <n> | DamageSource$ Valid <filter> | Defined$ CardController
//	NumDmg$ <n> | DamageSource$ Valid <filter> | Defined$ Enchanted
//
// (Aura Barbs' two halves: each enchantment deals n to its controller, then
// each Aura attached to a creature deals n to that creature). gorge walks the
// source set and deals once per member with that member as the damage source;
// the kernel's OpRepeatEach binds RefEach as the source and OpDealDamageFrom's
// B mode names the relative recipient (kop.DmgFromEachCtrl /
// kop.DmgFromEachAttached). done is false for any other shape, which stays on
// the ordinary source path. done is true whenever err is non-nil.
func (lc *lowerCtx) dealDamageValidSource(ps *params, n lValue) (bool, error) {
	src := strings.TrimSpace(ps.peek("DamageSource"))
	if !strings.HasPrefix(src, "Valid ") {
		return false, nil
	}
	if own := ps.peek("ValidTgts") != ""; own {
		return false, nil
	}
	if !ps.is("RelativeTarget", "True") {
		return true, refuse("DealDamage DamageSource$ %q without RelativeTarget$ True", src)
	}
	var cs string
	switch d := strings.TrimSpace(ps.str("Defined")); d {
	case "CardController":
		cs = "eachctrl"
	case "Enchanted":
		cs = "eachattached"
	default:
		return true, refuse("DealDamage DamageSource$ %q with Defined$ %q", src, d)
	}
	f, err := lc.filter(strings.TrimSpace(strings.TrimPrefix(src, "Valid ")))
	if err != nil {
		return true, err
	}
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return true, refuse("DealDamage DamageSource$ %q: not a battlefield object set", src)
	}
	ps.get("DamageSource")
	ps.consume("Defined", "RelativeTarget")
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "RepeatEachCard", F: f, C: kop.RepeatPlain, Skip: 1})
	lc.emit(lInstr{Op: "DealDamageFrom", Obj: lRef{Kind: rEach}, A: n, CS: cs, Note: "each source deals A"})
	lc.ab.Prog[at].Skip = 1
	return true, nil
}
