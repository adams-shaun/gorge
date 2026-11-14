package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// numTimesPay lowers a printed trigger's own "you may pay {unit} up to N
// times. When you pay this cost one or more times, choose up to that many"
// (Tranquil Frillback): AB$ ImmediateTrigger, Cost$ Mana<UNIT\NumTimes>,
// Announce$ NumTimes, AnnounceMax$ N, ConditionCheckSVar$ NumTimes,
// RememberSVarAmount$ NumTimes, Execute$ a Charm with MinCharmNum$ 0 and
// CharmNum$ Count$TriggerRememberAmount (gorge cumulative.go
// triggeredCostNumTimesAsk, cr-numtimes-announce 5dc9b0697).
//
// The body is Ext NumTimes (the announcement 0..N, only the counts the
// payer's potential mana covers; 0 declines and skips to the end), the
// ordinary IfPaid on the unit cost (execPay scales it by the announced
// count), then one guarded ReflexiveTrigger per count k = 1..N into a
// latent that is the charm with CharmNum$ k (the kernel's multi-pick charm
// reads a constant maximum, so the announced count selects the latent).
func (lc *lowerCtx) numTimesPay(sa *cards.SA, ps *params, cost string) error {
	if lc.ab.Kind != akTriggered || len(lc.ab.Prog) != 0 || sa.API != "ImmediateTrigger" {
		return refuse("Mana<...\\NumTimes> outside a trigger's own ImmediateTrigger cost")
	}
	if ps.str("Announce") != "NumTimes" || ps.str("ConditionCheckSVar") != "NumTimes" || ps.str("RememberSVarAmount") != "NumTimes" {
		return refuse("NumTimes announcement shape")
	}
	// AnnounceMax$ absent: gorge offers every count the potential mana
	// covers (up to 64); the kernel holds 7 and stalls loud past it
	max, open := 7, true
	if v, ok := ps.get("AnnounceMax"); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 7 {
			return refuse("NumTimes AnnounceMax$ %q", v)
		}
		max, open = n, false
	}
	exec := cards.ResolveSVar(lc.face.SVars, ps.str("Execute"))
	if exec == nil {
		return refuse("NumTimes Execute$ %q", ps.str("Execute"))
	}
	charm := exec.API == "Charm"
	if charm {
		if open || exec.Sub != nil || exec.Params["CharmNum"] != "Count$TriggerRememberAmount" || exec.Params["MinCharmNum"] != "0" {
			return refuse("NumTimes Execute$ is not an up-to-N charm")
		}
		if strings.Count(exec.Params["Choices"], ",")+1 < max {
			return refuse("AnnounceMax$ above the charm's modes")
		}
	}
	ps.consume("Cost", "Announce", "AnnounceMax", "ConditionCheckSVar", "RememberSVarAmount", "Execute", "TriggerDescription")
	unit := strings.TrimSuffix(strings.TrimPrefix(cost, "Mana<"), `\NumTimes>`)
	c, err := lc.costUnless(unit)
	if err != nil {
		return err
	}
	lc.trigOwnCost = true
	pay, err := lc.payInstr(c, lRef{Kind: rYou}, true)
	lc.trigOwnCost = false
	if err != nil {
		return err
	}
	if pay.CS != "Mana" || pay.C != 0 || len(pay.Hyb) != 0 || pay.A.Kind != vConst || pay.B.Kind != vConst || pay.A.N < 0 || pay.A.N > 15 {
		return refuse("NumTimes unit cost shape")
	}
	for i := 0; i < 5; i++ {
		if pay.B.N>>(3*i)&7*max > 7 {
			return refuse("NumTimes pips past 7 over the maximum")
		}
	}
	if pay.A.N*max > 255 {
		return refuse("NumTimes generic past 255")
	}
	// the announcement: A the unit's generic, B its packed pips, C the
	// maximum; Skip (patched to the chain end) is its decline jump; CS open
	// stalls loud when an eighth repetition is payable
	ann := lInstr{Op: "NumTimes", A: pay.A, B: pay.B, C: max, Skip: -1, Note: "announce 0..max; 0 declines to the chain end"}
	if open {
		ann.CS = "open"
	}
	lc.emit(ann)
	pay.Op = "IfPaid"
	payAt := len(lc.ab.Prog)
	lc.emit(pay)
	lc.ab.Prog[payAt].Skip = -1
	lc.ab.Prog[payAt].Note = "covers the Sub chain"
	// one guarded ReflexiveTrigger per announced count k, into a latent that
	// is the Execute$ body lowered with the count a constant: the charm with
	// CharmNum$ k (the kernel's multi-pick charm reads a constant maximum),
	// or any body whose SVar X = Count$TriggerRememberAmount reads k
	for k := 1; k <= max; k++ {
		lat := lc.newLatent("ReflexiveTrigger")
		body := exec
		if charm {
			ek := *exec
			ek.Params = make(map[string]string, len(exec.Params))
			for key, v := range exec.Params {
				ek.Params[key] = v
			}
			ek.Params["CharmNum"] = strconv.Itoa(k)
			body = &ek
		}
		savedMulti, savedK := lc.reflexMulti, lc.numTimesK
		lc.reflexMulti, lc.numTimesK = charm, k
		err := lc.withAbility(lat, func() error { return lc.chain(body, nil) })
		lc.reflexMulti, lc.numTimesK = savedMulti, savedK
		if err != nil {
			return err
		}
		skipAt := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue, A: lValue{Kind: vX}, Op: opEQ, B: cst(k)}})
		lc.emit(lInstr{Op: "ReflexiveTrigger", A: cst(1), C: lat.Index})
		lc.ab.Prog[skipAt].Skip = 1
	}
	return nil
}
