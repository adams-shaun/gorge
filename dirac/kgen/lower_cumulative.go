package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// cumulativeUpkeep lowers the body of gorge's cumulative upkeep expansion
// (CR 702.24a; engine/cards/kw_cumulativeupkeep.go: an upkeep trigger of
// ValidPlayer$ You, body DB$ CumulativeUpkeep | Cost$ <cost>, Mystic Remora).
// gorge's startCumulativeUpkeep puts an age counter on the source, then asks
// its controller to pay the cost once per age counter or sacrifice it
// (cumulative_pay / cumulative_sac, over the same resPay machinery as a
// trigger's own cost). The program is
//
//	Cumulative                  the age counter (skips the rest when the source is gone)
//	UnlessPaid You <kind>       the pay-or-decline ask, skipping the sacrifice when paid
//	Sacrifice self              a decline
//
// gorge multiplies the printed cost by the age counters n (scaleCost,
// cumulativeUpkeep.actionRemaining) in one ask:
//
//	mana        every generic, coloured and hybrid pip n times (n hybrid
//	            announcements first, pipAnnounceAsk), then one election
//	PayLife<k>  n*k life
//	Sac<k/..>   n*k permanents in one pick (continueCumulativeAction)
//	Discard<k/Card>  n*k cards in one pick
//
// A generic cost lowers as Mana with the amount n*AGE. A coloured or hybrid
// mana cost carries the cost of ONE counter (ExtCumulative B =
// kop.CumulativeScaled) and the kernel multiplies it by the age counters
// when it poses the ask (dirac/trig_cumulative_pay.go). A life, default
// Discard<k/Card> or Sac<k/Land|Creature> cost lowers as the matching
// UnlessPaid with the amount k*AGE (the pick is the echo-style object choice).
// A {X}, Phyrexian, {C} or snow cost, mana beside life, and the other
// parseCumulativeAction kinds (AddCounter, AddMana, Draw, GainLife,
// PutCardToLibFromSameGrave, ...) stay refused by name.
func (lc *lowerCtx) cumulativeUpkeep(ps *params) error {
	raw, ok := ps.get("Cost")
	if !ok {
		return refuse("CumulativeUpkeep without Cost$")
	}
	if lc.ab.Kind != akTriggered {
		return refuse("cumulative upkeep outside a trigger")
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 || n > 15 {
		return lc.cumulativeCost(raw)
	}
	lc.emit(lInstr{Op: "Cumulative", C: 2, Note: "age counter; the payment and the sacrifice are skipped if the source is gone"})
	lc.emit(lInstr{Op: "UnlessPaid", Obj: lRef{Kind: rYou}, CS: "Mana", Skip: 1,
		A:    lValue{Kind: vCountersOn, Ref: lRef{Kind: rSelf}, S: "AGE", Mul: n},
		B:    cst(0),
		Note: "pay the cost once per age counter, else sacrifice"})
	lc.emit(lInstr{Op: "Sacrifice", Obj: lRef{Kind: rYou}, A: cst(1), F: &lFilter{Clauses: []lClause{{Flags: fSelf}}}})
	return nil
}

// cumulativeCost lowers a cumulative upkeep cost that is not a bare generic
// amount (see cumulativeUpkeep).
func (lc *lowerCtx) cumulativeCost(raw string) error {
	refusal := func() error {
		return refuse("cumulative upkeep cost %q (only a mana, life, default discard or land or creature sacrifice cost lowers)", raw)
	}
	cost, err := lc.costUnless(strings.TrimSpace(raw))
	if err != nil {
		return refusal()
	}
	if cost.Sac != nil {
		// Sac<N/Spec> sacrifices what the payer controls: say so in the
		// filter, as a kernel cost's Filter does (build.go cost).
		f := *cost.Sac
		f.Clauses = append([]lClause(nil), f.Clauses...)
		if cost.SacN <= 0 || len(f.Clauses) != 1 || f.Clauses[0] != (lClause{Types: tLand}) && f.Clauses[0] != (lClause{Types: tCreature}) ||
			f.Players != 0 || f.Each || f.PickSumMax != nil || f.PickDistinctPower {
			return refusal()
		}
		f.Clauses[0].Ctrl = whoYou
		cost.Sac = &f
	}
	lc.trigOwnCost = true
	in, err := lc.payInstr(cost, lRef{Kind: rYou}, true)
	lc.trigOwnCost = false
	if err != nil {
		return refusal()
	}
	head := lInstr{Op: "Cumulative", C: 2, Note: "age counter; the payment and the sacrifice are skipped if the source is gone"}
	in.Op, in.Skip = "UnlessPaid", 1
	switch in.CS {
	case "Mana":
		m := cost.Mana
		pips := m.Pips[0] + m.Pips[1] + m.Pips[2] + m.Pips[3] + m.Pips[4]
		if in.C != 0 || cost.PayLife != 0 || m.None || m.X || m.XX || m.XXX || m.C != 0 || len(m.Twobrid)+len(m.Phyrexian) != 0 ||
			in.A.Kind != vConst || in.B.Kind != vConst || m.Generic < 0 || m.Generic > 15 || pips+len(m.Hybrid) < 1 {
			return refusal()
		}
		for _, p := range m.Pips {
			if p > 7 {
				return refuse("cumulative upkeep cost %q (a pip count past the three bits the payment packs)", raw)
			}
		}
		head.B = cst(kop.CumulativeScaled)
		head.Note += "; the kernel multiplies the unit cost below by the age counters"
		in.Note = "pay the cost of one age counter times the age counters, else sacrifice"
	case "PayLife", "Discard", "Sacrifice":
		if in.A.Kind != vConst || in.A.N < 1 || cost.Mana.Generic != 0 || cost.PayLife != 0 && in.CS != "PayLife" {
			return refusal()
		}
		in.A = lValue{Kind: vCountersOn, Ref: lRef{Kind: rSelf}, S: "AGE", Mul: in.A.N}
		in.Note = "pay the cost once per age counter, else sacrifice"
	default:
		return refusal()
	}
	lc.emit(head)
	lc.emit(in)
	lc.emit(lInstr{Op: "Sacrifice", Obj: lRef{Kind: rYou}, A: cst(1), F: &lFilter{Clauses: []lClause{{Flags: fSelf}}}})
	return nil
}
