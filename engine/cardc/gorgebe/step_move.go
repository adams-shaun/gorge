package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

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

// filterZone is the one zone f selects from: its own, or the one its
// combinator arguments agree on.
func filterZone(f *ir.Filter) (ir.Zone, bool) {
	if f == nil {
		return ir.ZoneNone, false
	}
	if z, ok := singleZone(f); ok {
		return z, true
	}
	found, ok := ir.ZoneNone, false
	for _, a := range f.Args {
		z, k := filterZone(a)
		if !k {
			continue
		}
		if ok && z != found {
			return ir.ZoneNone, false
		}
		found, ok = z, true
	}
	return found, ok
}

// ownedByPlayer returns the player whose cards f selects when f is just
// "owned by that player" (a target slot, or a player filter naming one).
func ownedByPlayer(f *ir.Filter) *ir.Ref {
	if f == nil || f.Op != ir.FPred || f.Pred != ir.POwnedBy || f.Ref == nil {
		return nil
	}
	r := f.Ref
	if r.Kind == ir.RefAll && r.Filter != nil && r.Filter.Op == ir.FPred && r.Filter.Pred == ir.PPlayerIs && r.Filter.Ref != nil {
		r = r.Filter.Ref
	}
	if r.Kind != ir.RefTarget {
		return nil
	}
	return r
}

