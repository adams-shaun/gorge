package state

import "github.com/adams-shaun/gorge/cards"

// This file owns the ONE flat view of a permanent's rules text that spans the
// top face and every card merged beneath it (CR 702.140d: a mutated permanent
// "has all abilities of the cards beneath it"). Before it, every reader in
// rules/ read o.Face() -- always the TOP card of a pile -- so an under-card's
// activated abilities, mana abilities and statics were invisible.
//
// The ordering is fixed and load-bearing: the top face comes first, then
// MergedCards in pile order (index 0 is the first under-card). A top-face
// ability therefore keeps exactly its old flat index, which is why
// events.Apply's AbilityPush decode and the wire's Option.Ability stay
// byte-compatible for every non-mutated permanent. A merged ability's flat
// index is one past the last top-face ability plus its own position in the
// merged list (never a bare top-face index, so the activation-limit census
// can tell the two apart by construction).

// PileFace is one face of a mutated pile together with its position in the
// pile: Merged is 0 for the top face and i+1 for the i-th under-card. Face is
// resolved through Object.Face() for the top (so a CR 613.1a copy effect is
// honoured) and through MergedFaceAt for the under-cards (which are their own
// cards and can never carry the top object's copy).
type PileFace struct {
	Face   *cards.Face
	Merged int
}

// PileFaces returns the pile's faces top-first, then each under-card in pile
// order, skipping any face that cannot be resolved. It allocates only for a
// mutated pile (a plain permanent yields a one-element slice); callers on a
// hot path that must stay allocation-free use PileFaceCount/PileFaceAt.
func (o *Object) PileFaces() []PileFace {
	out := make([]PileFace, 0, 1+len(o.MergedCards))
	if f := o.Face(); f != nil {
		out = append(out, PileFace{Face: f})
	}
	for i := range o.MergedCards {
		if f := o.MergedFaceAt(i); f != nil {
			out = append(out, PileFace{Face: f, Merged: i + 1})
		}
	}
	return out
}

// PileFaceAt returns the face at pile position i without allocating: i == 0
// is the top face, i in [1, len(MergedCards)] is the (i-1)-th under-card. A
// position that does not resolve (nil face) reports ok=false; a caller that
// walks positions must therefore not treat a gap as end-of-pile -- use
// PileFaceCount as the bound.
func (o *Object) PileFaceAt(i int) (PileFace, bool) {
	if i < 0 {
		return PileFace{}, false
	}
	if i == 0 {
		if f := o.Face(); f != nil {
			return PileFace{Face: f}, true
		}
		return PileFace{}, false
	}
	f := o.MergedFaceAt(i - 1)
	if f == nil {
		return PileFace{}, false
	}
	return PileFace{Face: f, Merged: i}, true
}

// PileFaceCount is the number of face SLOTS in the pile (1 + len(MergedCards)),
// independent of whether each resolves -- the correct walk bound for
// PileFaceAt.
func (o *Object) PileFaceCount() int { return 1 + len(o.MergedCards) }

// PileAbility is one activated ability of a pile together with the merged
// ordinal of the face that carries it (0 = top).
type PileAbility struct {
	SA     *cards.SA
	Merged int
}

// PileAbilities returns every face's activated ability list flattened
// top-first: the top face's Abilities in order, then each under-card's. It is
// the ONE enumeration the offer loop, the activation flow, the event fold and
// the activation-limit census share, so a flat index means the same ability
// everywhere.
func (o *Object) PileAbilities() []PileAbility {
	n := 0
	if f := o.Face(); f != nil {
		n += len(f.Abilities)
	}
	for i := range o.MergedCards {
		if f := o.MergedFaceAt(i); f != nil {
			n += len(f.Abilities)
		}
	}
	out := make([]PileAbility, 0, n)
	if f := o.Face(); f != nil {
		for _, ab := range f.Abilities {
			out = append(out, PileAbility{SA: ab})
		}
	}
	for i := range o.MergedCards {
		if f := o.MergedFaceAt(i); f != nil {
			for _, ab := range f.Abilities {
				out = append(out, PileAbility{SA: ab, Merged: i + 1})
			}
		}
	}
	return out
}

// PileAbilityAt resolves one flat pile-ability index without allocating. ok is
// false for a negative index or one past the end of the pile's abilities.
func (o *Object) PileAbilityAt(i int) (PileAbility, bool) {
	if i < 0 {
		return PileAbility{}, false
	}
	if f := o.Face(); f != nil {
		if i < len(f.Abilities) {
			return PileAbility{SA: f.Abilities[i]}, true
		}
		i -= len(f.Abilities)
	}
	for m := range o.MergedCards {
		f := o.MergedFaceAt(m)
		if f == nil {
			continue
		}
		if i < len(f.Abilities) {
			return PileAbility{SA: f.Abilities[i], Merged: m + 1}, true
		}
		i -= len(f.Abilities)
	}
	return PileAbility{}, false
}

// PileAbilityIndex is the inverse of PileAbilityAt for a face-local ability:
// it maps (merged ordinal, face-local index) to the flat pile index. ok is
// false when the face or index is out of range.
func (o *Object) PileAbilityIndex(merged, local int) (int, bool) {
	if merged < 0 || local < 0 {
		return 0, false
	}
	base := 0
	if f := o.Face(); f != nil {
		if merged == 0 {
			if local < len(f.Abilities) {
				return local, true
			}
			return 0, false
		}
		base = len(f.Abilities)
	} else if merged == 0 {
		return 0, false
	}
	for m := range o.MergedCards {
		f := o.MergedFaceAt(m)
		if f == nil {
			continue
		}
		if m+1 == merged {
			if local < len(f.Abilities) {
				return base + local, true
			}
			return 0, false
		}
		base += len(f.Abilities)
	}
	return 0, false
}

// PileSVars returns the SVar table of the face at the given merged ordinal
// (0 = top). A face that cannot be resolved yields nil, the same nil table a
// Face()-less object gives everywhere else.
func (o *Object) PileSVars(merged int) map[string]string {
	if f := o.PileFaceFor(merged); f != nil {
		return f.SVars
	}
	return nil
}

// PileFaceFor returns the face at the given merged ordinal (0 = top), or nil.
func (o *Object) PileFaceFor(merged int) *cards.Face {
	if merged <= 0 {
		return o.Face()
	}
	return o.MergedFaceAt(merged - 1)
}

// PileStatic is one static ability of a pile together with the merged ordinal
// of the face that carries it. The static collectors use it so a static's own
// SVar table travels with it (staticView carries the table for specCtx /
// SVar resolution).
type PileStatic struct {
	Static cards.Static
	Merged int
	Face   *cards.Face
}

// PileStatics returns every face's static list flattened top-first, each with
// the face that carries it.
func (o *Object) PileStatics() []PileStatic {
	n := 0
	if f := o.Face(); f != nil {
		n += len(f.Statics)
	}
	for i := range o.MergedCards {
		if f := o.MergedFaceAt(i); f != nil {
			n += len(f.Statics)
		}
	}
	out := make([]PileStatic, 0, n)
	if f := o.Face(); f != nil {
		for _, st := range f.Statics {
			out = append(out, PileStatic{Static: st, Face: f})
		}
	}
	for i := range o.MergedCards {
		f := o.MergedFaceAt(i)
		if f == nil {
			continue
		}
		for _, st := range f.Statics {
			out = append(out, PileStatic{Static: st, Merged: i + 1, Face: f})
		}
	}
	return out
}
