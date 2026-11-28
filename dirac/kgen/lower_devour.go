package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// kw:Devour (CR 702.82a; gorge engine/cards/kw_devour.go): "As this enters,
// you may sacrifice any number of <type>s. It enters with N times that many
// +1/+1 counters." gorge expands the keyword into Forge's CardFactoryUtil
// shape verbatim: one Moved ETB replacement whose ReplaceWith$ chain is
//
//	DB$ Sacrifice | Defined$ You | Amount$ <count of valid.YouCtrl+Other> |
//	    RememberSacrificed$ True | Optional$ True | SacValid$ <valid>.Other
//	DB$ PutCounter | ETB$ True | Defined$ Self | CounterType$ P1P1 |
//	    CounterNum$ <Count$RememberedSize/Times.N>
//	DB$ Cleanup | ClearRemembered$ True
//
// Only that exact chain lowers, as OpExt ExtEnterDevour (dirac/kw_devour.go):
// A = N, F = the sacrificed permanents' filter. A Devour X (Thromok the
// Insatiable: Times.X, n per devoured creature) stays refused, as does any
// other Sacrifice body in an ETB replacement.

// etbDevour lowers the Devour chain starting at node sa; it reports whether
// the chain is a Devour (an error is a refusal of one).
func (lc *lowerCtx) etbDevour(sa *cards.SA, other bool, cond *lCond) (bool, error) {
	if sa.API != "Sacrifice" {
		return false, nil
	}
	put := sa.Sub
	if put == nil || put.API != "PutCounter" || put.Sub == nil || put.Sub.API != "Cleanup" || put.Sub.Sub != nil {
		return false, nil
	}
	if other || cond.Kind != cNone || len(lc.ab.Prog) != 0 {
		return true, refuse("ETB replacement api Sacrifice beside another ETB replacement part")
	}
	ps := newParams(sa.Params)
	pp := newParams(put.Params)
	cp := newParams(put.Sub.Params)
	valid := strings.TrimSpace(ps.str("SacValid"))
	base, ok := strings.CutSuffix(valid, ".Other")
	if !ok || base == "" ||
		!ps.is("Defined", "You") || !ps.is("RememberSacrificed", "True") || !ps.is("Optional", "True") ||
		!pp.is("ETB", "True") || !pp.is("CounterType", "P1P1") || !cp.is("ClearRemembered", "True") {
		return true, refuse("ETB replacement api Sacrifice")
	}
	if d, has := pp.get("Defined"); has && strings.TrimSpace(d) != "Self" {
		return true, refuse("ETB replacement api Sacrifice")
	}
	// the Amount$ is the count of the valid permanents the controller has
	if ps.str("Amount") == "" || strings.TrimSpace(lc.face.SVars[ps.str("Amount")]) != "Count$Valid "+base+".YouCtrl+Other" {
		return true, refuse("Devour Amount$ %q", ps.str("Amount"))
	}
	// the counters are N per sacrificed permanent: Count$RememberedSize/Times.N
	body := strings.TrimSpace(lc.face.SVars[pp.str("CounterNum")])
	times, ok := strings.CutPrefix(body, "Count$RememberedSize/Times.")
	n, err := strconv.Atoi(times)
	if !ok || err != nil || n < 1 || n > 15 {
		return true, refuse("Devour counters %q", body)
	}
	f, err := lc.filter(valid)
	if err != nil {
		return true, err
	}
	ps.consume("SubAbility", "Defined", "Amount", "RememberSacrificed", "Optional", "SacValid")
	pp.consume("SubAbility", "ETB", "Defined", "CounterType", "CounterNum")
	cp.consume("ClearRemembered")
	for _, p := range []*params{ps, pp, cp} {
		if left := p.unread(); len(left) > 0 {
			return true, refuse("Devour unread params %v", left)
		}
	}
	lc.emit(lInstr{Op: "EnterDevour", Obj: lRef{Kind: rReplaced}, A: cst(n), F: f, Note: "devour: sacrifice any number as it enters"})
	return true, nil
}

// buildEnterDevour is OpExt ExtEnterDevour: A the counters per sacrificed
// permanent, the filter the sacrificed permanents'.
func buildEnterDevour(ki *kop.Instr, n int, f kop.FilterID) {
	ki.Op, ki.A, ki.B, ki.F = kop.OpExt, kop.Const(int(kop.ExtEnterDevour)), kop.Const(n), f
}
