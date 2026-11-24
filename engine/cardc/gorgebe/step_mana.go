package gorgebe

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerStep(ir.VerbAddMana, printAddMana) }

// wubrg is the colour letters in ColorSet bit order.
const wubrg = "WUBRG"

// manaLetter is the colour letter of a single-colour symbol: W, U, B, R, G, or
// C for colourless.
func manaLetter(s ir.ManaSym) (string, bool) {
	switch s.Kind {
	case ir.ManaColorless:
		return "C", true
	case ir.ManaColored:
		switch s.Colors {
		case ir.ColorW, ir.ColorU, ir.ColorB, ir.ColorR, ir.ColorG:
			return s.Colors.String(), true
		}
	}
	return "", false
}

// sameLetter returns L when every symbol of st is the same single colour L.
func sameLetter(st *ir.Step) (string, bool) {
	if len(st.Mana) == 0 {
		return "", false
	}
	l, ok := manaLetter(st.Mana[0])
	if !ok {
		return "", false
	}
	for _, s := range st.Mana[1:] {
		if x, ok := manaLetter(s); !ok || x != l {
			return "", false
		}
	}
	return l, true
}

func printAddMana(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:add-mana", st.Src)
	if st.Optional || st.Who != nil {
		return nil, 0, bad
	}
	if st.Word == "each-color-among" && st.Filter != nil && len(st.Mana) == 0 && st.N == nil {
		v, err := p.valid(st.Filter)
		if err != nil {
			return nil, 0, err
		}
		return &node{API: "Mana", Params: []kv{{"Produced", "Special EachColorAmong_Valid " + v}}}, 1, nil
	}
	if len(st.Mana) == 0 || (st.N != nil && len(st.Mana) != 1) {
		return nil, 0, bad
	}
	n := &node{API: "Mana"}
	amount := func(count int) error {
		switch {
		case st.N != nil:
			a, err := p.amount(st.N)
			if err != nil {
				return err
			}
			n.set("Amount", a)
		case count > 1:
			n.set("Amount", strconv.Itoa(count))
		}
		return nil
	}
	if l, ok := sameLetter(st); ok {
		n.set("Produced", l)
		if err := amount(len(st.Mana)); err != nil {
			return nil, 0, err
		}
		return n, 1, nil
	}
	if len(st.Mana) == 1 {
		s := st.Mana[0]
		if s.Kind != ir.ManaColored {
			return nil, 0, bad
		}
		var ls []string
		for i := 0; i < len(wubrg); i++ {
			if s.Colors&(1<<i) != 0 {
				ls = append(ls, string(wubrg[i]))
			}
		}
		switch {
		case s.Colors&^(1<<len(wubrg)-1) != 0 || len(ls) < 2:
			return nil, 0, bad
		case len(ls) == 5:
			n.set("Produced", "Any")
		default:
			v := "Combo"
			for _, l := range ls {
				v += " " + l
			}
			n.set("Produced", v)
		}
		if err := amount(1); err != nil {
			return nil, 0, err
		}
		return n, 1, nil
	}
	// Several symbols of distinct single colours.
	if st.N != nil {
		return nil, 0, bad
	}
	seen := map[string]bool{} // membership only, never ranged
	v := ""
	for i, s := range st.Mana {
		l, ok := manaLetter(s)
		if !ok || seen[l] {
			return nil, 0, bad
		}
		seen[l] = true
		if i > 0 {
			v += " "
		}
		v += l
	}
	n.set("Produced", v)
	return n, 1, nil
}
