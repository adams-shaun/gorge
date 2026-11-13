package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// echoGate reads gorge kwEcho's intervening if, Echo$ True (CR 702.30a;
// engine/rules/echo.go echoGateHolds): the source came under its
// controller's control since the beginning of their most recent upkeep. It
// becomes the trigger's only condition, CondEchoOwed, checked as the trigger
// fires only (CheckOnTriggerOnly; dirac/trig_echo.go). kwEcho's upkeep
// trigger (Phase$ Upkeep, ValidPlayer$ You) carries no other gate, so Echo$
// beside another condition, a fire-time gate, a gift or NoResolvingCheck$
// (otherGate) refuses. A trigger without Echo$ is left as it is.
func echoGate(tp *params, tr *lTrigger, ab *lAbility, conds *[]lCond, otherGate bool) error {
	v, ok := tp.get("Echo")
	if !ok {
		return nil
	}
	if v != "True" || tr.Event != "Phase" || tr.Step != "Upkeep" || tr.Whose != whoYou || len(*conds) != 0 || otherGate {
		return refuse("Echo$ %q beside another gate", v)
	}
	*conds = append(*conds, lCond{Kind: cEchoOwed})
	ab.Name = "CheckOnTriggerOnly"
	return nil
}

// echoUpkeep lowers the body of gorge's echo expansion (CR 702.30a;
// engine/cards/kw_echo.go: an upkeep trigger of ValidPlayer$ You carrying
// Echo$ True, body DB$ Echo | Cost$ <cost>; Cradle Guard, Mogg War Marshal,
// Avalanche Riders). gorge's startEcho asks the trigger's controller to pay
// the cost or sacrifice the source (echo_pay / echo_sac, over the same resPay
// machinery as a trigger's own cost) and asks nothing when the source is gone.
// The program is
//
//	Cumulative B=1 C=2    skips the rest when the source is gone (no age counter)
//	UnlessPaid You <kind> the pay-or-decline ask, skipping the sacrifice when paid
//	Sacrifice self        a decline
//
// The gate (Echo$ True) is the trigger's CondEchoOwed (echoGate above).
// A plain mana cost (generic and coloured pips) lowers as Mana; a default
// Discard<N/Card> or a Sac<N/Land> echo cost lowers as the trigger's own
// Discard/Sacrifice cost (payInstr). {X} (Volcano Hellion), a hybrid or
// Phyrexian pip (gorge announces those before the election), a {C} pip, a
// free echo and any other parseCumulativeAction kind stay refused by name.
func (lc *lowerCtx) echoUpkeep(ps *params) error {
	raw, ok := ps.get("Cost")
	if !ok {
		return refuse("Echo without Cost$")
	}
	if lc.ab.Kind != akTriggered {
		return refuse("echo outside a trigger")
	}
	m, merr := parseManaCost(strings.TrimSpace(raw))
	pips := m.Pips[0] + m.Pips[1] + m.Pips[2] + m.Pips[3] + m.Pips[4]
	plainMana := merr == nil && !m.None && !m.X && !m.XX && !m.XXX && m.C == 0 &&
		len(m.Hybrid)+len(m.Twobrid)+len(m.Phyrexian) == 0 &&
		m.Generic >= 0 && m.Generic <= 15 && m.Generic+pips >= 1
	if !plainMana {
		return lc.echoActionUpkeep(raw)
	}
	for _, n := range m.Pips {
		if n > 7 {
			return refuse("echo cost %q (a pip count past the three bits the payment packs)", raw)
		}
	}
	lc.emit(lInstr{Op: "Cumulative", C: 2, B: cst(kop.EchoNoCounter), Note: "echo: no age counter; the payment and the sacrifice are skipped if the source is gone"})
	lc.emit(lInstr{Op: "UnlessPaid", Obj: lRef{Kind: rYou}, CS: "Mana", Skip: 1,
		A: cst(m.Generic), B: cst(packPips(m.Pips)), Note: "pay the echo cost, else sacrifice"})
	lc.emit(lInstr{Op: "Sacrifice", Obj: lRef{Kind: rYou}, A: cst(1), F: &lFilter{Clauses: []lClause{{Flags: fSelf}}}})
	return nil
}

// echoActionUpkeep lowers an echo cost that is an action rather than mana:
// Discard<N/Card> (Deepcavern Imp, Rakdos Headliner) or Sac<N/Land> (Skizzik
// Surger). gorge's echoActionAsk then asks which N cards or lands are chosen
// (kernel PayDiscard / PaySac). Everything else keeps an "echo cost" refusal.
func (lc *lowerCtx) echoActionUpkeep(raw string) error {
	cost, err := lc.costUnless(strings.TrimSpace(raw))
	if err != nil {
		return refuse("echo cost %q (only a plain mana cost, a default discard or a land sacrifice lowers)", raw)
	}
	if cost.Sac != nil {
		// Sac<N/Spec> costs sacrifice what the payer controls: say so in the
		// filter, as a kernel cost's Filter does (build.go cost).
		f := *cost.Sac
		f.Clauses = append([]lClause(nil), f.Clauses...)
		if cost.SacN <= 0 || len(f.Clauses) != 1 || f.Clauses[0] != (lClause{Types: tLand}) ||
			f.Players != 0 || f.Each || f.PickSumMax != nil || f.PickDistinctPower {
			return refuse("echo cost %q (only a plain mana cost, a default discard or a land sacrifice lowers)", raw)
		}
		f.Clauses[0].Ctrl = whoYou
		cost.Sac = &f
	}
	lc.trigOwnCost = true
	in, err := lc.payInstr(cost, lRef{Kind: rYou}, true)
	lc.trigOwnCost = false
	if err != nil || in.CS != "Discard" && in.CS != "Sacrifice" {
		return refuse("echo cost %q (only a plain mana cost, a default discard or a land sacrifice lowers)", raw)
	}
	in.Op, in.Skip = "UnlessPaid", 1
	lc.emit(lInstr{Op: "Cumulative", C: 2, B: cst(kop.EchoNoCounter), Note: "echo: no age counter; the payment and the sacrifice are skipped if the source is gone"})
	lc.emit(in)
	lc.emit(lInstr{Op: "Sacrifice", Obj: lRef{Kind: rYou}, A: cst(1), F: &lFilter{Clauses: []lClause{{Flags: fSelf}}}})
	return nil
}
