package forge

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// LowerFilter reads a Forge object filter into an ir.Filter, or refuses it
// naming the first token it cannot read. It is a pure function of text.
//
// Grammar: spec = alt {"," alt}; alt = base ["." pred {"+" pred}]; a pred may
// start with "!". A whole spec of exactly "Self" or "Other" reads as
// "Card.Self" / "Card.Other". A comma part whose base is not readable,
// following an alternative whose last predicate starts with "named", is the
// rest of that name.
func LowerFilter(text string) (*ir.Filter, *Refusal) {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return nil, refuse(RefuseEmpty, "")
	case strings.HasPrefix(text, "EACH "):
		return nil, refuse(RefuseEachForm, "EACH")
	case text == "Self" || text == "Other":
		text = "Card." + text
	}
	alts := splitAlts(text)
	nodes := make([]*ir.Filter, 0, len(alts))
	for _, a := range alts {
		n, r := lowerAlt(a)
		if r != nil {
			return nil, r
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 1 {
		return nodes[0], nil
	}
	return anyRoot(nodes), nil
}

// anyRoot joins alternatives: the root takes their Subject and scope Zones
// when they agree, else SubjObject and no Zones.
func anyRoot(nodes []*ir.Filter) *ir.Filter {
	root := &ir.Filter{Op: ir.FAny, Args: nodes, Subject: nodes[0].Subject, Zones: nodes[0].Zones}
	for _, n := range nodes[1:] {
		if n.Subject != root.Subject {
			root.Subject = ir.SubjObject
		}
		if n.Zones != root.Zones {
			root.Zones = 0
		}
	}
	return root
}

// splitAlts splits text at commas, rejoining a part that continues a name.
func splitAlts(text string) []string {
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if n := len(out); n > 0 && endsInName(out[n-1]) {
			if _, r := lowerBase(strings.TrimSpace(baseOf(p))); r != nil {
				out[n-1] += "," + p
				continue
			}
		}
		out = append(out, p)
	}
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

// baseOf is an alternative's text before its first '.'.
func baseOf(alt string) string {
	if i := strings.IndexByte(alt, '.'); i >= 0 {
		return alt[:i]
	}
	return alt
}

// splitPreds splits an alternative's predicate text at '+'.
func splitPreds(alt string) []string {
	i := strings.IndexByte(alt, '.')
	if i < 0 {
		return nil
	}
	return strings.Split(alt[i+1:], "+")
}

// endsInName reports whether alt's last predicate is a named<Name>.
func endsInName(alt string) bool {
	ps := splitPreds(alt)
	if len(ps) == 0 {
		return false
	}
	last := strings.TrimPrefix(strings.TrimSpace(ps[len(ps)-1]), "!")
	return strings.HasPrefix(last, "named")
}

// altScope is what an alternative's nodes share.
type altScope struct {
	subj  ir.Subject
	zones ir.ZoneSet
}

// lowerAlt reads one alternative: its base node alone, or FAll of the base
// node and one node per predicate.
func lowerAlt(alt string) (*ir.Filter, *Refusal) {
	base, r := lowerBase(baseOf(alt))
	if r != nil {
		return nil, r
	}
	preds := splitPreds(alt)
	if preds == nil {
		return base, nil
	}
	sc := altScope{subj: base.Subject, zones: base.Zones}
	args := []*ir.Filter{base}
	for _, p := range preds {
		n, zones, r := lowerSignedPred(p, sc.subj)
		if r != nil {
			return nil, r
		}
		if zones != 0 {
			if sc.zones != 0 && sc.zones != zones {
				return nil, refuse(RefuseUnknownPredicate, p)
			}
			sc.zones = zones
		}
		if n != nil {
			args = append(args, n)
		}
	}
	var root *ir.Filter
	if len(args) == 1 {
		root = base
	} else {
		root = &ir.Filter{Op: ir.FAll, Args: args}
	}
	root.Subject, root.Zones = sc.subj, sc.zones
	return root, nil
}

// lowerSignedPred reads a predicate token with its optional '!'. A zone
// predicate returns no node and the zones it scopes the alternative to.
func lowerSignedPred(tok string, subj ir.Subject) (*ir.Filter, ir.ZoneSet, *Refusal) {
	body, neg := strings.CutPrefix(tok, "!")
	if z, ok, r := lowerZonePred(body, tok, neg); ok || r != nil {
		return nil, z, r
	}
	n, r := lowerPred(body, subj)
	if r != nil {
		r.Atom = foldDigits(tok)
		return nil, 0, r
	}
	setSrc(n, tok)
	if neg {
		n = not(subj, n)
	}
	return n, 0, nil
}

// setSrc records the token on every predicate node of n.
func setSrc(n *ir.Filter, tok string) {
	if n.Op == ir.FPred {
		n.Src = tok
	}
	for _, a := range n.Args {
		setSrc(a, tok)
	}
}

func pred(subj ir.Subject, p ir.Pred) *ir.Filter {
	return &ir.Filter{Subject: subj, Op: ir.FPred, Pred: p}
}

func not(subj ir.Subject, n *ir.Filter) *ir.Filter {
	return &ir.Filter{Subject: subj, Op: ir.FNot, Args: []*ir.Filter{n}}
}

func anyOf(subj ir.Subject, args ...*ir.Filter) *ir.Filter {
	return &ir.Filter{Subject: subj, Op: ir.FAny, Args: args}
}
