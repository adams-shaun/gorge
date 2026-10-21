package cards

import (
	"sort"
	"sync"
)

// Universe is the whole-corpus card-name universe a game's NameCard asks,
// chosen-name copies and land-type vocabulary range over
// (rules.Config.NameUniverse, state.Game.NameUniverse). It answers every
// whole-universe NAME query from pointer-free rows when it is backed by an
// imaged registry (Registry.Universe), so a game never materializes the
// corpus to list names; Card(i) materializes one card for the queries that
// need an object (NameCard's ValidCards$ matcher walk).
//
// UniverseOf adapts a plain card slice (test-built universes) and answers
// exactly the same. A Universe is immutable and shared by every game an
// embedder starts from it; a nil *Universe is the empty universe.
type Universe struct {
	cs []*Card
	lz *lazyCards

	namesOnce sync.Once
	names     []string

	normOnce sync.Once
	normKeys []string // sorted NormalizeName(Name(i)) keys
	normOrds []uint32 // parallel: FIRST ordinal with that key

	landOnce sync.Once
	land     []string
}

// UniverseOf is the universe over cs, in order. A nil or empty cs is a
// non-nil universe of length 0.
func UniverseOf(cs []*Card) *Universe { return &Universe{cs: cs} }

// Universe returns r's name universe: every card, in registry order. It is
// the same pointer on every call, so universe-keyed memos are shared.
func (r *Registry) Universe() *Universe {
	r.univOnce.Do(func() {
		if r.lazy != nil {
			r.univ = &Universe{lz: r.lazy}
			return
		}
		r.univ = UniverseOf(r.cards)
	})
	return r.univ
}

// Len is the number of cards in the universe.
func (u *Universe) Len() int {
	switch {
	case u == nil:
		return 0
	case u.lz != nil:
		return len(u.lz.mat)
	}
	return len(u.cs)
}

// Card returns card i, materializing it on an imaged universe.
func (u *Universe) Card(i int) *Card {
	if u.lz != nil {
		return u.lz.card(i)
	}
	return u.cs[i]
}

// Name is card i's primary-face name, or "" when the card has no primary
// face. It never materializes.
func (u *Universe) Name(i int) string {
	if u.lz != nil {
		im := u.lz.img
		r := im.Cards[i]
		if r.Faces.Count == 0 || int(r.Faces.Start) >= len(im.Faces) {
			return ""
		}
		f := im.Faces[r.Faces.Start]
		if f.Nil != 0 {
			return ""
		}
		return im.str(f.Name)
	}
	c := u.cs[i]
	if c == nil || len(c.Faces) == 0 || c.Faces[0] == nil {
		return ""
	}
	return c.Faces[0].Name
}

// NameableFace reports whether face i of a card whose AlternateMode is mode
// is a card name a player may choose (CR 201.4): the primary face always;
// the other half of a split card (CR 201.4b, 709), a flip card's alternative
// name (201.4c, 710.5), a transforming or modal DFC's back face (201.4d,
// 712.19), a meld pair's combined back face (201.4e), and an adventurer,
// omen or preparation card's alternative name (201.4f, 715.5, 720.5,
// 722.5). A specialize face (Alchemy) is not a separately nameable card.
func NameableFace(mode string, i int) bool {
	if i == 0 {
		return true
	}
	switch mode {
	case AlternateSplit, AlternateDoubleFaced, AlternateModal, "Flip", "Meld", "Adventure", "Omen", "Prepare":
		return true
	}
	return false
}

// eachNameableFace calls fn(ord, face, name) for every non-nil nameable face
// (NameableFace) of every card, in ordinal then face order. It never
// materializes an imaged universe.
func (u *Universe) eachNameableFace(fn func(ord, face int, name string)) {
	if u.lz != nil {
		im := u.lz.img
		for i, r := range im.Cards {
			mode := im.str(r.AlternateMode)
			a, z := im.slice(r.Faces, len(im.Faces))
			for j := a; j < z; j++ {
				if f := im.Faces[j]; f.Nil == 0 && NameableFace(mode, j-a) {
					fn(i, j-a, im.str(f.Name))
				}
			}
		}
		return
	}
	for i, c := range u.cs {
		if c == nil {
			continue
		}
		for j, f := range c.Faces {
			if f != nil && NameableFace(c.AlternateMode, j) {
				fn(i, j, f.Name)
			}
		}
	}
}

