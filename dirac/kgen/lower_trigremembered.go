package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TriggerRemembered$ inside a reflexive trigger's body.
//
// gorge effImmediateTrigger hands the "when you do" instance the parent
// chain's remembered set (RememberObjects$ Remembered, with the cards a
// settled UnlessCost$ Discard<> paid appended under RememberDiscarded$);
// TriggerRemembered resolves to that set (count_ref.go refTargetsTriggerRemembered)
// and reads Amount (the size), CardManaCost (the summed mana value) or
// GreatestCardManaCost (the largest). The set is fixed once the trigger is put
// on the stack and a card's mana value does not change with its zone, so the
// kernel captures the number there: OpReflexiveTrigger's B is evaluated in the
// parent's context and rides the pending entry's trigAmt, which the body reads
// as vTriggerAmount.

// trigRemWaiting marks a reflexive latent that may read TriggerRemembered$
// but has not yet.
const trigRemWaiting = "-"

// trigRemLKIPick marks a reflexive latent handed the battle-link pick's
// remembered set (RememberObjects$ RememberedLKI): the exiled card is off the
// battlefield, so its power is only readable from before it left (the
// pick's remLK accumulation, zone_hiddenpick.go pickLeaves), not from the
// remembered set itself.
const trigRemLKIPick = "remLKIPick"

// trigRemInit is trigRemAllowed narrowed to the chains that can honour it: the
// pick's set is only handed over when the parent's program already holds the
// battle-link pick (value_remlki.go remLKPick) that accumulates the last-known
// power. A RememberObjects$ RememberedLKI over any other writer (Goblin Crash
// Pilot's SacrificeAll) keeps refusing every TriggerRemembered$ read.
func (lc *lowerCtx) trigRemInit(ps *params) string {
	r := trigRemAllowed(ps)
	if r != trigRemLKIPick {
		return r
	}
	if lc.gateOverPick() {
		return r
	}
	return ""
}

// gateOverPick: the ability's program already holds the battle-link pick
// (value_remlki.go remLKPick), so the plain ConditionDefined$ Remembered gate
// reads the pick's own set. That set is the card's only remembered write
// (gorge zone_change.go: RememberLKI persists nothing on the source) and the
// chain closes with ClearRemembered$ (Yannik, Scavenging Sentinel), so no
// earlier resolution's memory can stand on the host and the persistent mark
// (kop.CondRemMark) is not ORed in.
func (lc *lowerCtx) gateOverPick() bool {
	if lc.ab == nil {
		return false
	}
	for _, in := range lc.ab.Prog {
		if remLKPick(in) {
			return true
		}
	}
	return false
}

// trigRemAllowed is the latent's initial TrigRem: waiting when the
// ImmediateTrigger hands the instance the remembered set (RememberObjects$
// Remembered), the pick's set when it hands RememberedLKI, else "" (every
// TriggerRemembered$ read refuses). It does not consume a parameter.
func trigRemAllowed(ps *params) string {
	switch strings.TrimSpace(ps.m["RememberObjects"]) {
	case "Remembered":
		return trigRemWaiting
	case "RememberedLKI":
		return trigRemLKIPick
	}
	return ""
}

// triggerRemembered lowers a TriggerRemembered$<rest> value head: legal only
// in the body of a reflexive latent that was handed the remembered set, one
// rest per latent (there is one trigAmt).
func (lc *lowerCtx) triggerRemembered(rest string) (lValue, error) {
	if strings.HasPrefix(rest, "CardCounters.") {
		// a delayed trigger's body reading the counters its remembered card
		// had (CR 608.2h last known information): the kernel reads the
		// entry's LKI snapshot, which is the trigger's own card only when the
		// delayed program remembered it (Ochre Jelly's Split)
		if ab := lc.ab; ab == nil || !ab.Latent || ab.Name != "DelayedTrigger" || !ab.DelayedSelf {
			return lValue{}, fmt.Errorf("value head TriggerRemembered$%s outside a delayed trigger remembering the trigger's own card", rest)
		}
		return lValue{Kind: vCountersOn, Ref: lRef{Kind: rDelayedRemembered}, S: strings.TrimPrefix(rest, "CardCounters.")}, nil
	}
	if rest == "CardPower" {
		if ab := lc.ab; ab != nil && ab.Latent && ab.Name == "ReflexiveTrigger" && ab.TrigRem == trigRemLKIPick {
			// the pick's remembered card's power (Yannik, Scavenging
			// Sentinel: RememberObjects$ RememberedLKI, the battle-link pick's
			// set). The card is in exile at the read, so the number is the
			// power it had before it left (CR 608.2h, gorge
			// TriggerRemembered$CardPower), which the pick accumulated into
			// the resolution's remLK (zone_hiddenpick.go pickLeaves;
			// refuseRemLKIPT pairs the read with that one writer). The body
			// reads the captured amount like every TriggerRemembered$ read;
			// trigRememberedCapture's B is ExRemLKIPT, evaluated in the
			// parent's context as the trigger is put on the stack.
			ab.TrigRem = rest
			lc.remLKN++
			return lValue{Kind: vTriggerAmount}, nil
		}
		// the captured object's power (Thorin, Mountain-king: the creature
		// the reflexive instance was handed by RememberObjects$ ParentTarget):
		// live while it is on the battlefield, its last known power once it
		// left (CR 608.2h), taken as the trigger was put on the stack (gorge
		// rules rememberedPTLKI; the kernel's reflexBites snapshot and
		// reflexBiteGone).
		if ab := lc.ab; ab == nil || !ab.Latent || ab.Name != "ReflexiveTrigger" || !ab.ReflexObj {
			return lValue{}, fmt.Errorf("value head TriggerRemembered$CardPower outside a reflexive trigger handed one object")
		}
		return lValue{Kind: vPowerOf, Ref: lRef{Kind: rDelayedRemembered}}, nil
	}
	switch rest {
	case "Amount", "CardManaCost", "GreatestCardManaCost":
	default:
		return lValue{}, fmt.Errorf("value head TriggerRemembered$%s", rest)
	}
	ab := lc.ab
	if ab == nil || !ab.Latent || ab.Name != "ReflexiveTrigger" || ab.TrigRem == "" {
		return lValue{}, fmt.Errorf("value head TriggerRemembered$%s outside a reflexive trigger handed RememberObjects$ Remembered", rest)
	}
	if ab.TrigRem != trigRemWaiting && ab.TrigRem != rest {
		return lValue{}, fmt.Errorf("TriggerRemembered$%s beside TriggerRemembered$%s: one captured amount", rest, ab.TrigRem)
	}
	ab.TrigRem = rest
	return lValue{Kind: vTriggerAmount}, nil
}

