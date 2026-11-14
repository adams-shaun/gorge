package kgen

import (
	"strconv"
	"strings"
)

// The announced-X floor XMin<N> and the announced sacrifice Sac<X/Spec>
// (CR 107.3, 601.2b; gorge Cost.XMin, cost.CostPart.Announced; kernel
// dirac/cost_sacx.go). Both ride a spell's own Cost$ or an activated
// ability's own cost only: the kernel wires the floor into the X ask
// (addXOptions) and the announced count into the cost-pick stage, and no
// other X stage (loyalty, counter removal, evidence) reads a floor.

// xminToken reads the bare cost token XMin<N> (N >= 1) into c.
func xminToken(c *lCost, t string) error {
	n, err := strconv.Atoi(strings.TrimPrefix(t, "XMin"))
	if err != nil || n < 1 || n > 255 || c.XMin != 0 {
		return refuse("xmin cost %q", t)
	}
	c.XMin = n
	return nil
}

// sacXCost reads Sac<X/Spec> (spec not the source or its host) into c: the
// announced X permanents matching Spec are sacrificed (kop.CostSac, N 0).
func (lc *lowerCtx) sacXCost(c *lCost, parts []string, t string) error {
	if len(parts) < 2 || len(parts) > 3 {
		return refuse("cost token %q", t)
	}
	switch parts[1] {
	case "CARDNAME", "NICKNAME", "OriginalHost":
		return refuse("cost token %q", t)
	}
	if !lc.lifeXSpell && !lc.actCost {
		return refuse("sacrifice cost %q outside a spell's or activated ability's own cost", t)
	}
	if c.Sac != nil {
		return refuse("sacrifice cost %q beside a second sacrifice part", t)
	}
	f, err := lc.filter(strings.ReplaceAll(parts[1], ";", ","))
	if err != nil {
		return err
	}
	c.Sac, c.SacN, c.SacX = f, 0, true
	return nil
}

// announcedXCheck refuses the announced-X parts the kernel cannot pose
// together: a sacrifice count beside another announced part, a sacrifice
// count beside the source's own sacrifice, and a floor with no supported
// announced X to floor. s is the cost string (for the reason).
func (lc *lowerCtx) announcedXCheck(c *lCost, s string) error {
	if c.SacX {
		if c.LifeX || c.LoyaltyX || c.SubCtrX || c.EvidenceX || c.Waterbend > 0 {
			return refuse("sacrifice cost %q beside a second announced-X part", s)
		}
		if c.SacSelf {
			return refuse("sacrifice cost %q beside a self-sacrifice", s)
		}
	}
	if c.ExileGX {
		if !c.Mana.X || c.SacX || c.LifeX || c.LoyaltyX || c.SubCtrX || c.EvidenceX || c.Waterbend > 0 || c.WaterbendX || c.XMin != 0 || c.SubAmongN > 0 {
			return refuse("exile cost ExileFromGrave<X/..> in %q beside no printed {X} or a second announced-X part", s)
		}
	}
	if c.XMin == 0 {
		return nil
	}
	if lc.lifeXSpell {
		// gorge folds only the non-mana parts of a spell's own Cost$ into the
		// cast (pay.foldAdditionalCost: the mana part restates the printed
		// cost, and the floor rides on it): Mind Grind's {X} is offered at 0
		// (lockstep; gorge finding). Refused until gorge floors the cast.
		return refuse("xmin cost %q on a spell's Cost$: gorge folds no floor into the cast (gorge finding)", s)
	}
	if !lc.actCost {
		return refuse("xmin cost %q beside no supported announced X", s)
	}
	if c.LifeX || c.LoyaltyX || c.SubCtrX || c.EvidenceX || c.Waterbend > 0 || c.SubAmongN > 0 {
		return refuse("xmin cost %q beside no supported announced X", s)
	}
	if c.Mana.X || c.SacX {
		return nil
	}
	return refuse("xmin cost %q beside no supported announced X", s)
}

// xminParam folds an activated ability's XMin$ parameter into its cost's
// floor by maximum (gorge xAsk with xMinAbilityParam): a positive integer
// only; anything else is ignored, as gorge ignores it. Read only on a
// non-mana activated ability (gorge reads nothing on a spell or a mana
// ability, so kgen's unread-params refusal stays there). On a cost that
// announces no X gorge binds nothing, so the parameter is read and dropped;
// beside an announced X the kernel has no floor for, it refuses.
func xminParam(c *lCost, ps *params, activated bool) error {
	if !activated {
		return nil
	}
	v, ok := ps.get("XMin")
	if !ok {
		return nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return nil
	}
	if c.LifeX || c.LoyaltyX || c.SubCtrX || c.EvidenceX || c.Waterbend > 0 || c.SubAmongN > 0 || n > 255 {
		return refuse("xmin cost %q beside no supported announced X", "XMin$ "+strings.TrimSpace(v))
	}
	if c.Mana.X || c.SacX {
		c.XMin = max(c.XMin, n)
	}
	return nil
}
