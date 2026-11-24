package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerAbility(ir.AbActivated, printActivated) }

// printActivated prints an activated ability as an A:AB$ line.
func printActivated(p *pr, a *ir.Ability) error {
	if a.Cost == nil {
		return unprintable("activated:no-cost", a.Src)
	}
	p.sacSelf = false
	for _, c := range a.Cost.Parts {
		if c.Kind == ir.CostSacrifice && c.Obj != nil && c.Obj.Kind == ir.RefSelf {
			p.sacSelf = true
		}
	}
	head, err := p.program(a.Body)
	p.sacSelf = false
	if err != nil {
		return err
	}
	cost, err := p.costText(a.Cost)
	if err != nil {
		return err
	}
	head.Params = append([]kv{{"Cost", cost}}, head.Params...)
	if a.InstantOnly {
		if head.API != "Mana" {
			return unprintable("activated:instant-only", a.Src)
		}
		head.set("InstantSpeed", "True")
	}
	if a.SorceryOnly {
		head.set("SorcerySpeed", "True")
	}
	if a.Once {
		head.set("ActivationLimit", "1")
	}
	if a.Zones != 0 && a.Zones != ir.MakeZoneSet(ir.ZoneBattlefield) {
		head.set("ActivationZone", zoneList(a.Zones))
	}
	for _, c := range a.Cost.Parts {
		if c.Kind == ir.CostLoyalty {
			head.set("Planeswalker", "True")
			break
		}
	}
	if a.Cond != nil {
		f, err := p.cond(a.Cond)
		if err != nil {
			return err
		}
		f = p.settle(f)
		ps, err := f.onActivation()
		if err != nil {
			return err
		}
		for _, x := range ps {
			head.set(x.K, x.V)
		}
	}
	p.lines = append(p.lines, "A:"+p.render("AB", head))
	return nil
}

// zoneList spells zs as capitalised, comma-joined zone words.
func zoneList(zs ir.ZoneSet) string {
	var out []string
	for _, z := range zs.Zones() {
		out = append(out, zoneWord(z))
	}
	return strings.Join(out, ",")
}
