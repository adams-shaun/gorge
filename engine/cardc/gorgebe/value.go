package gorgebe

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { valueFn = printValue }

// colourNames is the gorge spelling of each colour, in WUBRG order.
var colourNames = []struct {
	c    ir.ColorSet
	name string
}{
	{ir.ColorW, "White"}, {ir.ColorU, "Blue"}, {ir.ColorB, "Black"}, {ir.ColorR, "Red"}, {ir.ColorG, "Green"},
}

// isYou reports whether r is you: RefYou, or RefAll of "the player is you".
func isYou(r *ir.Ref) bool {
	if r == nil {
		return false
	}
	switch r.Kind {
	case ir.RefYou:
		return true
	case ir.RefAll:
		f := r.Filter
		return f != nil && f.Op == ir.FPred && f.Pred == ir.PPlayerIs && f.Ref != nil && f.Ref.Kind == ir.RefYou
	}
	return false
}

// isPlayerTarget reports whether r is a target slot of the innermost frame
// whose filter selects players.
func (p *pr) isPlayerTarget(r *ir.Ref) bool {
	if r == nil || r.Kind != ir.RefTarget {
		return false
	}
	f := p.frame()
	if f == nil || int(r.Slot) >= len(f.targets) {
		return false
	}
	t := f.targets[r.Slot]
	return t != nil && t.Filter != nil && t.Filter.Subject == ir.SubjPlayer
}

func isConst(v *ir.Value) bool { return v != nil && v.Kind == ir.VConst }

// body returns v's SVar body, or its decimal literal when v is a constant.
func (p *pr) body(v *ir.Value) (string, error) {
	if v == nil {
		return "", unprintable("value:nil", "")
	}
	if v.Kind == ir.VConst {
		return strconv.Itoa(int(v.N)), nil
	}
	return printValue(p, v)
}

func isLiteral(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func printValue(p *pr, v *ir.Value) (string, error) {
	no := func() (string, error) { return "", unprintable("value:"+v.Kind.String(), v.Src) }
	switch v.Kind {
	case ir.VX:
		return "Count$xPaid", nil
	case ir.VAdd, ir.VSub, ir.VMul:
		return printArith(p, v)
	case ir.VHalfDown, ir.VHalfUp:
		if len(v.Args) != 1 {
			return no()
		}
		b, err := p.body(v.Args[0])
		if err != nil {
			return "", err
		}
		if strings.Contains(b, "/") || isLiteral(b) {
			return "", unprintable("value:"+v.Kind.String()+"-nested", v.Src)
		}
		if v.Kind == ir.VHalfDown {
			return b + "/HalfDown", nil
		}
		return b + "/HalfUp", nil
	case ir.VCount:
		if v.Filter == nil {
			return no()
		}
		f, err := p.valid(v.Filter)
		if err != nil {
			return "", err
		}
		var zs []string
		if z := v.Filter.Zones; z != 0 && z != 1<<ir.ZoneBattlefield {
			for _, zone := range z.Zones() {
				n := zone.String()
				zs = append(zs, strings.ToUpper(n[:1])+n[1:])
			}
		}
		return "Count$Valid" + strings.Join(zs, ",") + " " + f, nil
	case ir.VCountersAmong:
		f := v.Filter
		if f != nil && f.Op == ir.FPred && f.Pred == ir.PIs && f.Ref != nil && f.Ref.Kind == ir.RefSelf && v.Word != "" {
			return "Count$CardCounters." + v.Word, nil
		}
		return no()
	case ir.VPower, ir.VToughness, ir.VManaValue:
		stat := map[ir.ValueKind]string{ir.VPower: "CardPower", ir.VToughness: "CardToughness", ir.VManaValue: "CardManaCost"}[v.Kind]
		if v.Ref != nil {
			switch v.Ref.Kind {
			case ir.RefSelf:
				return "Count$" + stat, nil
			case ir.RefTarget:
				return "Targeted$" + stat, nil
			case ir.RefEventObject:
				switch p.ev {
				case ir.EvEnters, ir.EvDies, ir.EvPutIntoGraveyard, ir.EvLeaves, ir.EvCountersPut:
					return "TriggeredCard$" + stat, nil
				}
			}
		}
		return no()
	case ir.VDistinctManaValues:
		if v.Filter == nil {
			return no()
		}
		f, err := p.valid(v.Filter)
		if err != nil {
			return "", err
		}
		return "Count$Valid " + f + "$DifferentCardManaCost", nil
	case ir.VLife:
		if isYou(v.Ref) {
			return "Count$YourLifeTotal", nil
		}
		if p.isPlayerTarget(v.Ref) {
			return "TargetedPlayer$LifeTotal", nil
		}
		return no()
	case ir.VStartingLife:
		if isYou(v.Ref) {
			return "Count$YourStartingLife", nil
		}
		return no()
	case ir.VLifeGainedThisTurn:
		if isYou(v.Ref) {
			return "Count$LifeYouGainedThisTurn", nil
		}
		return no()
	case ir.VDevotion:
		if v.Ref != nil && !isYou(v.Ref) {
			return no()
		}
		var cs []string
		for _, c := range colourNames {
			if v.Colors&c.c != 0 {
				cs = append(cs, c.name)
			}
		}
		switch len(cs) {
		case 1:
			return "Count$Devotion." + cs[0], nil
		case 2:
			return "Count$DevotionDual." + cs[0] + "." + cs[1], nil
		}
		return no()
	case ir.VChosenNumber:
		return "Count$ChosenNumber", nil
	case ir.VEventAmount:
		switch p.ev {
		case ir.EvGainsLife, ir.EvLosesLife:
			return "TriggerCount$LifeAmount", nil
		case ir.EvDealsCombatDamage, ir.EvDealsCombatDamageToPlayer, ir.EvDealsNoncombatDamageToPlayer:
			return "TriggerCount$DamageAmount", nil
		case ir.EvWouldDealDamage:
			return "ReplaceCount$DamageAmount", nil
		case ir.EvCountersPut:
			return "TriggerCount$Amount", nil
		}
		return "", unprintable("value:event-amount@"+p.ev.String(), v.Src)
	}
	return no()
}

// printArith prints VAdd, VSub and VMul: a fold of two constants, or one
// operation suffix on the non-constant operand's body.
func printArith(p *pr, v *ir.Value) (string, error) {
	no := func() (string, error) { return "", unprintable("value:"+v.Kind.String(), v.Src) }
	if len(v.Args) != 2 {
		return no()
	}
	a, b := v.Args[0], v.Args[1]
	if isConst(a) && isConst(b) {
		x, y := int64(a.N), int64(b.N)
		switch v.Kind {
		case ir.VAdd:
			x += y
		case ir.VSub:
			x -= y
		default:
			x *= y
		}
		return strconv.FormatInt(x, 10), nil
	}
	var k int32
	var rest *ir.Value
	switch {
	case isConst(b):
		k, rest = b.N, a
	case isConst(a) && v.Kind != ir.VSub:
		k, rest = a.N, b
	default:
		return no()
	}
	if k < 0 {
		return no()
	}
	body, err := p.body(rest)
	if err != nil {
		return "", err
	}
	if strings.Contains(body, "/") || isLiteral(body) {
		return "", unprintable("value:"+v.Kind.String()+"-nested", v.Src)
	}
	op := map[ir.ValueKind]string{ir.VAdd: "/Plus.", ir.VSub: "/Minus.", ir.VMul: "/Times."}[v.Kind]
	return body + op + strconv.Itoa(int(k)), nil
}
