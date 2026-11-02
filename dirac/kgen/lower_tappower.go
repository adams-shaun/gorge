package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
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
		c.Loyalty == 0 && c.SubCtr == 0 && c.PayLife == 0 && c.TapN == 0 && !c.TapHost && c.Forage == nil &&
		c.ExileG == nil && c.Energy == 0 && !c.ReturnSelf && !c.ExileSelf && c.Return == nil
}

// tapPowerValueStatic lowers Mode$ TapPowerValue (graveexile): the card's own Crew
// and/or Saddle tap-power cost values its power as N greater (gorge
// statics_assignment.go:233, statics.go:689). The bonus is a per-card
// constant added only while paying a Crew or Saddle cost, so Station never
// sees it; Value$ Toughness (gorge statics_assignment.go:269-271) makes the
// card crew / saddle with its toughness rather than its power
// (kop.TapPowToughness). Any other ValidSA (Station, a non-self ValidCard)
// refuses.
func (lc *lowerCtx) tapPowerValueStatic(s cards.Static) error {
	ps := newParams(s.Params)
	ps.consume("Mode", "Description")
	if ps.str("ValidCard") != "Card.Self" {
		return refuse("TapPowerValue ValidCard %q", ps.str("ValidCard"))
	}
	ps.consume("ValidCard")
	n, tough := int(kop.TapPowToughness), ps.str("Value") == "Toughness"
	if !tough {
		var err error
		n, err = strconv.Atoi(ps.str("Value"))
		if err != nil || n <= 0 || n > 127 {
			return refuse("TapPowerValue Value %q", ps.str("Value"))
		}
	}
	ps.consume("Value")
	crew, saddle := false, false
	for _, v := range strings.Split(ps.str("ValidSA"), ",") {
		switch v {
		case "Activated.Crew+Vehicle":
			crew = true
		case "Activated.Saddle+Mount":
			saddle = true
		default:
			return refuse("TapPowerValue ValidSA %q", v)
		}
	}
	if !crew && !saddle {
		return refuse("TapPowerValue without a Crew or Saddle cost")
	}
	ps.consume("ValidSA")
	if left := ps.unread(); len(left) > 0 {
		return refuse("TapPowerValue unread params %v", left)
	}
	if crew {
		lc.out.TapPowerCrew = n
	}
	if saddle {
		lc.out.TapPowerSaddle = n
	}
	return nil
}
