package oracle

import (
	"sort"
	"strings"
	"sync/atomic"

	"github.com/adams-shaun/gorge/bots/spellbench/builtins"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
)

// sbtCard is one table card's gorge knowledge, resolved from the corpus on
// first use (SBTCards.card): the strings gorge's view projects for an
// object of that card (name, type line, mana cost, spell API, mana
// production), the kop -> gorge ability index map, and the compiled card
// itself. The derived view fields (keywords and P/T off the battlefield)
// live in sbtView and are computed only when a view reads them: building
// them needs a scratch engine, and the ability mapping and the hbot facts
// table never look at them.
type sbtCard struct {
	name     string
	types    string
	manaCost string
	spellAPI string
	pt       string
	prod     *cards.ManaProduction
	gc       *cards.Card
	creature bool // the printed type line has Creature
	pool     int  // set pool index (-1 none): AbilityOrigin
	ward     uint8
	vw       atomic.Pointer[sbtView]
}

// sbtView is a table card's derived view fields, computed on first view
// use (SBTCards.view) and cached: printed keywords (gorge's spelling,
// ViewCharacteristics off the battlefield) and the view's P/T.
type sbtView struct {
	kw       []string
	pow, tou int32
}

// SBTCards is the per-table gorge knowledge the sbt agent (sbt.go) reads.
// A table card is resolved from reg on first use and cached, so the corpus'
// imaged registry materialises only the cards a game actually touches: the
// whole table (or corpus) is never walked up front. Read-only afterwards,
// so any number of agents (one per goroutine) may share it.
type SBTCards struct {
	tbl    *kop.Table
	set    cardset.Set
	reg    *cards.Registry
	names  map[string]string
	at     []atomic.Pointer[sbtCard]
	toks   map[string][]*cards.Card
	lookup builtins.CardLookup
}

// NewSBTCards is NewSBTCardsSet over the "fdn" set.
func NewSBTCards(reg *cards.Registry, tbl *kop.Table, names map[string]string) *SBTCards {
	return NewSBTCardsSet(reg, tbl, names, fdnSet())
}

// NewSBTCardsSet resolves tbl's cards in reg the way FeatPool does (a token
// by its script, a card by name; names maps a kernel name to gorge's), with
// set supplying the pool indexes and ability origins. Resolution is lazy per
// card: reg.Lookup runs on first touch, never a whole-corpus Catalog.
func NewSBTCardsSet(reg *cards.Registry, tbl *kop.Table, names map[string]string, set cardset.Set) *SBTCards {
	c := &SBTCards{tbl: tbl, set: set, reg: reg, names: names,
		at: make([]atomic.Pointer[sbtCard], len(tbl.Cards)), toks: tokenIndex(reg),
		lookup: builtins.NewRegistryLookup(reg)}
	return c
}

// tokenIndex indexes the corpus' compiled token scripts by face name (first
// stem in sorted order wins), the token half of FeatPool's resolution. It
// touches only the registry's already-compiled Tokens, not its cards.
func tokenIndex(reg *cards.Registry) map[string][]*cards.Card {
	tokensByName := map[string][]*cards.Card{}
	var tkeys []string
	for k := range reg.Tokens {
		tkeys = append(tkeys, k)
	}
	sort.Strings(tkeys)
	for _, k := range tkeys {
		tc := reg.Tokens[k]
		if nm := cardName(tc); nm != "" {
			tokensByName[nm] = append(tokensByName[nm], tc)
		}
	}
	return tokensByName
}

// card resolves table card i on first use and returns its knowledge; nil
// when i is outside the table. Concurrent callers race benignly: the first
// resolution publishes, the rest load it.
func (c *SBTCards) card(i dirac.CardID) *sbtCard {
	if int(i) >= len(c.at) {
		return nil
	}
	if p := c.at[i].Load(); p != nil {
		return p
	}
	p := c.resolve(i)
	if c.at[i].CompareAndSwap(nil, p) {
		return p
	}
	return c.at[i].Load()
}

