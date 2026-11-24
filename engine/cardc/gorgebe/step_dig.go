package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() {
	registerStep(ir.VerbSearch, printSearch)
	registerStep(ir.VerbLook, printDig)
	registerStep(ir.VerbReveal, printDig)
	registerStep(ir.VerbLookAtHand, printRevealHand)
	registerStep(ir.VerbRevealHand, printRevealHand)
	registerStep(ir.VerbChoose, func(_ *pr, steps []*ir.Step) (*node, int, error) {
		return nil, 0, unprintable("verb:choose", steps[0].Src)
	})
	registerStep(ir.VerbShuffle, printShuffle)
}

func zsteps(prog *ir.Program) []*ir.Step {
	if prog == nil {
		return nil
	}
	return prog.Steps
}

func printSearch(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:search", st.Src)
	th := zsteps(st.Then)
	if len(th) != 3 || !zwhoYou(st) {
		return nil, 0, bad
	}
	ch, mv, sh := th[0], th[1], th[2]
	if ch.Verb != ir.VerbChoose || ch.Optional || ch.Obj == nil || !chooseOf(ch.Obj) || !onlyZone(ch.Obj.Filter, ir.ZoneLibrary) ||
		mv.Verb != ir.VerbMove || mv.Optional || mv.Obj == nil || mv.Obj.Kind != ir.RefReceipt || mv.Obj.Receipt != ir.RcFound || mv.To == ir.ZoneNone ||
		sh.Verb != ir.VerbShuffle || sh.Optional {
		return nil, 0, bad
	}
	tapped, ok := zenter(mv.Enter)
	if !ok {
		return nil, 0, bad
	}
	v, err := p.valid(ch.Obj.Filter)
	if err != nil {
		return nil, 0, err
	}
	k, err := p.amount(ch.Obj.N)
	if err != nil {
		return nil, 0, err
	}
	n := &node{API: "ChangeZone", Params: []kv{
		{"Origin", "Library"}, {"Destination", zoneWord(mv.To)}, {"ChangeType", v}, {"ChangeNum", k}, {"Mandatory", "True"},
	}}
	if tapped {
		n.set("Tapped", "True")
	}
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func printDig(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:"+st.Verb.String(), st.Src)
	top := st.Obj
	if top == nil || top.Kind != ir.RefTopOfLibrary || st.Optional || !zwhoYou(st) || (top.Of != nil && !isYou(top.Of)) {
		return nil, 0, bad
	}
	th := zsteps(st.Then)
	switch {
	case len(th) == 2 && th[0].Verb == ir.VerbMove && th[1].Verb == ir.VerbMove:
		n, err := digNode(p, st, th[0], th[1])
		if err != nil {
			return nil, 0, err
		}
		return n, 1, nil
	case len(th) == 1 && st.Verb == ir.VerbLook:
		m := th[0]
		if m.Verb != ir.VerbMove || m.Optional || m.Obj != nil || m.To != ir.ZoneLibrary || m.Word != "top-any-order" {
			return nil, 0, bad
		}
		k, err := p.amount(top.N)
		if err != nil {
			return nil, 0, err
		}
		n := &node{API: "RearrangeTopOfLibrary", Params: []kv{{"NumCards", k}}}
		if len(steps) > 1 && steps[1].Verb == ir.VerbShuffle && steps[1].Optional && steps[1].Who == nil {
			n.set("MayShuffle", "True")
			return n, 2, nil
		}
		return n, 1, nil
	}
	return nil, 0, bad
}

func digNode(p *pr, st, m1, m2 *ir.Step) (*node, error) {
	bad := unprintable("verb:"+st.Verb.String(), st.Src)
	o := m1.Obj
	if o == nil || o.Kind != ir.RefChoose || o.Of != nil || m2.Obj != nil || m2.Optional ||
		m1.To == ir.ZoneNone || m2.To == ir.ZoneNone || !zwhoYou(m1) || !zwhoYou(m2) {
		return nil, bad
	}
	for _, e := range []*ir.Enter{m1.Enter, m2.Enter} {
		if tapped, ok := zenter(e); tapped || !ok {
			return nil, unprintable("verb:dig-enter", st.Src)
		}
	}
	dn, err := p.amount(st.Obj.N)
	if err != nil {
		return nil, err
	}
	num := "Any"
	if o.N != nil {
		if num, err = p.amount(o.N); err != nil {
			return nil, err
		}
	}
	valid := "Card"
	if o.Filter != nil {
		if valid, err = p.valid(o.Filter); err != nil {
			return nil, err
		}
	}
	n := &node{API: "Dig", Params: []kv{
		{"DigNum", dn}, {"ChangeNum", num}, {"ChangeValid", valid},
		{"DestinationZone", zoneWord(m1.To)}, {"DestinationZone2", zoneWord(m2.To)},
	}}
	if m1.Optional {
		n.set("Optional", "True")
	}
	reveal := st.Verb == ir.VerbReveal
	switch m1.Word {
	case "":
	case "reveal":
		reveal = true
	case "top":
		if m1.To != ir.ZoneLibrary {
			return nil, unprintable("verb:dig-word", st.Src)
		}
		n.set("LibraryPosition", "0")
	default:
		return nil, unprintable("verb:dig-word", st.Src)
	}
	if reveal {
		n.set("Reveal", "True")
	}
	switch m2.Word {
	case "":
	case "bottom", "bottom-any-order":
		if m2.To != ir.ZoneLibrary {
			return nil, unprintable("verb:dig-word", st.Src)
		}
		n.set("LibraryPosition2", "-1")
	default:
		return nil, unprintable("verb:dig-word", st.Src)
	}
	return n, nil
}

func printRevealHand(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	n := &node{API: "RevealHand"}
	if st.Verb == ir.VerbLookAtHand {
		n.set("Look", "True")
	}
	if st.Obj == nil {
		return nil, 0, unprintable("verb:"+st.Verb.String(), st.Src)
	}
	if err := p.operand(n, "Defined", st.Obj); err != nil {
		return nil, 0, err
	}
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func printShuffle(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if st.Optional {
		return nil, 0, unprintable("verb:shuffle-optional", st.Src)
	}
	n := &node{API: "Shuffle"}
	if st.Who != nil {
		if err := p.operand(n, "Defined", st.Who); err != nil {
			return nil, 0, err
		}
	}
	return n, 1, nil
}
