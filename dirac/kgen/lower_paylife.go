package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// lower_paylife.go: "as CARDNAME enters, pay any amount of life" (Phyrexian
// Processor, Minion of the Wastes, Nameless Race): a plain R:Event$ Moved
// ReplacementResult$ Updated line whose ReplaceWith$ body is the one node
//
//	AB$ StoreSVar | Cost$ Mandatory PayLife<X> | [XMax$ <cap>] | SVar$ <name>
//	  | Type$ Calculate | Expression$ X      (SVar:X:Count$xPaid)
//
// gorge asks the amount at the entry boundary (etbPayLifeBound / etbPayLifeOptions:
// a KChoose of "paylife" options 0..min(life, XMax$)), pays it, and the body
// stores it as the entering permanent's SVar number. The kernel lowers the
// replacement to the enters program ExtPayLifeX (B the cap value, or 32767
// without XMax$; C the stored name, as ExtStoreNum's), which asks, pays and
// stores in one step (dirac/repl_paylife.go). The name's readers (a token's
// TokenPower$, a CDA's SetPower$) are ExSVarNum reads through lc.value.

func init() { instrOp("ExtPayLifeX", (*instrCtx).opExtPayLifeX) }

func (ic *instrCtx) opExtPayLifeX() error {
	ki, in := &ic.ki, &ic.in
	idx, err := ic.cb.svarIndex(in.CS)
	if err != nil {
		return err
	}
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtPayLifeX))
	ki.C = kop.StoreNumC(int(idx), in.C&1 != 0, in.C>>1)
	return nil
}

// payLifeNoCap is ExtPayLifeX's B for a body without XMax$ (dirac payLifeNoCap).
const payLifeNoCap = 32767

// etbPayLife lowers the StoreSVar body of an as-enters pay-any-amount-of-life
// replacement. Only the exact shape lowers; every other StoreSVar body of an
// ETB replacement refuses by name.
func (lc *lowerCtx) etbPayLife(ps *params, node, sa *cards.SA, other bool, cond *lCond) error {
	if other || node != sa || sa.Sub != nil || cond.Kind != cNone || len(lc.ab.Prog) != 0 || !lc.replUpdated {
		return refuse("ETB replacement api StoreSVar: PayLife body beside another part or without a plain R: line")
	}
	if c := ps.str("Cost"); c != "Mandatory PayLife<X>" {
		return refuse("ETB replacement api StoreSVar: Cost$ %q is not the announced PayLife<X>", c)
	}
	if strings.TrimSpace(lc.face.SVars["X"]) != "Count$xPaid" {
		return refuse("ETB replacement api StoreSVar: PayLife<X> without SVar X = Count$xPaid")
	}
	name := ps.str("SVar")
	if ps.str("Type") != "Calculate" || ps.str("Expression") != "X" || name == "" {
		return refuse("ETB replacement api StoreSVar: PayLife body must store Expression$ X as Type$ Calculate")
	}
	if !lc.storesNum(name) || lc.numCondRead(name) {
		return refuse("ETB replacement api StoreSVar: PayLife SVar %s is read by a CheckSVar$ condition", name)
	}
	def, err := lc.printedDefault(name)
	if err != nil {
		return err
	}
	capV := cst(payLifeNoCap)
	if x := ps.str("XMax"); x != "" {
		if capV, err = lc.value(x); err != nil {
			return refuse("ETB replacement api StoreSVar XMax$ %s: %v", x, err)
		}
	}
	// the printed default deletes the record on a write of it, as ExtStoreNum's C
	c := 0
	if def.isConst() && kop.StoreNumDefFits(def.N) {
		c = def.N<<1 | 1
	}
	ps.consume("Cost", "XMax", "SVar", "Type", "Expression")
	lc.emit(lInstr{Op: "ExtPayLifeX", B: capV, C: c, CS: name, Note: "pay any amount of life as it enters"})
	return nil
}
