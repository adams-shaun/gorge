package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() {
	registerStep(ir.VerbMove, printMove)
	registerStep(ir.VerbExile, printMove)
}

// zoptional puts st's optionality on n: OptionalDecider$ You for a nil Who,
// else it is unprintable.
func zoptional(n *node, st *ir.Step) error {
	if !st.Optional {
		return nil
	}
	if st.Who != nil {
		return unprintable("optional:"+st.Verb.String(), st.Src)
	}
	n.set("OptionalDecider", "You")
	return nil
}

// zwhoYou reports whether st acts as you: no Who, or you.
func zwhoYou(st *ir.Step) bool { return st.Who == nil || isYou(st.Who) }

// zenter applies a move's Enter: tapped reports Tapped$ True. Anything but
// an under-owner or tapped entry is refused.
func zenter(e *ir.Enter) (tapped, ok bool) {
	if e == nil {
		return false, true
	}
	if e.Controller != nil || e.Counter != "" || len(e.Effects) > 0 {
		return false, false
	}
	return e.Tapped, true
}

func printMove(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	n, err := moveNode(p, st)
	if err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func moveNode(p *pr, st *ir.Step) (*node, error) {
	bad := unprintable("verb:move", st.Src)
	to := st.To
	if st.Verb == ir.VerbExile {
		to = ir.ZoneExile
	}
	obj := slotRef(st.Obj)
	if to == ir.ZoneNone || obj == nil || !zwhoYou(st) {
		return nil, bad
	}
	dest := zoneWord(to)
	var n *node
	switch obj.Kind {
	case ir.RefTarget:
		origin := ir.ZoneBattlefield
		if st.From != ir.ZoneNone {
			origin = st.From
		} else if f := p.frame(); f != nil && int(obj.Slot) < len(f.targets) && f.targets[obj.Slot] != nil {
			if z, ok := singleZone(f.targets[obj.Slot].Filter); ok {
				origin = z
			}
		}
		n = &node{API: "ChangeZone", Params: []kv{{"Origin", zoneWord(origin)}, {"Destination", dest}}}
		if err := p.operand(n, "Defined", obj); err != nil {
			return nil, err
		}
	case ir.RefSelf:
		n = &node{API: "ChangeZone", Params: []kv{{"Defined", "Self"}}}
		if st.From != ir.ZoneNone {
			n.set("Origin", zoneWord(st.From))
		}
		n.set("Destination", dest)
	case ir.RefChoose, ir.RefAll:
		z, ok := singleZone(obj.Filter)
		if !ok || (obj.Kind == ir.RefChoose && obj.Of != nil) {
			return nil, bad
		}
		if slot, ok := ownedByTargetPlayer(obj.Filter); ok && obj.Kind == ir.RefAll {
			// "target player's graveyard": the player is the target, every card moves
			n = &node{API: "ChangeZoneAll"}
			if err := p.operand(n, "Defined", &ir.Ref{Kind: ir.RefTarget, Slot: slot, Src: obj.Src}); err != nil {
				return nil, err
			}
			n.Params = append(n.Params, kv{"Origin", zoneWord(z)}, kv{"Destination", dest}, kv{"ChangeType", "Card"})
			break
		}
		v, err := p.valid(obj.Filter)
		if err != nil {
			return nil, err
		}
		if obj.Kind == ir.RefAll {
			n = &node{API: "ChangeZoneAll", Params: []kv{{"ChangeType", v}, {"Origin", zoneWord(z)}, {"Destination", dest}}}
			break
		}
		k, err := p.amount(obj.N)
		if err != nil {
			return nil, err
		}
		n = &node{API: "ChangeZone", Params: []kv{{"Origin", zoneWord(z)}, {"Destination", dest}, {"ChangeType", v}, {"ChangeNum", k}, {"Mandatory", "True"}}}
	case ir.RefTopOfLibrary:
		if to != ir.ZoneExile {
			return nil, bad
		}
		k, err := p.amount(obj.N)
		if err != nil {
			return nil, err
		}
		n = &node{API: "Mill", Params: []kv{{"NumCards", k}, {"Destination", "Exile"}}}
		who := obj.Of
		if who == nil {
			who = st.Who
		}
		if who != nil {
			if err := p.operand(n, "Defined", who); err != nil {
				return nil, err
			}
		}
	default:
		return nil, bad
	}
	switch st.Word {
	case "":
	case "top":
		n.set("LibraryPosition", "0")
	case "bottom", "bottom-any-order":
		n.set("LibraryPosition", "-1")
	case "top-any-order":
		n.set("Reorder", "True")
	default:
		return nil, unprintable("verb:move-word", st.Src)
	}
	tapped, ok := zenter(st.Enter)
	if !ok {
		return nil, unprintable("verb:move-enter", st.Src)
	}
	if tapped {
		n.set("Tapped", "True")
	}
	if err := zoptional(n, st); err != nil {
		return nil, err
	}
	return n, nil
}

// ownedByTargetPlayer reports whether f is exactly "owned by the player target
// slot k chose", and k.
func ownedByTargetPlayer(f *ir.Filter) (uint8, bool) {
	if f == nil || f.Op != ir.FPred || f.Pred != ir.POwnedBy || f.Ref == nil || f.Ref.Kind != ir.RefAll || f.Ref.Filter == nil {
		return 0, false
	}
	g := f.Ref.Filter
	if g.Op != ir.FPred || g.Pred != ir.PPlayerIs || g.Ref == nil || g.Ref.Kind != ir.RefTarget {
		return 0, false
	}
	return g.Ref.Slot, true
}
