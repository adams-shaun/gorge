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
	// choose, an optional reveal of the found card, an empty do, then the
	// move and the shuffle in either order.
	if len(th) == 0 || th[0].Verb != ir.VerbChoose {
		return nil, 0, bad
	}
	ch := th[0]
	if ch.Optional || ch.Obj == nil || !chooseOf(ch.Obj) || !onlyZone(ch.Obj.Filter, ir.ZoneLibrary) {
		return nil, 0, bad
	}
	var mv *ir.Step
	shuffles := 0
	for _, s := range th[1:] {
		switch {
		case s.Verb == ir.VerbReveal && !s.Optional && s.Who == nil && s.Obj != nil && s.Obj.Kind == ir.RefReceipt && s.Obj.Receipt == ir.RcFound:
		case s.Verb == ir.VerbDo && s.Then == nil && !s.Optional && s.Who == nil:
		case s.Verb == ir.VerbShuffle && !s.Optional && s.Who == nil:
			shuffles++
		case s.Verb == ir.VerbMove && mv == nil && !s.Optional && s.Who == nil && s.Obj != nil && s.Obj.Kind == ir.RefReceipt && s.Obj.Receipt == ir.RcFound && s.To != ir.ZoneNone:
			mv = s
		default:
			return nil, 0, bad
		}
	}
	if mv == nil || shuffles != 1 {
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
	n := &node{API: "ChangeZone", Params: []kv{{"Origin", "Library"}, {"Destination", zoneWord(mv.To)}}}
	switch mv.Word {
	case "":
	case "top":
		if mv.To != ir.ZoneLibrary {
			return nil, 0, bad
		}
		n.set("LibraryPosition", "0")
	default:
		return nil, 0, bad
	}
	n.set("ChangeType", v)
	n.set("ChangeNum", k)
	if tapped {
		n.set("Tapped", "True")
	}
	if st.Who != nil && !isYou(st.Who) {
		if st.Optional {
			n.set("Optional", "True")
		}
		d, err := p.defined(st.Who)
		if err != nil {
			return nil, 0, err
		}
		n.set("DefinedPlayer", d)
	} else if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	if mv.To == ir.ZoneLibrary {
		n.set("ShuffleNonMandatory", "True")
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
	case "bottom-random":
		if m2.To != ir.ZoneLibrary {
			return nil, unprintable("verb:dig-word", st.Src)
		}
		n.set("LibraryPosition2", "-1")
		n.set("RestRandomOrder", "True")
	default:
		return nil, unprintable("verb:dig-word", st.Src)
	}
	return n, nil
}

func printRevealHand(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if th := zsteps(st.Then); len(th) > 0 {
		return revealHandThen(p, st, th)
	}
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

// revealHandThen prints a hand reveal followed by you choosing a card from it
// to discard (one Discard, Mode$ RevealYouChoose) or to exile (RevealHand,
// then a ChangeZone chosen by you).
func revealHandThen(p *pr, st *ir.Step, th []*ir.Step) (*node, int, error) {
	bad := unprintable("verb:"+st.Verb.String(), st.Src)
	if st.Verb != ir.VerbRevealHand || st.Optional || st.Obj != nil || st.Who == nil || len(th) != 1 {
		return nil, 0, bad
	}
	c := th[0]
	if (c.Verb != ir.VerbDiscard && c.Verb != ir.VerbExile) || c.Optional || c.Who != nil || !chooseOf(c.Obj) || !onlyZone(c.Obj.Filter, ir.ZoneHand) {
		return nil, 0, bad
	}
	v, err := p.valid(c.Obj.Filter)
	if err != nil {
		return nil, 0, err
	}
	k, err := p.amount(c.Obj.N)
	if err != nil {
		return nil, 0, err
	}
	n := &node{API: "RevealHand"}
	if c.Verb == ir.VerbDiscard {
		n = &node{API: "Discard"}
	}
	if err := p.operand(n, "Defined", st.Who); err != nil {
		return nil, 0, err
	}
	if c.Verb == ir.VerbDiscard {
		n.set("Mode", "RevealYouChoose")
		n.set("DiscardValid", v)
		n.set("NumCards", k)
		return n, 1, nil
	}
	d, err := p.defined(st.Who)
	if err != nil {
		return nil, 0, err
	}
	n.Sub = &node{API: "ChangeZone", Params: []kv{{"Origin", "Hand"}, {"Destination", "Exile"}, {"DefinedPlayer", d}, {"Chooser", "You"}, {"ChangeType", v}}}
	if k != "1" {
		n.Sub.set("ChangeNum", k)
	}
	return n, 1, nil
}