// trigRememberedCapture is the ReflexiveTrigger's B, the parent-context value
// of the rest the body read (the zero Const when it read none, leaving every
// other reflexive trigger unchanged). When the body read one it consumes
// RememberObjects$ and RememberDiscarded$; RememberDiscarded$ marks the
// gated Discard<> unless-cost (the instruction just emitted) to remember
// what it discards (kop.PayBRemember). Without a read the parameters stay
// unread, so the node refuses as before.
func (lc *lowerCtx) trigRememberedCapture(ps *params, lat *lAbility) (lValue, error) {
	rest := lat.TrigRem
	if rest == "" || rest == trigRemWaiting || rest == trigRemLKIPick {
		return lValue{}, nil // no TriggerRemembered$ read: the parameters stay unread, so the node refuses as before
	}
	if rest == trigRemXPaid {
		return lValue{Kind: vX}, nil // lower_triggerxcost.go: the parent's paid X
	}
	if rest == trigRemNumTimes {
		return lValue{Kind: vX}, nil // lower_numtimes.go: the parent's announced count
	}
	if rest == trigRemSacPower {
		// Spawner>Sacrificed$CardPower: the parent resolution's IfPaid
		// Sacrifice sum, captured as the reflexive trigger goes on the stack
		if err := lc.markParentIfPaidSacrifice(":power"); err != nil {
			return lValue{}, err
		}
		lc.remLKN++ // refuseRemLKIPT pairs this capture with the one writer
		return lValue{Kind: vRemLKIPT, N: 0}, nil
	}
	ps.consume("RememberObjects")
	if v, ok := ps.m["RememberDiscarded"]; ok {
		if strings.TrimSpace(v) != "True" {
			return lValue{}, refuse("ImmediateTrigger RememberDiscarded$ %q", v)
		}
		pr := lc.ab.Prog
		if len(pr) == 0 || (pr[len(pr)-1].Op != "UnlessPaid" && pr[len(pr)-1].Op != "UnlessSwitched") || pr[len(pr)-1].CS != "Discard" {
			return lValue{}, refuse("ImmediateTrigger RememberDiscarded$ without a Discard<> UnlessCost$")
		}
		pr[len(pr)-1].B = cst(kop.PayBRemember)
		pr[len(pr)-1].Note = noteRememberDiscarded // kremWrites: the unless-cost discard writes the remembered set
		ps.consume("RememberDiscarded")
	}
	switch rest {
	case "Amount":
		// gorge evalRememberedOK, as Remembered$Amount (value.go)
		return lValue{Kind: vRememberedCount, S: "ctx"}, nil
	case "CardManaCost":
		return lValue{Kind: vMVOf, Ref: lRef{Kind: rRemembered}}, nil
	case "CardPower":
		// the pick's remembered card's power before it left (Yannik;
		// triggerRemembered's trigRemLKIPick arm): the sum the pick
		// accumulated into the resolution's remLK (zone_hiddenpick.go
		// pickLeaves), read as the trigger is put on the stack
		return lValue{Kind: vRemLKIPT, N: 0}, nil
	}
	// GreatestCardManaCost: the largest mana value among the remembered cards,
	// wherever they went (a discard Rest in Peace or madness replaced to exile
	// still counts): ExMVOf over the remembered set with N == AggGreatestMV.
	// The set must be a Discard<> payment's.
	if _, ok := ps.m["RememberDiscarded"]; !ok {
		return lValue{}, refuse("TriggerRemembered$GreatestCardManaCost without RememberDiscarded$")
	}
	return lValue{Kind: vMVOf, Ref: lRef{Kind: rRemembered}, N: kop.AggGreatestMV}, nil
}
