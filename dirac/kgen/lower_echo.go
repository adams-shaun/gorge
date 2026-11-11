package kgen

import "strings"

// echoUpkeep lowers the body of gorge's echo expansion (CR 702.30a;
// engine/cards/kw_echo.go: an upkeep trigger of ValidPlayer$ You carrying
// Echo$ True, body DB$ Echo | Cost$ <cost>; Cradle Guard, Mogg War Marshal,
// Avalanche Riders). gorge's startEcho asks the trigger's controller to pay
// the cost or sacrifice the source (echo_pay / echo_sac, over the same resPay
// machinery as a trigger's own cost) and asks nothing when the source is gone.
// The program is
//
//	Cumulative B=1 C=2    skips the rest when the source is gone (no age counter)
//	UnlessPaid You Mana   the pay-or-decline ask, skipping the sacrifice when paid
//	Sacrifice self        a decline
//
// The gate (Echo$ True) is the trigger's CondEchoOwed (lower_trigger.go).
// Only a cost of plain mana, generic and coloured pips, lowers: {X} (Volcano
// Hellion), the action costs of parseCumulativeAction (Discard<1/Card>,
// Sac<2/Land>), a hybrid or Phyrexian pip (gorge announces those before the
// election), a {C} pip and a free echo (no ask worth a decision) stay refused
// by name.
func (lc *lowerCtx) echoUpkeep(ps *params) error {
	raw, ok := ps.get("Cost")
	if !ok {
		return refuse("Echo without Cost$")
	}
	if lc.ab.Kind != akTriggered {
		return refuse("echo outside a trigger")
	}
	m, err := parseManaCost(strings.TrimSpace(raw))
	pips := m.Pips[0] + m.Pips[1] + m.Pips[2] + m.Pips[3] + m.Pips[4]
	if err != nil || m.None || m.X || m.XX || m.XXX || m.C != 0 || len(m.Hybrid)+len(m.Twobrid)+len(m.Phyrexian) != 0 ||
		m.Generic < 0 || m.Generic > 15 || m.Generic+pips < 1 {
		return refuse("echo cost %q (only a plain mana cost of generic and coloured pips lowers)", raw)
	}
	for _, n := range m.Pips {
		if n > 7 {
			return refuse("echo cost %q (a pip count past the three bits the payment packs)", raw)
		}
	}
	lc.emit(lInstr{Op: "Cumulative", C: 2, B: cst(1), Note: "echo: no age counter; the payment and the sacrifice are skipped if the source is gone"})
	lc.emit(lInstr{Op: "UnlessPaid", Obj: lRef{Kind: rYou}, CS: "Mana", Skip: 1,
		A: cst(m.Generic), B: cst(packPips(m.Pips)), Note: "pay the echo cost, else sacrifice"})
	lc.emit(lInstr{Op: "Sacrifice", Obj: lRef{Kind: rYou}, A: cst(1), F: &lFilter{Clauses: []lClause{{Flags: fSelf}}}})
	return nil
}
