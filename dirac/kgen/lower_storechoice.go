package kgen

import (
	"slices"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// lower_storechoice.go: "As CARDNAME enters, choose two abilities from among
// ..." (Rick, Steadfast Leader / Greymond, Avacyn's Stalwart): a
// K:ETBReplacement:Other whose body is a GenericChoice (no SetChosenMode$)
// over StoreSVar branches, each writing a literal flag that a CheckSVar$
// static of the card reads.
//
// gorge poses the choice as the permanent enters: effCharm asks the controller
// one KModes mode (Min 1, Max 1, one Kind "mode" option per Choices$ entry)
// and runs the chosen branch, whose StoreSVar writes Object.RuntimeSVars. The
// kernel mirrors the Siege mode election (OpChooseBranch C 2, a ChGenBranch
// ask posed with the permanent on the battlefield), generalised to the branch
// count in A: after the answer the enters program runs the chosen Branch
// block, whose ExtStoreNum writes the entering permanent's number. The
// statics' CheckSVar$ is an ordinary condition over the ExSVarNum read.
//
// The face's leaves-the-battlefield reset (a Static$ True ChangesZone trigger
// of Card.Self writing Number 0 over those names) lowers as the ordinary
// off-stack static trigger (Tf3Static, lower_statictrigger.go). It is not a
// no-op: gorge resolves it at the trigger drain, after the resolution that
// moved the card, so a blinked Rick re-enters, is asked again, and then has
// the answer's flags zeroed.

// isFlagChoice reports whether a GenericChoice's branches are all StoreSVar
// nodes: the shape etbStoreChoice owns. Any other GenericChoice keeps the Siege
// path's refusals.
func (lc *lowerCtx) isFlagChoice(node *cards.SA) bool {
	names := strings.Split(node.Params["Choices"], ",")
	for _, n := range names {
		if b := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(n)); b == nil || b.API != "StoreSVar" {
			return false
		}
	}
	return len(names) > 0 && names[0] != ""
}

// firstOtherKey is the alphabetically first key of params outside allowed ("" when none),
// so a refusal names the same key on every run.
func firstOtherKey(params map[string]string, allowed ...string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !slices.Contains(allowed, k) {
			return k
		}
	}
	return ""
}

// etbStoreChoice lowers the whole body of an ETB replacement that is a
// GenericChoice of StoreSVar flag branches.
func (lc *lowerCtx) etbStoreChoice(node, sa *cards.SA, other bool, cond *lCond) error {
	if other || node != sa || sa.Sub != nil || cond.Kind != cNone || len(lc.ab.Prog) != 0 {
		return refuse("GenericChoice of flags beside another ETB replacement part")
	}
	if k := firstOtherKey(node.Params, "Choices", "SpellDescription"); k != "" {
		return refuse("ETB GenericChoice of flags with %s$", k)
	}
	var names []string
	for _, n := range strings.Split(node.Params["Choices"], ",") {
		names = append(names, strings.TrimSpace(n))
	}
	if len(names) < 2 || len(names) > 8 {
		return refuse("ETB GenericChoice of flags with %d choices", len(names))
	}
	type branch struct {
		name string
		v    lValue
		c    int
	}
	var bs []branch
	for _, n := range names {
		b := cards.ResolveSVar(lc.face.SVars, n)
		if b == nil || b.API != "StoreSVar" || b.Sub != nil {
			return refuse("ETB GenericChoice branch %q is not a bare StoreSVar", n)
		}
		if k := firstOtherKey(b.Params, "SVar", "Type", "Expression", "SpellDescription"); k != "" {
			return refuse("ETB GenericChoice branch %q with %s$", n, k)
		}
		name := strings.TrimSpace(b.Params["SVar"])
		if strings.TrimSpace(b.Params["Type"]) != "Number" || !lc.storesNum(name) || lc.numCondRead(name) {
			return refuse("ETB GenericChoice branch %q stores %q beyond a Number flag", n, name)
		}
		v, err := lc.value(strings.TrimSpace(b.Params["Expression"]))
		if err != nil || !v.isConst() {
			return refuse("ETB GenericChoice branch %q Expression$ %q is not a literal", n, b.Params["Expression"])
		}
		def, err := lc.printedDefault(name)
		if err != nil {
			return err
		}
		bs = append(bs, branch{name, v, storeNumDefC(def)})
	}
	lc.emit(lInstr{Op: "GenericChoice", Obj: lRef{Kind: rYou}, A: cst(len(bs)), C: 2})
	for i, b := range bs {
		at := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Branch", C: i})
		lc.emit(lInstr{Op: "ExtStoreNum", B: b.v, C: b.c, CS: b.name})
		lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	}
	return nil
}

// storeNumDefC is ExtStoreNum's C for a write: the printed default's delete
// flag when that default is a literal the field holds (kop.StoreNumC).
func storeNumDefC(def lValue) int {
	if def.isConst() && kop.StoreNumDefFits(def.N) {
		return def.N<<1 | 1
	}
	return 0
}
