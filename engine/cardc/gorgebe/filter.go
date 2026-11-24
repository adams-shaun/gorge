package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { validFn = printValid }

// part is one piece of a term: a type word (typ) or a qualifier.
type part struct {
	typ bool
	s   string
}

// term is one OR-alternative of a Valid list.
type term struct {
	subj  ir.Subject
	parts []part
}

const maxTerms = 8

func printValid(p *pr, f *ir.Filter) (string, error) {
	ts, err := normalise(p, f, ir.SubjNone)
	if err != nil {
		return "", err
	}
	out := make([]string, len(ts))
	for i, t := range ts {
		s, err := renderTerm(t, f.Src)
		if err != nil {
			return "", err
		}
		out[i] = s
	}
	if isAnyTarget(out) {
		return "Any", nil
	}
	return strings.Join(out, ","), nil
}

func isAnyTarget(terms []string) bool {
	if len(terms) != 4 {
		return false
	}
	seen := map[string]bool{}
	for _, t := range terms {
		seen[t] = true
	}
	return seen["Creature"] && seen["Planeswalker"] && seen["Battle"] && seen["Player"]
}

func renderTerm(t term, src string) (string, error) {
	base := ""
	var quals []string
	add := func(q string) {
		for _, x := range quals {
			if x == q {
				return
			}
		}
		quals = append(quals, q)
	}
	for _, x := range t.parts {
		if x.typ && base == "" {
			base = x.s
		} else {
			add(x.s)
		}
	}
	if base == "" {
		switch t.subj {
		case ir.SubjPermanent:
			base = "Permanent"
		case ir.SubjCard, ir.SubjSpell, ir.SubjObject:
			base = "Card"
		case ir.SubjPlayer:
			base = "Player"
		default:
			return "", unprintable("filter:no-subject", src)
		}
	}
	if base == "Player" && len(quals) == 1 && (quals[0] == "You" || quals[0] == "Opponent") {
		return quals[0], nil
	}
	if len(quals) == 0 {
		return base, nil
	}
	return base + "." + strings.Join(quals, "+"), nil
}

func normalise(p *pr, f *ir.Filter, inh ir.Subject) ([]term, error) {
	subj := inh
	if f.Subject != ir.SubjNone {
		subj = f.Subject
	}
	switch f.Op {
	case ir.FAny:
		if len(f.Args) == 0 {
			return nil, unprintable("filter:empty", f.Src)
		}
		var out []term
		for _, a := range f.Args {
			ts, err := normalise(p, a, subj)
			if err != nil {
				return nil, err
			}
			out = append(out, ts...)
		}
		return out, nil
	case ir.FAll:
		if len(f.Args) == 0 {
			return nil, unprintable("filter:empty", f.Src)
		}
		out := []term{{subj: subj}}
		for _, a := range f.Args {
			ts, err := normalise(p, a, subj)
			if err != nil {
				return nil, err
			}
			var next []term
			for _, x := range out {
				for _, y := range ts {
					m := term{subj: x.subj}
					if m.subj == ir.SubjNone {
						m.subj = y.subj
					}
					m.parts = append(append([]part{}, x.parts...), y.parts...)
					next = append(next, m)
				}
			}
			if len(next) > maxTerms {
				return nil, unprintable("filter:too-wide", f.Src)
			}
			out = next
		}
		if f.Subject != ir.SubjNone {
			for i := range out {
				out[i].subj = f.Subject
			}
		}
		return out, nil
	case ir.FNot:
		if len(f.Args) != 1 || f.Args[0] == nil || f.Args[0].Op != ir.FPred {
			return nil, unprintable("filter:not", f.Src)
		}
		pt, err := predPart(p, f.Args[0], true)
		if err != nil {
			return nil, err
		}
		s := subj
		if f.Args[0].Subject != ir.SubjNone && f.Subject == ir.SubjNone {
			s = f.Args[0].Subject
		}
		return []term{{subj: s, parts: pt}}, nil
	case ir.FPred:
		pt, err := predPart(p, f, false)
		if err != nil {
			return nil, err
		}
		return []term{{subj: subj, parts: pt}}, nil
	}
	return nil, unprintable("filter:"+f.Op.String(), f.Src)
}

var colourWords = map[ir.ColorSet]string{
	ir.ColorW: "White", ir.ColorU: "Blue", ir.ColorB: "Black", ir.ColorR: "Red", ir.ColorG: "Green",
}

