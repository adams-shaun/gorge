package gorgebe

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// This file is the cost printer (CC-22b): Cost$ and UnlessCost$ text. CC-23
// reuses it for activated abilities and keyword costs.

// zoneWord is a zone's gorge spelling: Battlefield, Hand, Library, ...
func zoneWord(z ir.Zone) string {
	s := z.String()
	return strings.ToUpper(s[:1]) + s[1:]
}

// onlyZone reports whether f selects from exactly the zone z at its top level.
func onlyZone(f *ir.Filter, z ir.Zone) bool {
	return f != nil && f.Zones == ir.MakeZoneSet(z)
}

// singleZone returns f's zone when it names exactly one.
func singleZone(f *ir.Filter) (ir.Zone, bool) {
	if f == nil {
		return ir.ZoneNone, false
	}
	zs := f.Zones.Zones()
	if len(zs) != 1 {
		return ir.ZoneNone, false
	}
	return zs[0], true
}

// chooseOf reports whether r is a choice made by you: RefChoose with Of nil or you.
func chooseOf(r *ir.Ref) bool {
	return r != nil && r.Kind == ir.RefChoose && (r.Of == nil || isYou(r.Of))
}

// costText prints c as Cost$ text: the mana symbols, then each part in order.
func (p *pr) costText(c *ir.Cost) (string, error) {
	if c == nil || (!c.HasMana && len(c.Parts) == 0) {
		src := ""
		if c != nil {
			src = c.Src
		}
		return "", unprintable("cost:empty", src)
	}
	var out []string
	if c.HasMana {
		m, err := manaCost(c.Mana)
		if err != nil {
			return "", err
		}
		out = append(out, m)
	}
	for _, part := range c.Parts {
		s, err := p.costPart(part)
		if err != nil {
			return "", err
		}
		out = append(out, s)
	}
	return strings.Join(out, " "), nil
}

func (p *pr) costPart(c *ir.CostPart) (string, error) {
	bad := unprintable("cost:"+c.Kind.String(), c.Src)
	// n is the part's count; a chosen object's own count wins.
	n := func() (string, error) {
		v := c.N
		if c.Obj != nil && c.Obj.Kind == ir.RefChoose && c.Obj.N != nil {
			v = c.Obj.N
		}
		if v == nil {
			return "1", nil
		}
		return p.amount(v)
	}
	self := c.Obj != nil && c.Obj.Kind == ir.RefSelf
	with := func(head, tail string) (string, error) {
		k, err := n()
		if err != nil {
			return "", err
		}
		return head + "<" + k + "/" + tail + ">", nil
	}
	chosen := func(head string, z ir.Zone) (string, error) {
		if !chooseOf(c.Obj) || (z != ir.ZoneNone && !onlyZone(c.Obj.Filter, z)) || c.Obj.Filter == nil {
			return "", bad
		}
		v, err := p.valid(c.Obj.Filter)
		if err != nil {
			return "", err
		}
		return with(head, v)
	}
	switch c.Kind {
	case ir.CostTapSelf:
		return "T", nil
	case ir.CostUntapSelf:
		return "Q", nil
	case ir.CostSacrifice:
		if self {
			return "Sac<1/CARDNAME>", nil
		}
		return chosen("Sac", ir.ZoneNone)
	case ir.CostDiscard:
		if self {
			return "Discard<1/CARDNAME>", nil
		}
		return chosen("Discard", ir.ZoneHand)
	case ir.CostPayLife:
		k, err := n()
		if err != nil {
			return "", err
		}
		return "PayLife<" + k + ">", nil
	case ir.CostExile:
		if self {
			return "Exile<1/CARDNAME>", nil
		}
		if chooseOf(c.Obj) && onlyZone(c.Obj.Filter, ir.ZoneGraveyard) {
			return chosen("ExileFromGrave", ir.ZoneGraveyard)
		}
		return chosen("ExileFromHand", ir.ZoneHand)
	case ir.CostRemoveCounters:
		if !self || c.Word == "" {
			return "", bad
		}
		return with("SubCounter", c.Word)
	case ir.CostReturnToHand:
		if !self {
			return "", bad
		}
		return "Return<1/CARDNAME>", nil
	case ir.CostTapUntapped:
		return chosen("tapXType", ir.ZoneNone)
	case ir.CostReveal:
		return chosen("Reveal", ir.ZoneHand)
	case ir.CostMill:
		k, err := n()
		if err != nil {
			return "", err
		}
		return "Mill<" + k + ">", nil
	case ir.CostLoyalty:
		if !isConst(c.N) {
			return "", bad
		}
		if c.N.N >= 0 {
			return "AddCounter<" + strconv.Itoa(int(c.N.N)) + "/LOYALTY>", nil
		}
		return "SubCounter<" + strconv.Itoa(int(-c.N.N)) + "/LOYALTY>", nil
	}
	return "", bad
}