// dropAnyOwner returns f without its "owned by any player" terms, which
// restrict nothing; f itself when it has none.
func dropAnyOwner(f *ir.Filter) *ir.Filter {
	if f == nil || f.Op != ir.FAll {
		return f
	}
	var args []*ir.Filter
	changed := false
	for _, a := range f.Args {
		if a != nil && a.Op == ir.FPred && a.Pred == ir.POwnedBy && a.Ref != nil && a.Ref.Kind == ir.RefAll &&
			a.Ref.Filter != nil && a.Ref.Filter.Op == ir.FPred && a.Ref.Filter.Pred == ir.PAny {
			changed = true
			continue
		}
		if d := dropAnyOwner(a); d != a {
			a, changed = d, true
		}
		args = append(args, a)
	}
	if !changed {
		return f
	}
	if len(args) == 1 {
		return args[0]
	}
	c := *f
	c.Args = args
	return &c
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

// zenterOn applies a move's Enter to n: tapped, entering under your control
// (GainControl$ True, ChangeZoneAll only) and entering with counters.
func (p *pr) zenterOn(n *node, e *ir.Enter) bool {
	if e == nil {
		return true
	}
	if (e.Counter == "") != (e.N == nil) {
		return false
	}
	var types, kws []string
	for _, x := range e.Effects {
		switch x.Kind {
		case ir.EffAddSubtype, ir.EffAddCardType:
			for _, w := range x.Words {
				types = append(types, Respell(w))
			}
		case ir.EffAddAbility:
			k, ok := bareKeywords(x)
			if !ok {
				return false
			}
			kws = append(kws, k...)
		default:
			return false
		}
	}
	if e.Controller != nil {
		if !isYou(e.Controller) || n.API != "ChangeZoneAll" {
			return false
		}
		n.set("GainControl", "True")
	}
	if e.Tapped {
		n.set("Tapped", "True")
	}
	if e.Counter != "" {
		k, err := p.amount(e.N)
		if err != nil {
			return false
		}
		n.set("WithCountersType", e.Counter)
		n.set("WithCountersAmount", k)
	}
	if len(types)+len(kws) > 0 {
		body := "Mode$ Continuous | Affected$ Card.IsRemembered"
		if len(types) > 0 {
			body += " | AddType$ " + strings.Join(types, " & ")
		}
		if len(kws) > 0 {
			body += " | AddKeyword$ " + strings.Join(kws, " & ")
		}
		name := p.fresh("Animate")
		p.svars = append(p.svars, kv{name, body})
		n.set("StaticEffect", name)
	}
	return true
}

func printMove(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if o := st.Obj; o != nil && o.Kind == ir.RefTopOfLibrary && (st.Verb == ir.VerbExile || st.To == ir.ZoneExile) &&
		len(steps) > 1 && exileKept(steps[1]) {
		n, err := printExileKept(p, st)
		if err != nil {
			return nil, 0, err
		}
		return n, 1, nil
	}
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
	if to == ir.ZoneNone || obj == nil {
		return nil, bad
	}
	// Someone else acts only where the move names them: a choice of theirs
	// (DefinedPlayer$), or the top-or-bottom pick of a library's owner.
	who := st.Who
	if zwhoYou(st) {
		who = nil
	} else if !(obj.Kind == ir.RefChoose && obj.Of == nil) && st.Word != "top-or-bottom" && !(obj.Kind == ir.RefAll && st.Word == "shuffle") {
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
		from := st.From
		if from == ir.ZoneNone {
			from = p.abilityZone()
		}
		switch {
		case from != ir.ZoneNone && p.ev == ir.EvNone:
			// Forge's form of a source moving out of the zone its activated
			// ability works from: the zone as Origin$ and no Defined$.
			n = &node{API: "ChangeZone", Params: []kv{{"Origin", zoneWord(from)}}}
		case from != ir.ZoneNone:
			n = &node{API: "ChangeZone", Params: []kv{{"Defined", "Self"}, {"Origin", zoneWord(from)}}}
		default:
			n = &node{API: "ChangeZone", Params: []kv{{"Defined", "Self"}}}
		}
		n.set("Destination", dest)
	case ir.RefEventObject:
		origin := st.From
		if origin == ir.ZoneNone {
			switch p.ev {
			case ir.EvDies, ir.EvPutIntoGraveyard:
				origin = ir.ZoneGraveyard
			default:
				return nil, bad
			}
		}
		n = &node{API: "ChangeZone", Params: []kv{{"Origin", zoneWord(origin)}, {"Destination", dest}}}
		if p.evSelf && p.ev == ir.EvDies {
			// A card that dies is a new object in the graveyard; Self names the old one.
			n.set("Defined", "TriggeredNewCardLKICopy")
		} else if err := p.operand(n, "Defined", obj); err != nil {
			return nil, err
		}
	case ir.RefChoose, ir.RefAll:
		z, ok := filterZone(obj.Filter)
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
		owner := ownedByPlayer(obj.Filter)
		v, err := "Card", error(nil)
		if owner == nil {
			v, err = p.valid(dropAnyOwner(obj.Filter))
		}
		if err != nil {
			return nil, err
		}
		if obj.Kind == ir.RefAll && owner != nil {
			// Forge names the player alone: every card of theirs in that zone.
			n = &node{API: "ChangeZoneAll", Params: []kv{{"Origin", zoneWord(z)}, {"Destination", dest}}}
			if err := p.operand(n, "Defined", owner); err != nil {
				return nil, err
			}
			break
		}
		if obj.Kind == ir.RefAll {
			n = &node{API: "ChangeZoneAll", Params: []kv{{"ChangeType", v}, {"Origin", zoneWord(z)}, {"Destination", dest}}}
			if who != nil {
				d, err := p.defined(who)
				if err != nil {
					return nil, err
				}
				n.set("Defined", d)
			}
			break
		}
		k, err := p.amount(obj.N)
		if err != nil {
			return nil, err
		}
		n = &node{API: "ChangeZone", Params: []kv{{"Origin", zoneWord(z)}, {"Destination", dest}, {"ChangeType", v}, {"ChangeNum", k}, {"Mandatory", "True"}}}
		if who != nil {
			if err := p.operand(n, "Defined", who); err != nil {
				return nil, err
			}
			d, err := p.defined(who)
			if err != nil {
				return nil, err
			}
			n.set("DefinedPlayer", d)
			n.set("Chooser", d)
			who = nil
		}
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
		if obj.Kind == ir.RefChoose {
			n.set("LibraryPosition", "0")
		}
		n.set("Reorder", "True")
	case "shuffle":
		if obj.Kind != ir.RefAll || to != ir.ZoneLibrary {
			return nil, bad
		}
		n.set("Shuffle", "True")
	case "top-or-bottom":
		if who == nil || to != ir.ZoneLibrary {
			return nil, bad
		}
		d, err := p.defined(who)
		if err != nil {
			return nil, err
		}
		n.set("AlternativeDecider", d)
		n.set("DestinationAlternative", dest)
		n.set("LibraryPositionAlternative", "-1")
	default:
		return nil, unprintable("verb:move-word", st.Src)
	}
	if !p.zenterOn(n, st.Enter) {
		return nil, unprintable("verb:move-enter", st.Src)
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