// predPart is the parts one predicate contributes; neg is its FNot form.
func predPart(p *pr, f *ir.Filter, neg bool) ([]part, error) {
	name := f.Pred.String()
	bad := func() ([]part, error) { return nil, unprintable("pred:"+name, f.Src) }
	badNot := func() ([]part, error) { return nil, unprintable("pred:"+name+"-not", f.Src) }
	// pos returns the positive qualifier, or the negated one when neg is set.
	pick := func(pos, ng string) ([]part, error) {
		if neg {
			return []part{{false, ng}}, nil
		}
		return []part{{false, pos}}, nil
	}
	typ := func(w string) ([]part, error) {
		if neg {
			return []part{{false, "non" + w}}, nil
		}
		return []part{{true, w}}, nil
	}
	cmpN := func() (string, string, error) {
		if f.Cmp == ir.CmpNone || f.Value == nil {
			return "", "", unprintable("pred:"+name, f.Src)
		}
		n, err := p.amount(f.Value)
		if err != nil {
			return "", "", err
		}
		return strings.ToUpper(f.Cmp.String()), n, nil
	}
	switch f.Pred {
	case ir.PAny:
		if neg {
			return badNot()
		}
		return nil, nil
	case ir.PCardType, ir.PSubtype:
		return typ(Respell(f.Word))
	case ir.PSupertype:
		if neg {
			return []part{{false, "non" + Respell(f.Word)}}, nil
		}
		return []part{{false, Respell(f.Word)}}, nil
	case ir.PPermanentCard:
		if neg {
			return badNot()
		}
		return typ("Permanent")
	case ir.PColor:
		w, ok := colourWords[f.Colors]
		if !ok {
			return nil, unprintable("pred:color-multi", f.Src)
		}
		return pick(w, "non"+w)
	case ir.PColorless:
		return pick("Colorless", "nonColorless")
	case ir.PMulticolored:
		if neg {
			return badNot()
		}
		return pick("MultiColor", "")
	case ir.PManaValue, ir.PPower, ir.PToughness:
		if neg {
			return badNot()
		}
		c, n, err := cmpN()
		if err != nil {
			return nil, err
		}
		key := map[ir.Pred]string{ir.PManaValue: "cmc", ir.PPower: "power", ir.PToughness: "toughness"}[f.Pred]
		return []part{{false, key + c + n}}, nil
	case ir.PToken:
		return pick("token", "!token")
	case ir.PTapped:
		return pick("tapped", "untapped")
	case ir.PAttacking, ir.PBlocking:
		if neg {
			return badNot()
		}
		return []part{{false, name}}, nil
	case ir.PHasKeyword:
		return pick("with"+f.Word, "without"+f.Word)
	case ir.PCounters:
		if neg {
			return badNot()
		}
		c, n, err := cmpN()
		if err != nil {
			return nil, err
		}
		return []part{{false, "counters_" + c + n + "_" + f.Word}}, nil
	case ir.PName:
		return pick("named"+f.Word, "!named"+f.Word)
	case ir.PControlledBy, ir.POwnedBy:
		if neg {
			return badNot()
		}
		w, ok := playerWord(p, f.Ref)
		suffix := "Ctrl"
		if f.Pred == ir.POwnedBy {
			suffix = "Own"
		}
		switch {
		case !ok:
			return bad()
		case w == "Active" && f.Pred == ir.POwnedBy:
			return bad()
		case w == "Active":
			w = "ActivePlayer"
		}
		return []part{{false, w + suffix}}, nil
	case ir.PIs:
		if f.Ref != nil && f.Ref.Kind == ir.RefAttachedTo && f.Ref.Of != nil && f.Ref.Of.Kind == ir.RefSelf && p.attachWord() != "" {
			// the creature (permanent) this equipment or aura is attached to
			w := map[string]string{"Equipped": "EquippedBy", "Enchanted": "EnchantedBy"}[p.attachWord()]
			if neg {
				return []part{{false, "!" + w}}, nil
			}
			if w == "EquippedBy" {
				return []part{{true, "Creature"}, {false, w}}, nil
			}
			return []part{{false, w}}, nil
		}
		if f.Ref != nil && (f.Ref.Kind == ir.RefSelf || (f.Ref.Kind == ir.RefEventObject && p.evSelf)) {
			return pick("Self", "Other")
		}
		return bad()
	case ir.PPlayerIs:
		w, ok := playerWord(p, f.Ref)
		// Forge spells neither Opp nor the targeted player as a Player qualifier.
		if !ok || w == "Opp" || w == "TargetedPlayer" {
			return bad()
		}
		if neg {
			return []part{{false, "!" + w}}, nil
		}
		return []part{{false, w}}, nil
	case ir.POpponentOf:
		if neg {
			return badNot()
		}
		if w, ok := playerWord(p, f.Ref); ok && w == "You" {
			return []part{{false, "Opponent"}}, nil
		}
		return bad()
	}
	return bad()
}
