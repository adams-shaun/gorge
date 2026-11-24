package gorgebe

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Unprintable is a construct the backend cannot print yet.
type Unprintable struct{ What, Path string }

func (e *Unprintable) Error() string { return "gorgebe: cannot print " + e.What + " at " + e.Path }

// Print prints c as gorge script text. oracle[i] is face i's Oracle text.
func Print(c *ir.Card, oracle []string) ([]byte, error) { return print(c, oracle, true) }

// PrintCharacteristics prints the same text without any K: line. It fails only
// on the layout or the face count.
func PrintCharacteristics(c *ir.Card, oracle []string) ([]byte, error) {
	return print(c, oracle, false)
}

func print(c *ir.Card, oracle []string, abilities bool) ([]byte, error) {
	var b strings.Builder
	switch c.Layout {
	case ir.LayoutNormal:
	case ir.LayoutTransform:
		b.WriteString("AlternateMode:DoubleFaced\n")
	default:
		return nil, &Unprintable{What: "layout:" + c.Layout.String(), Path: c.Src}
	}
	if len(oracle) != len(c.Faces) {
		return nil, &Unprintable{What: "faces", Path: c.Src}
	}
	if c.Layout == ir.LayoutNormal && len(c.Faces) != 1 {
		return nil, &Unprintable{What: "faces", Path: c.Src}
	}
	for i, f := range c.Faces {
		if i > 0 {
			b.WriteString("ALTERNATE\n")
		}
		if err := printFace(&b, f, oracle[i], abilities); err != nil {
			return nil, err
		}
	}
	return []byte(b.String()), nil
}

func printFace(b *strings.Builder, f *ir.Face, oracle string, abilities bool) error {
	if abilities && len(f.Refusals) > 0 {
		r := f.Refusals[0]
		return &Unprintable{What: "refusal:" + r.Key, Path: r.Path}
	}
	if f.Name != "" {
		b.WriteString("Name:" + f.Name + "\n")
	}
	if f.HasManaCost {
		b.WriteString("ManaCost:" + manaCost(f.ManaCost) + "\n")
	} else {
		b.WriteString("ManaCost:no cost\n")
	}
	var words []string
	for _, l := range [][]string{f.Supertypes, f.Types, f.Subtypes} {
		for _, w := range l {
			words = append(words, Respell(w))
		}
	}
	if len(words) > 0 {
		b.WriteString("Types:" + strings.Join(words, " ") + "\n")
	}
	if f.Power != nil && f.Toughness != nil {
		b.WriteString("PT:" + pt(f.Power) + "/" + pt(f.Toughness) + "\n")
	}
	if f.Loyalty != nil {
		fmt.Fprintf(b, "Loyalty:%d\n", *f.Loyalty)
	}
	if f.Defense != nil {
		fmt.Fprintf(b, "Defense:%d\n", *f.Defense)
	}
	if f.ColorIndicator != 0 {
		var cs []string
		for i, n := range []string{"white", "blue", "black", "red", "green"} {
			if f.ColorIndicator&(1<<i) != 0 {
				cs = append(cs, n)
			}
		}
		b.WriteString("Colors:" + strings.Join(cs, ",") + "\n")
	}
	if abilities {
		for _, a := range f.Abilities {
			if a.Kind != ir.AbKeyword {
				return &Unprintable{What: "ability:" + a.Kind.String(), Path: a.Src}
			}
			k := a.Keyword
			if k == nil {
				return &Unprintable{What: "ability:keyword", Path: a.Src}
			}
			if k.N != nil || k.Cost != nil || k.Filter != nil {
				return &Unprintable{What: "keyword:" + k.Name, Path: a.Src}
			}
			b.WriteString("K:" + k.Name + "\n")
		}
	}
	if oracle != "" {
		b.WriteString("Oracle:" + oracle + "\n")
	}
	return nil
}

func pt(p *ir.PT) string {
	if !p.Star {
		return strconv.Itoa(int(p.N))
	}
	switch {
	case p.N > 0:
		return "*+" + strconv.Itoa(int(p.N))
	case p.N < 0:
		return "*-" + strconv.Itoa(int(-p.N))
	}
	return "*"
}

// hybridPairs is the order gorge spells a two-color pair.
var hybridPairs = [...]string{"WU", "UB", "BR", "RG", "GW", "WB", "UR", "BG", "RW", "GU"}

func letters(c ir.ColorSet) string { return c.String() }

func pair(c ir.ColorSet) string {
	s := letters(c)
	for _, p := range hybridPairs {
		if len(s) == 2 && strings.ContainsRune(p, rune(s[0])) && strings.ContainsRune(p, rune(s[1])) {
			return p
		}
	}
	return s
}

func manaCost(syms []ir.ManaSym) string {
	if len(syms) == 0 {
		return "0"
	}
	out := make([]string, len(syms))
	for i, s := range syms {
		switch s.Kind {
		case ir.ManaGeneric:
			out[i] = strconv.Itoa(int(s.N))
		case ir.ManaColored:
			out[i] = letters(s.Colors)
		case ir.ManaHybrid:
			out[i] = pair(s.Colors)
		case ir.ManaPhyrexian:
			out[i] = letters(s.Colors) + "P"
		case ir.ManaHybridPhyrexian:
			out[i] = pair(s.Colors) + "P"
		case ir.ManaTwoHybrid:
			out[i] = "2" + letters(s.Colors)
		case ir.ManaColorless:
			out[i] = "C"
		case ir.ManaX:
			out[i] = "X"
		case ir.ManaSnow:
			out[i] = "S"
		default:
			out[i] = "?"
		}
	}
	return strings.Join(out, " ")
}
