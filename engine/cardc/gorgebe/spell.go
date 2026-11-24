package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() { registerAbility(ir.AbSpell, printSpell) }

// printSpell prints a spell ability as an A:SP$ line. A spell with a cost, a
// condition, a keyword other than Kicker or any activation restriction is CC-23's.
func printSpell(p *pr, a *ir.Ability) error {
	for _, f := range []struct {
		name string
		set  bool
	}{
		{"cost", a.Cost != nil}, {"cond", a.Cond != nil}, {"keyword", a.Keyword != nil && a.Keyword.Name != "Kicker"},
		{"zones", a.Zones != 0}, {"sorceryOnly", a.SorceryOnly}, {"instantOnly", a.InstantOnly}, {"once", a.Once},
	} {
		if f.set {
			return unprintable("spell:"+f.name, a.Src)
		}
	}
	var kicker string
	if a.Keyword != nil {
		l, err := p.keywordLine(a.Keyword, a.Src)
		if err != nil {
			return err
		}
		kicker = l
	}
	head, err := p.program(a.Body)
	if err != nil {
		return err
	}
	if kicker != "" {
		p.lines = append(p.lines, kicker)
	}
	p.lines = append(p.lines, "A:"+p.render("SP", head))
	return nil
}
