package forge

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// refSubj is the set of subjects a reference can denote.
type refSubj uint8

const (
	rsObject refSubj = 1 << iota
	rsPlayer
	rsEither = rsObject | rsPlayer
)

// LowerRef reads a Forge reference (the value of a RefKeys parameter) into an
// ir.Ref that denotes want, ir.SubjObject or ir.SubjPlayer, or refuses it
// naming the first token it cannot read. It is a pure function of text and
// want: no card, no SVar table, no game. A text that can only denote the other
// subject is refused as wrong-subject.
//
// Grammar: text = part {"&" part}, spaces around a part ignored; two or more parts are a RefUnion. A part
// is, in order: "Valid <filter>" or "ValidAll <filter>" (RefAll, the latter
// with AllZones when it selects objects); a player
// filter with base Player or Opponent (RefAll); an atom word; an atom word
// with a projection suffix (Controller, Owner); or "<A>And<B>" with A and B
// atoms (RefUnion).
func LowerRef(text string, want ir.Subject) (*ir.Ref, *Refusal) {
	var mask refSubj
	switch want {
	case ir.SubjObject:
		mask = rsObject
	case ir.SubjPlayer:
		mask = rsPlayer
	default:
		return nil, refuse(RefuseWrongSubject, want.String())
	}
	text = strings.TrimSpace(text)
	parts := strings.Split(text, "&")
	refs := make([]*ir.Ref, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		r, subj, word, ref := lowerRefPart(p)
		if ref != nil {
			return nil, ref
		}
		if subj&mask == 0 {
			return nil, refuse(RefuseWrongSubject, word)
		}
		refs = append(refs, r)
	}
	if len(refs) == 1 {
		return refs[0], nil
	}
	root := &ir.Ref{Kind: ir.RefUnion, Union: refs}
	setRefSrc(root, text)
	return root, nil
}

// lowerRefPart reads one part, returning the subjects it can denote and the
// word a wrong-subject refusal names.
func lowerRefPart(p string) (*ir.Ref, refSubj, string, *Refusal) {
	if p == "" {
		return nil, 0, "", refuse(RefuseEmpty, "")
	}
	for _, head := range []string{"Valid", "ValidAll"} {
		if rest, ok := strings.CutPrefix(p, head+" "); ok {
			f, r := LowerFilter(rest)
			if r == nil {
				r = readPlayerSource(f)
			}
			if r != nil {
				return nil, 0, "", refuse(RefuseValidForm, r.Kind.String()+":"+dropName(r.Atom))
			}
			subj := filterSubj(f)
			if subj == 0 {
				return nil, 0, "", refuse(RefuseWrongSubject, head)
			}
			all := refAll(f, p)
			// Players are in no zone, so the two heads mean the same for them.
			all.AllZones = head == "ValidAll" && subj == rsObject
			return all, subj, head, nil
		}
	}
	base := baseOf(p)
	if base == "Player" || base == "Opponent" {
		f, r := LowerFilter(p)
		if r == nil {
			r = readPlayerSource(f)
		}
		if r != nil {
			return nil, 0, "", &Refusal{Kind: r.Kind, Atom: dropName(r.Atom)}
		}
		if filterSubj(f) != rsPlayer {
			return nil, 0, "", refuse(RefuseWrongSubject, base)
		}
		return refAll(f, p), rsPlayer, base, nil
	}
	if first, _, spaced := strings.Cut(p, " "); spaced || base != p {
		if b := baseOf(first); b != first {
			first = b + "."
		}
		return nil, 0, "", refuse(RefuseUnknownReferent, first)
	}
	r, subj, ref := lowerRefWord(p)
	return r, subj, p, ref
}

// dropName cuts an inner filter refusal's atom after its first "named", so a
// refusal names the predicate, never the card name it carries.
func dropName(atom string) string {
	if i := strings.Index(atom, "named"); i >= 0 {
		return atom[:i+len("named")]
	}
	return atom
}

// refAll is every object or player matching f.
func refAll(f *ir.Filter, src string) *ir.Ref {
	return &ir.Ref{Kind: ir.RefAll, Filter: f, Src: src}
}

// readPlayerSource rewrites, in place, the source-object words LowerFilter
// reads on a player subject as "is (not) the source": no player is the
// source, so "Other" and "StrictlyOther" (under their FNot) become "is not
// you", as Forge's player property reads them, and "Self" and "StrictlySelf"
// are refused. LowerFilter's own output is unchanged; only references read
// the player form.
func readPlayerSource(f *ir.Filter) *Refusal {
	if f.Subject == ir.SubjPlayer && f.Op == ir.FPred && f.Pred == ir.PIs && f.Ref != nil && f.Ref.Kind == ir.RefSelf {
		switch strings.TrimPrefix(f.Src, "!") {
		case "Other", "StrictlyOther":
			f.Pred, f.Ref = ir.PPlayerIs, refYou()
		default:
			return refuse(RefuseUnknownPredicate, f.Src) // "!" kept: the token as written
		}
	}
	for _, a := range f.Args {
		if r := readPlayerSource(a); r != nil {
			return r
		}
	}
	return nil
}

// filterSubj is the subject a filter's matches have: 0 for a root whose
// alternatives mix players and objects (anyRoot calls such a root an object),
// or an FAll whose base does, which denotes neither subject alone.
func filterSubj(f *ir.Filter) refSubj {
	var s refSubj
	if f.Op == ir.FAll && len(f.Args) > 0 && filterSubj(f.Args[0]) == 0 {
		return 0
	}
	if f.Op == ir.FAny {
		for _, a := range f.Args {
			s |= filterSubj(a)
		}
		if s == rsEither {
			return 0
		}
	}
	if f.Subject == ir.SubjPlayer {
		return rsPlayer
	}
	return rsObject
}

// setRefSrc sets src on every node of r that has none.
func setRefSrc(r *ir.Ref, src string) {
	if r == nil {
		return
	}
	if r.Src == "" {
		r.Src = src
	}
	if r.N != nil && r.N.Src == "" {
		r.N.Src = src
	}
	setRefSrc(r.Of, src)
	for _, u := range r.Union {
		setRefSrc(u, src)
	}
}