// resolve builds table card i's knowledge from the corpus, materialising
// that one card (and the token it names) through the registry's lazy route.
func (c *SBTCards) resolve(i dirac.CardID) *sbtCard {
	cd := &c.tbl.Cards[i]
	nm := cd.Name
	if g, ok := c.names[nm]; ok {
		nm = g
	}
	s := &sbtCard{pool: -1, ward: 255}
	var gc *cards.Card
	if tc, ok := c.reg.Tokens[nm]; ok && cd.Types&kop.TToken != 0 {
		gc = tc
	} else if cd.Types&kop.TToken != 0 {
		toks := c.toks[nm]
		if extra := c.toks[nm+" Token"]; len(extra) > 0 {
			// copy: c.toks is shared by concurrent resolvers
			toks = append(append([]*cards.Card(nil), toks...), extra...)
		}
		gc = matchToken(cd, toks)
	} else if lc, ok := c.reg.Lookup(nm); ok {
		gc = lc
	}
	s.gc = gc
	if j, ok := c.set.Lookup(cd.Name); ok {
		s.pool = j
	}
	if gc != nil {
		for j, tr := range gc.Faces[0].Triggers {
			if tr.Params["Keyword"] == "Ward" {
				s.ward = 0x80 | uint8(j)
			}
		}
	}
	if gc == nil {
		s.name = strings.TrimPrefix(cd.Name, "name:")
		s.types = kernelTypeLine(c.tbl, cd)
		s.creature = cd.Types&kop.TCreature != 0
		return s
	}
	f := gc.Faces[0]
	s.name = f.Name
	s.types = strings.Join(f.Types, " ")
	s.manaCost = f.ManaCost
	s.pt = f.PT
	if sa := f.SpellAbility(); sa != nil {
		s.spellAPI = sa.API
	}
	if mp := f.ManaProduction(); !mp.IsZero() {
		p := mp
		s.prod = &p
	}
	for _, t := range f.Types {
		if t == "Creature" {
			s.creature = true
		}
	}
	return s
}

// view returns table card i's derived view fields, computing them on first
// use. Only sbt_view reads them; resolving a card for its strings or
// abilities (HBotFactsSet, gorgeAbility, gorgeSA) stays engine-free.
func (c *SBTCards) view(i dirac.CardID) *sbtView {
	sc := c.card(i)
	if sc == nil {
		return nil
	}
	if v := sc.vw.Load(); v != nil {
		return v
	}
	v := c.resolveView(i, sc)
	if sc.vw.CompareAndSwap(nil, v) {
		return v
	}
	return sc.vw.Load()
}

// resolveView builds card i's derived view fields: gorge's own keywords and
// P/T for an object off the battlefield, from a one-card scratch engine so
// only this card materialises (the old whole-table engine materialised
// every card in the table).
func (c *SBTCards) resolveView(i dirac.CardID, sc *sbtCard) *sbtView {
	v := &sbtView{}
	if sc.gc == nil {
		cd := &c.tbl.Cards[i]
		v.pow, v.tou = int32(cd.Power), int32(cd.Toughness)
		return v
	}
	e := rules.New(rules.Config{Seed: 1, Names: []string{"p0", "p1"}, Decks: [][]*cards.Card{{sc.gc}, {sc.gc}}, Tokens: c.reg.Tokens})
	_, _, kws, p, t := e.ViewCharacteristics(e.G.Objs[0].ID)
	v.kw = append([]string(nil), kws...)
	v.pow, v.tou = p, t
	return v
}

// Lookup is the CardLookup the agents' seats share (gorge's compiled card
// by name, tokens by face name).
func (c *SBTCards) Lookup() builtins.CardLookup { return c.lookup }

// gorgeAbility maps card's kop ability index to gorge's Face.Abilities
// index (activated and mana abilities), 0x80|Face.Triggers index for a
// trigger; ok false when the ability has no gorge counterpart (FDNSide's
// origin rule, falling back to RealKernel's ordinal rule for a table outside
// the generated pool).
func (c *SBTCards) gorgeAbility(card dirac.CardID, ab uint8) (uint8, bool) {
	if int(card) >= len(c.tbl.Cards) {
		return ab, false
	}
	sc := c.card(card)
	if ab == 253 {
		return sc.ward, sc.ward != 255
	}
	if ab >= 253 {
		return ab, false
	}
	abs := c.tbl.Cards[card].Abilities
	if int(ab) < len(abs) && abs[ab].AFlags&kop.AfLatent != 0 {
		return 255, false
	}
	if sc.pool >= 0 {
		o := c.set.AbilityOrigin(sc.pool, int(ab))
		return o, o != 0xFF
	}
	if int(ab) >= len(abs) {
		return 255, false
	}
	trig := abs[ab].Kind == kop.AbTriggered
	n := 0
	for i := 0; i < int(ab); i++ {
		k := abs[i].Kind
		if trig && k == kop.AbTriggered || !trig && (k == kop.AbActivated || k == kop.AbMana) {
			n++
		}
	}
	if trig {
		return 0x80 | uint8(n), true
	}
	return uint8(n), true
}

// gorgeSA is the gorge spell ability gorge index gi of card names: the
// Face.Abilities entry, the Face.Triggers entry's execute SA (0x80|i), or
// the card's spell ability (255).
func (c *SBTCards) gorgeSA(card dirac.CardID, gi uint8) *cards.SA {
	sc := c.card(card)
	if sc == nil || sc.gc == nil {
		return nil
	}
	return canon.FaceSA(sc.gc.Faces[0], gi)
}
