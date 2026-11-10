package kgen

import "strconv"

// kcounters: Station (CR 702.184a). gorge (pin ec2218895) keeps K:Station
// rules-side (rules/station.go): a priority special option "station" (as a
// sorcery, while another untapped creature you control exists), then one
// KChoose "station" Min=Max=1 over those creatures (always asked), then the
// Tap and a KeywordAbilityPush of the minted body
// `AB$ PutCounter | Defined$ Self | CounterType$ CHARGE | CounterNum$ X`
// (cards/kw_station.go), X the tapped creature's power read at RESOLUTION
// (stationAmount: in its new zone if it left, negative -> 0). The kernel
// lowers it as one activated ability flagged AfStation: CostSorcery and a
// one-creature tap (CostTapX over Creature.Other you control, untapped),
// whose pick the kernel poses before the push (ctr_kinds.go),
// and a program putting max(0, power of the tapped creature) CHARGE
// counters on the source. The tapped creature rides the stack entry's
// trigger object (refPT reads it off the battlefield as gorge does).

// lowerStation appends the Station ability for K:Station:<N>. The
// threshold abilities (STATION N+) are the face's own statics and
// triggers (counters_GE<N>_CHARGE), lowered with the rest of the face.
func (lc *lowerCtx) lowerStation(param string) error {
	if n, err := strconv.Atoi(param); err != nil || n < 1 {
		return refuse("station %q", param)
	}
	if lc.out.Types&tPermanent == 0 {
		return refuse("station on a nonpermanent")
	}
	for _, a := range lc.out.Abilities {
		if a.Station {
			return refuse("two Station keywords")
		}
	}
	cost, err := lc.costHand("tapXType<1/Creature.Other>")
	if err != nil {
		return err
	}
	// power plus every Station-scoped TapPowerValue bonus, read at
	// resolution (Tapestry Warden's toughness, Sulu's tripled power): the
	// kernel's vTapPower is the same read the Crew/Saddle payment uses
	pw := lValue{Kind: vTapPower, Ref: lRef{Kind: rTriggered}}
	amount := lValue{Kind: vCompare, N: 0, Op: opGE, Sub: []lValue{pw, {Kind: vBranches, Sub: []lValue{pw, cst(0)}}}}
	ab := &lAbility{Kind: akActivated, Index: len(lc.out.Abilities), Name: "Station", Cost: cost, Sorcery: true, Station: true}
	ab.Prog = []lInstr{{Op: "PutCounter", Obj: lRef{Kind: rSelf}, A: amount, CS: "CHARGE", Note: "Station: the tapped creature's power, at resolution"}}
	lc.out.Abilities = append(lc.out.Abilities, ab)
	return nil
}
