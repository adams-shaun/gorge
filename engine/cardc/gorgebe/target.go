package gorgebe

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { targetFn = printTarget }

func printTarget(p *pr, t *ir.Target) ([]kv, error) {
	if t.Filter == nil {
		return nil, unprintable("target:no-filter", t.Src)
	}
	v, err := p.valid(t.Filter)
	if err != nil {
		return nil, err
	}
	out := []kv{{"ValidTgts", v}}
	if t.Filter.Subject == ir.SubjSpell {
		out = append(out, kv{"TargetType", "Spell"})
	}
	var zs []string
	other := false
	for _, z := range t.Filter.Zones.Zones() {
		if z != ir.ZoneBattlefield && z != ir.ZoneStack {
			other = true
		}
		zs = append(zs, strings.ToUpper(z.String()[:1])+z.String()[1:])
	}
	if other {
		out = append(out, kv{"TgtZone", strings.Join(zs, ",")})
	}
	if !(t.Min == 1 && t.Max == 1) {
		out = append(out, kv{"TargetMin", itoa(t.Min)})
		if t.MaxV != nil {
			m, err := p.amount(t.MaxV)
			if err != nil {
				return nil, err
			}
			out = append(out, kv{"TargetMax", m})
		} else {
			out = append(out, kv{"TargetMax", itoa(t.Max)})
		}
	}
	if t.Distinct {
		out = append(out, kv{"TargetUnique", "True"})
	}
	return out, nil
}

func itoa(n int32) string { return strconv.Itoa(int(n)) }
