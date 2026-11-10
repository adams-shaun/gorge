package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Aggregate taps:
// Crew (CR 702.122), Saddle (CR 702.171) and Exhaust (CR 702.177). gorge
// at the pin (ec2218895) expands K:Crew / K:Saddle into one AB$ each
// (cards/kw_crew.go crewAbilitySA, kw_saddle.go saddleAbilitySA) whose
// Cost$ is tapXType<Any/Creature.Other+withTotalPowerGE<N>>; the floor is
// moved out of the spec into CostPart.MinPower (rules/cost/parse.go
// stripGroupPowerFloor) and paid by one KChoose "tapcost" (Min 1, Max the
// candidates, MinSum N, each option valued at the candidate's power;
// rules/cast_asks.go tapPermanentCostAsk), the kernel's CostTapPower.

// tapPowerCost lowers tapXType<Any/Spec+withTotalPowerGE<N>[/description]>
// into c.TapPowN / c.TapPowF. The Any form without the floor (an election
// whose effect scales with the taps) and every X form refuse.
func (lc *lowerCtx) tapPowerCost(c *lCost, parts []string, t string) error {
	if len(parts) < 2 || len(parts) > 3 || c.TapPowN != 0 || c.TapN != 0 {
		return refuse("tap cost %q", t)
	}
	spec := strings.ReplaceAll(parts[1], ";", ",")
	i := strings.Index(spec, "withTotalPowerGE")
	if i < 0 {
		return refuse("tap cost %q: any number without a total-power floor", t)
	}
	j := i + len("withTotalPowerGE")
	k := j
	for k < len(spec) && spec[k] >= '0' && spec[k] <= '9' {
		k++
	}
	n, err := strconv.Atoi(spec[j:k])
	if err != nil || n < 1 || n > 255 {
		return refuse("tap cost %q: floor", t)
	}
	spec = strings.TrimSuffix(spec[:i]+spec[k:], "+")
	if strings.Contains(spec, ",") || strings.Contains(spec, "withTotalPower") {
		return refuse("tap cost %q: spec", t)
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	for i := range f.Clauses {
		f.Clauses[i].Ctrl = whoYou
		f.Clauses[i].Flags |= fUntapped
		if f.Clauses[i].Types == 0 {
			f.Clauses[i].Types = tCreature
		}
	}
	c.TapPowN, c.TapPowF = n, f
	return nil
}

// tapPowerAlone: the any-number tap is an activated ability's whole cost
// (Crew, Saddle, Mossbridge Troll): no mana, no {T}, no other part. gorge's
// planner never prices such an ability (no mana), and the kernel's pick
// owns the build stage alone.
func tapPowerAlone(c lCost) bool {
	return c.Mana.None && !c.Tap && !c.Untap && !c.SacSelf && c.Sac == nil && c.DiscardN == 0 && !c.DiscardSelf &&
		c.Loyalty == 0 && c.SubCtr == 0 && c.PayLife == 0 && c.TapN == 0 && !c.TapHost && !c.SacHost && c.Forage == nil &&
		c.ExileG == nil && c.Energy == 0 && !c.ReturnSelf && !c.ExileSelf && c.Return == nil
}

// tapPowerScope is one ValidSA$ alternative: the action kind and the host
// subtype it names (Spacecraft, Vehicle, Mount), if any.
type tapPowerScope struct {
	kind string
	host string
}

// tapPowerScopes parses a TapPowerValue ValidSA$ into one static per
// alternative. Forge spells the value as a comma-separated OR list of
// "Activated.<kind>[+<qualifier>]"; an empty value admits every activated
// action, as gorge's tapPowerSAScope does. Any other shape (a Spell scope,
// a kind this build cannot classify) refuses.
func tapPowerScopes(validSA string) ([]tapPowerScope, error) {
	v := strings.TrimSpace(validSA)
	if v == "" {
		return []tapPowerScope{{kind: "Crew"}, {kind: "Saddle"}, {kind: "Station"}}, nil
	}
	var out []tapPowerScope
	for _, alt := range strings.Split(v, ",") {
		alt = strings.TrimSpace(alt)
		kind, rest, ok := strings.Cut(alt, ".")
		if !ok || kind != "Activated" {
			return nil, refuse("TapPowerValue ValidSA %q", alt)
		}
		token, qualifier, _ := strings.Cut(rest, "+")
		switch token = strings.TrimSpace(token); token {
		case "Crew", "Saddle", "Station":
		default:
			return nil, refuse("TapPowerValue ValidSA %q", alt)
		}
		sc := tapPowerScope{kind: token, host: strings.TrimSpace(qualifier)}
		out = append(out, sc)
	}
	if len(out) == 0 {
		return nil, refuse("TapPowerValue without a ValidSA action")
	}
	return out, nil
}

// tapPowerSelfValue reports whether v reads only the tapped creature: every
// ref is the candidate (rSelf in the SVar, evaluated with the candidate as its
// source). Sulu's Value$ X = Count$CardPower/Twice is that shape; a body
// naming a target or the trigger's object is not the candidate and refuses.
func tapPowerSelfValue(v lValue) bool {
	if v.Ref.Kind != rNone && v.Ref.Kind != rSelf {
		return false
	}
	for _, s := range v.Sub {
		if !tapPowerSelfValue(s) {
			return false
		}
	}
	return true
}

// tapPowerValueStatic lowers Mode$ TapPowerValue (graveexile, Tapestry
// Warden, Sulu): one StatTapPower static per ValidSA$ alternative. Filter is
// ValidCard$, AddSub the alternative's host subtype, Amount the value
// (Toughness sentinel, a literal 1..127, or a lowered SVar read over the
// tapped creature). The kernel reads the same static walk for the Crew/Saddle
// payment and for Station's amount at resolution (gorge
// statics_assignment.go tapPowerValueFor). The old per-card TapPowerCrew /
// TapPowerSaddle constants stay in kop.Card for existing tables but are no
// longer emitted: the static path replaces them with identical results.
func (lc *lowerCtx) tapPowerValueStatic(s cards.Static) error {
	ps := newParams(s.Params)
	ps.consume("Mode", "Description")
	f, err := lc.filter(ps.str("ValidCard"))
	if err != nil {
		return err
	}
	ps.consume("ValidCard")

	var flags []string
	var amount lValue
	switch v := strings.TrimSpace(ps.str("Value")); v {
	case "Toughness":
		flags = append(flags, "Toughness")
	default:
		if n, err := strconv.Atoi(v); err == nil {
			if n <= 0 || n > 127 {
				return refuse("TapPowerValue Value %q", v)
			}
			amount = cst(n)
			break
		}
		body, ok := lc.face.SVars[v]
		if !ok {
			return refuse("TapPowerValue Value %q", v)
		}
		val, err := lc.svarValue(body, 0)
		if err != nil {
			return refuse("TapPowerValue Value %q: %v", v, err)
		}
		if !tapPowerSelfValue(val) {
			return refuse("TapPowerValue Value %q reads beyond the tapped creature", v)
		}
		amount = val
	}
	ps.consume("Value")

	scopes, err := tapPowerScopes(ps.str("ValidSA"))
	if err != nil {
		return err
	}
	ps.consume("ValidSA")
	if left := ps.unread(); len(left) > 0 {
		return refuse("TapPowerValue unread params %v", left)
	}
	for _, sc := range scopes {
		st := lStatic{Kind: "TapPower", F: f, Amount: amount}
		if sc.host != "" {
			if st.F2, err = lc.filter(sc.host); err != nil {
				return err
			}
		}
		st.Flags = append(st.Flags, sc.kind)
		st.Flags = append(st.Flags, flags...)
		lc.out.Statics = append(lc.out.Statics, st)
	}
	return nil
}
