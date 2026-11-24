package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() { registerAbility(ir.AbKeyword, printKeywordArgs) }

// keywordLine prints a keyword with arguments as a K: line.
func (p *pr) keywordLine(k *ir.Keyword, src string) (string, error) {
	if k == nil || k.Name == "" {
		return "", unprintable("keyword:", src)
	}
	bad := unprintable("keyword:"+k.Name, src)
	switch k.Name {
	case "Kicker", "Flashback", "Miracle", "Ward":
		if k.Cost == nil || k.N != nil || k.Filter != nil {
			return "", bad
		}
		c, err := p.costText(k.Cost)
		if err != nil {
			return "", err
		}
		return "K:" + k.Name + ":" + c, nil
	case "Protection":
		f := k.Filter
		if f == nil || k.Cost != nil || k.N != nil || f.Op != ir.FPred || f.Pred != ir.PColor {
			return "", bad
		}
		for _, c := range colourNames {
			if f.Colors == c.c {
				return "K:Protection from " + lower(c.name), nil
			}
		}
	}
	return "", bad
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func printKeywordArgs(p *pr, a *ir.Ability) error {
	l, err := p.keywordLine(a.Keyword, a.Src)
	if err != nil {
		return err
	}
	p.lines = append(p.lines, l)
	return nil
}
