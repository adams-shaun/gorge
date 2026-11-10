package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// CopyPermanent DefinedName$ (Mutable Explorer): the copy source is a card of
// gorge's name universe, not an object. The named card's face rides the pool
// as a table token (copyCardKey + name, as an override's name card does) so
// the kernel has a CardID to mint the token copy from (kop.ExtCopyNamed).

// copyCardKey prefixes the token key of a card a CopyPermanent copies by name.
const copyCardKey = "copycard:"

// namedCard is the card gorge's CopyPermanent DefinedName$ resolves name to:
// the first card of the whole corpus whose primary face name matches
// (state.Game.NamedCard over the compiled universe). Lookup answers every
// corpus name even on a subset registry (whose Universe is only the subset),
// and a game's name universe is the full corpus except where a name chooser
// narrows it (cardset.copyNamedGuard refuses that matchup).
func (p *pool) namedCard(name string) *cards.Card {
	if p.reg == nil {
		return nil
	}
	want := cards.NormalizeName(name)
	c, ok := p.reg.Lookup(name)
	if !ok || want == "" || len(c.Faces) == 0 || c.Faces[0] == nil || cards.NormalizeName(c.Faces[0].Name) != want {
		return nil
	}
	return c
}

// lowerCopyCardEntry lowers the card named by a copyCardKey token key as a
// table token of its own face, carrying the card's printed name for the
// name filters.
func lowerCopyCardEntry(p *pool, k string) *lCard {
	out := &lCard{Name: k}
	c := p.namedCard(strings.TrimPrefix(k, copyCardKey))
	if c == nil {
		(&lowerCtx{p: p, name: k, out: out, mode: -1, known: p.knownSubtype}).refuseNode("card", "copy-by-name key %q", k)
		return out
	}
	out = lowerCard(p, k, c, false)
	if len(c.Faces) == 1 {
		out.printed = c.Faces[0].Name
	}
	return out
}

// namedCopy is a CopyPermanent DefinedName$ source as copyNamedCard read it.
type namedCopy struct {
	key   string // copyCardKey + the card's name; "" when gorge's universe holds no such card
	found bool
}

// copyNamedCard reads CopyPermanent's DefinedName$ source. A name the
// universe does not hold makes gorge copy nothing (its Note path), which
// found = false lowers to: no instruction. A named card is lowered only when
// it is one plain face that makes no token of its own and lowers cleanly,
// and the shapes the op does not carry (a target, Defined$, Choices$, a copy
// count, counters) refuse by name.
func (lc *lowerCtx) copyNamedCard(ps *params, raw string) (*namedCopy, error) {
	name := strings.TrimSpace(raw)
	for _, k := range []string{"ValidTgts", "Defined", "Choices", "Populate", "NumCopies", "WithCountersType"} {
		if _, ok := ps.m[k]; ok {
			return nil, refuse("CopyPermanent DefinedName$ %q beside %s$", name, k)
		}
	}
	if strings.ContainsAny(name, "|=") {
		return nil, refuse("CopyPermanent DefinedName$ %q", name)
	}
	c := lc.p.namedCard(name)
	if c == nil {
		return &namedCopy{}, nil
	}
	if strings.HasPrefix(lc.name, copyCardKey) {
		return nil, refuse("CopyPermanent DefinedName$ %q on a card that is itself copied by name", name)
	}
	if len(c.Faces) != 1 || c.AlternateMode != "" {
		return nil, refuse("CopyPermanent DefinedName$ %q: not a single-face card", name)
	}
	key := copyCardKey + c.Faces[0].Name
	e := lowerCopyCardEntry(lc.p, key)
	if len(e.Refusals) > 0 {
		return nil, refuse("CopyPermanent DefinedName$ %q: the named card does not lower (%s)", name, e.Refusals[0].Reason)
	}
	if len(e.Tokens) > 0 {
		return nil, refuse("CopyPermanent DefinedName$ %q: the named card makes tokens", name)
	}
	return &namedCopy{key: key, found: true}, nil
}
