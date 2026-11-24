package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() {
	registerStep(ir.VerbSearch, printSearch)
	registerStep(ir.VerbRevealUntil, printRevealUntil)
	registerStep(ir.VerbLook, printDig)
	registerStep(ir.VerbReveal, printDig)
	registerStep(ir.VerbLookAtHand, printRevealHand)
	registerStep(ir.VerbRevealHand, printRevealHand)
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
	if !st.Optional {
		n.set("Mandatory", "True")
	}
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
	if st.Verb == ir.VerbLook && top != nil && top.Kind == ir.RefTopOfLibrary && top.Of != nil && !isYou(top.Of) && len(steps) > 1 {
		return printLookThey(p, steps)
	}
	if st.Verb == ir.VerbReveal && st.Who != nil && !isYou(st.Who) && len(steps) > 1 && !st.Optional {
		return printRevealTopOf(p, steps)
	}
	if top == nil || top.Kind != ir.RefTopOfLibrary || st.Optional || !zwhoYou(st) || (top.Of != nil && !isYou(top.Of)) {
		return nil, 0, bad
	}
	th := zsteps(st.Then)
	switch {
	case len(th) == 0 && st.Verb == ir.VerbLook && len(steps) >= 3 && steps[1].Verb == ir.VerbPayCost:
		return printPeekReveal(p, steps)
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

// printPeekReveal prints "look at the top card, you may reveal it, if it is a
// <filter> card transform this" as PeekAndReveal, SetState and Cleanup.
func printPeekReveal(p *pr, steps []*ir.Step) (*node, int, error) {
	bad := unprintable("verb:look", steps[0].Src)
	look, pay, outer := steps[0], steps[1], steps[2]
	if look.Obj.N == nil || look.Obj.N.Kind != ir.VConst || !pay.Optional || pay.Who != nil || pay.Cost == nil || len(pay.Cost.Parts) != 1 || pay.Cost.HasMana ||
		pay.Cost.Parts[0].Kind != ir.CostReveal || pay.Cost.Parts[0].Obj == nil || pay.Cost.Parts[0].Obj.Kind != ir.RefTopOfLibrary ||
		outer.Verb != ir.VerbIf || outer.Cond == nil || outer.Cond.Kind != ir.CCostPaid || outer.Optional {
		return nil, 0, bad
	}
	o := zsteps(outer.Then)
	if len(o) != 1 || o[0].Verb != ir.VerbIf || o[0].Cond == nil || o[0].Cond.Kind != ir.CReceiptNonEmpty || o[0].Cond.Receipt != ir.RcRevealed || o[0].Cond.Filter == nil {
		return nil, 0, bad
	}
	tr := zsteps(o[0].Then)
	if len(tr) != 1 || tr[0].Verb != ir.VerbTransform || tr[0].Obj == nil || tr[0].Obj.Kind != ir.RefSelf || tr[0].Optional || tr[0].Who != nil {
		return nil, 0, bad
	}
	v, err := p.valid(o[0].Cond.Filter)
	if err != nil {
		return nil, 0, err
	}
	terms := strings.Split(v, ",")
	for i, t := range terms {
		if !strings.HasPrefix(t, "Card.") {
			terms[i] = "Card." + t
		}
	}
	k, err := p.amount(look.Obj.N)
	if err != nil {
		return nil, 0, err
	}
	n := &node{API: "PeekAndReveal", Params: []kv{{"PeekAmount", k}, {"RevealOptional", "True"}, {"RememberRevealed", "True"}}}
	n.Sub = &node{API: "SetState", Params: []kv{
		{"Defined", "Self"}, {"Mode", "Transform"}, {"ConditionDefined", "Remembered"},
		{"ConditionPresent", strings.Join(terms, ",")}, {"ConditionCompare", "EQ1"},
	}}
	n.Sub.Sub = &node{API: "Cleanup", Params: []kv{{"ClearRemembered", "True"}}}
	return n, 3, nil
}

// printRevealTopOf prints "a player reveals the top card of their library; if
// it is a <filter> card they put it into their hand" as one Dig.
func printRevealTopOf(p *pr, steps []*ir.Step) (*node, int, error) {
	st, ifs := steps[0], steps[1]
	bad := unprintable("verb:reveal", st.Src)
	top := st.Obj
	if top == nil || top.Kind != ir.RefTopOfLibrary || top.Of != nil || ifs.Verb != ir.VerbIf || ifs.Optional || ifs.Cond == nil ||
		ifs.Cond.Kind != ir.CReceiptNonEmpty || ifs.Cond.Receipt != ir.RcRevealed || ifs.Cond.Filter == nil {
		return nil, 0, bad
	}
	th := zsteps(ifs.Then)
	if len(th) != 1 || th[0].Verb != ir.VerbMove || th[0].Optional || th[0].Enter != nil || th[0].Word != "" || th[0].To != ir.ZoneHand ||
		!ir.Equal(th[0].Who, st.Who) || !ir.Equal(th[0].Obj, top) {
		return nil, 0, bad
	}
	v, err := p.valid(ifs.Cond.Filter)
	if err != nil {
		return nil, 0, err
	}
	k, err := p.amount(top.N)
	if err != nil {
		return nil, 0, err
	}
	n := &node{API: "Dig"}
	if st.Who.Kind == ir.RefDefendingPlayer && p.ev == ir.EvAttacks {
		n.set("Defined", "TriggeredDefendingPlayer")
	} else if err := p.operand(n, "Defined", st.Who); err != nil {
		return nil, 0, err
	}
	n.Params = append(n.Params, kv{"DigNum", k}, kv{"Reveal", "True"}, kv{"ChangeNum", "All"}, kv{"ChangeValid", v}, kv{"LibraryPosition2", "0"})
	return n, 2, nil
}

// printRevealUntil prints "reveal cards from the top until a <filter> card;
// put it into your hand and the rest on the bottom at random" as DigUntil.
func printRevealUntil(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:reveal-until", st.Src)
	th := zsteps(st.Then)
	if st.Obj != nil || !zwhoYou(st) || len(th) != 2 || st.Filter == nil {
		return nil, 0, bad
	}
	f, r := th[0], th[1]
	if f.Verb != ir.VerbMove || f.Optional || f.Who != nil || f.Enter != nil || f.Word != "" || f.To == ir.ZoneNone || f.Obj == nil || f.Obj.Kind != ir.RefReceipt || f.Obj.Receipt != ir.RcFound ||
		r.Verb != ir.VerbMove || r.Optional || r.Who != nil || r.Enter != nil || r.Obj != nil || r.To != ir.ZoneLibrary {
		return nil, 0, bad
	}
	v, err := p.valid(st.Filter)
	if err != nil {
		return nil, 0, err
	}
	n := &node{API: "DigUntil", Params: []kv{{"Valid", v}, {"FoundDestination", zoneWord(f.To)}, {"RevealedDestination", "Library"}}}
	switch r.Word {
	case "bottom":
		n.set("RevealedLibraryPosition", "-1")
	case "bottom-random":
		n.set("RevealedLibraryPosition", "-1")
		n.set("RevealRandomOrder", "True")
	default:
		return nil, 0, bad
	}
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

// printLookThey prints "look at the top card of a player's library; you may put
// it on the bottom" as one Dig that may move any of the one card.
func printLookThey(p *pr, steps []*ir.Step) (*node, int, error) {
	st, mv := steps[0], steps[1]
	bad := unprintable("verb:look", st.Src)
	if st.Optional || !zwhoYou(st) || len(zsteps(st.Then)) > 0 || mv.Verb != ir.VerbMove || !mv.Optional || mv.Who != nil || mv.Enter != nil ||
		mv.Word != "bottom" || mv.To != ir.ZoneLibrary || !ir.Equal(mv.Obj, st.Obj) {
		return nil, 0, bad
	}
	k, err := p.amount(st.Obj.N)
	if err != nil {
		return nil, 0, err
	}
	n := &node{API: "Dig"}
	if err := p.operand(n, "Defined", st.Obj.Of); err != nil {
		return nil, 0, err
	}
	n.Params = append(n.Params, kv{"DigNum", k}, kv{"ChangeNum", "Any"}, kv{"DestinationZone", "Library"}, kv{"LibraryPosition2", "0"})
	return n, 2, nil
}
