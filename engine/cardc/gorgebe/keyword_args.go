package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerAbility(ir.AbKeyword, printKeywordArgs) }

// keywordLine prints a keyword with arguments as a K: line.
func (p *pr) keywordLine(k *ir.Keyword, src string) (string, error) {
	if k == nil || k.Name == "" {
		return "", unprintable("keyword:", src)
	}
	bad := unprintable("keyword:"+k.Name, src)
	switch k.Name {
	case "Kicker", "Flashback", "Miracle", "Ward", "Surge":
		if k.Cost == nil || k.N != nil || k.Filter != nil {
			return "", bad
		}
		c, err := p.costText(k.Cost)
		if err != nil {
			return "", err
		}
		return "K:" + k.Name + ":" + c, nil
	case "Equip":
		// Forge's default: equip to a creature you control.
		if k.Cost == nil || k.N != nil || !isCreatureType(k.Filter) {
			return "", bad
		}
		c, err := p.costText(k.Cost)
		if err != nil {
			return "", err
		}
		return "K:Equip:" + c, nil
	case "Enchant":
		if k.Cost != nil || k.N != nil || k.Filter == nil {
			return "", bad
		}
		v, err := p.valid(k.Filter)
		if err != nil {
			return "", err
		}
		d, ok := typeUnionText(k.Filter)
		if !ok {
			return "K:Enchant:" + v, nil
		}
		return "K:Enchant:" + v + ":" + d, nil
	case "Hexproof", "Affinity":
		if k.Cost != nil || k.N != nil || k.Filter == nil {
			return "", bad
		}
		v, err := p.valid(k.Filter)
		if err != nil {
			return "", err
		}
		return "K:" + k.Name + ":" + v, nil
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

// typeUnionText is the lower-case list "creature, land, or planeswalker" of an
// any-filter over card types, the description Forge appends to such an Enchant
// line; false when f is not a union of two or more card types.
func typeUnionText(f *ir.Filter) (string, bool) {
	if f.Op != ir.FAny || len(f.Args) < 2 {
		return "", false
	}
	var ws []string
	for _, a := range f.Args {
		if !isPred(a, ir.PCardType) {
			return "", false
		}
		ws = append(ws, lower(a.Word))
	}
	if len(ws) == 2 {
		return ws[0] + " or " + ws[1], true
	}
	return strings.Join(ws[:len(ws)-1], ", ") + ", or " + ws[len(ws)-1], true
}

func isCreatureType(f *ir.Filter) bool {
	return f != nil && f.Op == ir.FPred && f.Pred == ir.PCardType && f.Word == "Creature" && len(f.Args) == 0 && f.Ref == nil
}
