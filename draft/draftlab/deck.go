package draftlab

import (
	"fmt"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/bots/spellbench"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// randomDeckSaltBase tags the random-deck rng stream, cmd/draftsim's
// randomDeck salt.
const randomDeckSaltBase = 0x8AD5E5

// randomDeckMain is how many non-land pool cards draftsim's random deck
// plays before AutoLands fills to 40.
const randomDeckMain = 23

// Deck is one engine-loadable deck the lab produced: from a built pool
// (Build), a random build (RandomDeck), a deck list (ListDeck) or a
// spellbench catalog (CatalogDecks). File always resolves against the
// lab's registry.
type Deck struct {
	// Name is the deck's display name.
	Name string
	// Set is the pool's set code, "" for a list deck.
	Set string
	// Source says where the deck came from: "build:<ref>", "random",
	// "list", or "catalog:<id>".
	Source string
	// DraftID is the 17Lands draft_id of the pool, "" when none. Callers
	// set it (DraftResult.DraftIDs has the per-seat values).
	DraftID string
	// Pool is the deck's pool, in pick order; nil for a list deck.
	Pool []limited.CardRef
	// Build is the built deck the pool produced; nil for a list deck.
	Build *build.Deck
	// File is the engine-loadable list (basics as entries).
	File deck.File
}

// Build turns a pool into a deck with the named builder: the pool's one
// set feeds build.Validate, ToDeckFile writes the engine-loadable file and
// the file is checked to resolve against the lab's registry. The deck's
// name is "<set> <ref>".
func (l *Lab) Build(pool []limited.CardRef, b NamedBuilder) (Deck, error) {
	return l.buildNamed(pool, b, "")
}

// buildNamed is Build's body with an explicit deck-file name ("" names the
// deck "<set> <ref>"); the event driver names its decks
// "p<seat> <set> <mode>" instead.
func (l *Lab) buildNamed(pool []limited.CardRef, b NamedBuilder, name string) (Deck, error) {
	set, err := refsSet(pool)
	if err != nil {
		return Deck{}, fmt.Errorf("draftlab: build %s: %w", b.Ref, err)
	}
	if name == "" {
		name = fmt.Sprintf("%s %s", set, b.Ref)
	}
	var d build.Deck
	if fb, ok := b.Builder.(FallibleBuilder); ok {
		var err error
		if d, err = fb.BuildErr(set, pool); err != nil {
			return Deck{}, fmt.Errorf("draftlab: build %s: %w", b.Ref, err)
		}
	} else {
		d = b.Builder.Build(set, pool)
	}
	if err := build.Validate(pool, d); err != nil {
		return Deck{}, fmt.Errorf("draftlab: build %s: %w", b.Ref, err)
	}
	f := build.ToDeckFile(pool, d, name)
	if _, err := f.Resolve(l.reg); err != nil {
		return Deck{}, fmt.Errorf("draftlab: build %s deck file: %w", b.Ref, err)
	}
	return Deck{
		Name:   name,
		Set:    set,
		Source: "build:" + b.Ref.String(),
		Pool:   append([]limited.CardRef(nil), pool...),
		Build:  &d,
		File:   f,
	}, nil
}

// RandomDeck builds draftsim's random baseline deck from the pool: 23
// random non-land pool cards (Fisher–Yates over a copy from
// rand.NewPCG(seed, randomDeckSaltBase<<32); a pool too thin gives what it
// has) plus build.AutoLands basics up to 40. It is deterministic per seed.
func (l *Lab) RandomDeck(pool []limited.CardRef, seed uint64) (Deck, error) {
	set, err := refsSet(pool)
	if err != nil {
		return Deck{}, fmt.Errorf("draftlab: random deck: %w", err)
	}
	rng := rand.New(rand.NewPCG(seed, randomDeckSaltBase<<32))
	nonLand := make([]limited.CardRef, 0, len(pool))
	for _, ref := range pool {
		c, ok := l.reg.Lookup(ref.Name)
		if !ok {
			return Deck{}, fmt.Errorf("draftlab: random deck: pool card %q is not in the corpus", ref.Name)
		}
		if len(c.Faces) > 0 && c.Faces[0].IsLand() {
			continue
		}
		nonLand = append(nonLand, ref)
	}
	shuffled := append([]limited.CardRef(nil), nonLand...)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	n := randomDeckMain
	if len(shuffled) < n {
		n = len(shuffled)
	}
	d := build.Deck{Basics: [5]int{}}
	var pips [5]int
	for _, ref := range shuffled[:n] {
		d.Main = append(d.Main, ref.ID)
		c, _ := l.reg.Lookup(ref.Name)
		for _, face := range c.Faces {
			for i := 0; i < 5; i++ {
				pips[i] += int(cards.ManaCostChroma(face.ManaCost, 1<<i))
			}
		}
	}
	d.Basics = build.AutoLands(d.Main, func(string) [5]int { return pips })
	f := build.ToDeckFile(pool, d, "random deck")
	if _, err := f.Resolve(l.reg); err != nil {
		return Deck{}, fmt.Errorf("draftlab: random deck: %w", err)
	}
	return Deck{
		Name:   "random deck",
		Set:    set,
		Source: "random",
		Pool:   append([]limited.CardRef(nil), pool...),
		Build:  &d,
		File:   f,
	}, nil
}

// ListDeck adopts a deck list as a lab deck: every card must resolve
// against the lab's registry (the first unknown card is named in the
// error). The deck's Set, Pool and Build stay empty — a list deck is not
// tied to a pool.
func (l *Lab) ListDeck(f deck.File) (Deck, error) {
	if _, err := f.Resolve(l.reg); err != nil {
		return Deck{}, fmt.Errorf("draftlab: list deck %q: %w", f.Name, err)
	}
	return Deck{Name: f.Name, Source: "list", File: f}, nil
}

// CatalogDecks adopts a spellbench catalog's decks as lab decks, in
// CatalogIDs order. Every deck must resolve against the lab's registry.
// id is the catalog directory's stem ("fdn-limited", "pauper-kernel",
// "repo-constructed").
func (l *Lab) CatalogDecks(id string) ([]Deck, error) {
	dir := "decks/" + id
	ids, err := spellbench.CatalogIDs(dir)
	if err != nil {
		return nil, fmt.Errorf("draftlab: catalog %q: %w", id, err)
	}
	out := make([]Deck, 0, len(ids))
	for _, cid := range ids {
		f, err := spellbench.File(dir, cid)
		if err != nil {
			return nil, fmt.Errorf("draftlab: catalog %q: %w", id, err)
		}
		if _, err := f.Resolve(l.reg); err != nil {
			return nil, fmt.Errorf("draftlab: catalog deck %s: %w", cid, err)
		}
		out = append(out, Deck{Name: f.Name, Source: "catalog:" + cid, File: f})
	}
	return out, nil
}

// refsSet derives the one set a pool's CardRefs belong to: the distinct
// Set value ignoring the bonus sheet ("SPG" rides inside another set's
// Play Booster packs by design; see draft.go's packSet). An empty pool or
// a CardRef without a set is an error.
func refsSet(refs []limited.CardRef) (string, error) {
	set := ""
	for _, c := range refs {
		if c.Set == "" {
			return "", fmt.Errorf("pool card %s (%s) carries no set", c.ID, c.Name)
		}
		if c.Set == bonusSheetSet {
			continue
		}
		if set == "" {
			set = c.Set
		}
		if c.Set != set {
			return "", fmt.Errorf("mixed sets in the pool: %q and %q", set, c.Set)
		}
	}
	if set == "" {
		return "", fmt.Errorf("pool carries no base-set cards")
	}
	return set, nil
}