// Names is the sorted, distinct, non-empty list of every nameable face name
// (NameableFace; what a live match snapshots at genesis). Memoised and
// SHARED: read-only.
func (u *Universe) Names() []string {
	if u == nil {
		return []string{}
	}
	u.namesOnce.Do(func() {
		seen := make(map[string]bool, u.Len())
		out := make([]string, 0, u.Len())
		u.eachNameableFace(func(_, _ int, name string) {
			if name == "" || seen[name] {
				return
			}
			seen[name] = true
			out = append(out, name)
		})
		sort.Strings(out)
		u.names = out[:len(out):len(out)]
	})
	return u.names
}

// FaceByName is the first (ordinal, face) whose nameable face is named
// exactly name, in ordinal then face order: the face a chosen name names
// (a back-face name names that back face, CR 712.19).
func (u *Universe) FaceByName(name string) (ord, face int, ok bool) {
	if u == nil || name == "" {
		return 0, 0, false
	}
	if i, found := u.FirstByName(name); found {
		return i, 0, true
	}
	ord, face = -1, -1
	u.eachNameableFace(func(i, j int, n string) {
		if ord < 0 && j > 0 && n == name {
			ord, face = i, j
		}
	})
	return ord, face, ord >= 0
}

// FirstByName is the first ordinal whose primary-face name is exactly name.
func (u *Universe) FirstByName(name string) (int, bool) {
	if u != nil && u.lz != nil {
		im := u.lz.img
		j := sort.Search(len(im.FirstOrd), func(k int) bool { return string(im.strBytes(im.FirstOrd[k].Name)) >= name })
		if j == len(im.FirstOrd) || string(im.strBytes(im.FirstOrd[j].Name)) != name {
			return 0, false
		}
		return int(im.FirstOrd[j].Ord), true
	}
	for i, n := 0, u.Len(); i < n; i++ {
		if u.Name(i) == name {
			return i, true
		}
	}
	return 0, false
}

// FirstByNormalized is the first ordinal whose primary-face name normalizes
// (NormalizeName) to norm -- state.Game.NamedCard's semantics.
func (u *Universe) FirstByNormalized(norm string) (int, bool) {
	if u == nil || norm == "" {
		return 0, false
	}
	u.normOnce.Do(func() {
		first := map[string]uint32{}
		for i, n := 0, u.Len(); i < n; i++ {
			k := NormalizeName(u.Name(i))
			if _, ok := first[k]; !ok {
				first[k] = uint32(i)
			}
		}
		keys := make([]string, 0, len(first))
		for k := range first {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ords := make([]uint32, len(keys))
		for i, k := range keys {
			ords[i] = first[k]
		}
		u.normKeys, u.normOrds = keys, ords
	})
	j := sort.SearchStrings(u.normKeys, norm)
	if j == len(u.normKeys) || u.normKeys[j] != norm {
		return 0, false
	}
	return int(u.normOrds[j]), true
}

// EachFaceTypes calls fn with the printed type words of every non-nil face
// of every card, in order. On an imaged universe it reads the image rows and
// never materializes; fn must not retain types.
func (u *Universe) EachFaceTypes(fn func(types []string)) {
	if u == nil {
		return
	}
	if u.lz != nil {
		im := u.lz.img
		for _, r := range im.Cards {
			a, z := im.slice(r.Faces, len(im.Faces))
			for _, f := range im.Faces[a:z] {
				if f.Nil == 0 {
					fn(im.words(f.Types))
				}
			}
		}
		return
	}
	for _, c := range u.cs {
		if c == nil {
			continue
		}
		for _, f := range c.Faces {
			if f != nil {
				fn(f.Types)
			}
		}
	}
}

// LandTypeWords returns the universe's land-type vocabulary, built once by
// build (engine/rules/chars owns the word rules) and memoised on the universe.
// SHARED: read-only.
func (u *Universe) LandTypeWords(build func(*Universe) []string) []string {
	if u == nil {
		return build(u)
	}
	u.landOnce.Do(func() {
		out := build(u)
		u.land = out[:len(out):len(out)]
	})
	return u.land
}
