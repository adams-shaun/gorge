package feat

import (
	"strings"
	"unicode"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CardSpec is what a pool builder supplies per kernel CardID: the card's
// strings as gorge's view projects them for an ordinary object.
type CardSpec struct {
	Name     string   // the face name (CardView.Name)
	Types    string   // the printed type line, words joined by " " (CardView.Types)
	ManaCost string   // the printed mana cost in Forge notation (CardView.ManaCost)
	SpellAPI string   // the card's spell-ability API, "" for none (CardView.SpellAPI)
	Keywords []string // the printed keywords, gorge's spelling and order (CardView.Keywords)
	// Power and Toughness are the view's P/T off the battlefield (the
	// printed values for a creature card, else 0).
	Power, Toughness int32
	// Avail is what the permanent adds to gorge's AvailableMana total
	// (every colour alternative of its single usable mana ability times its
	// amount) while it can be tapped for mana; 0 for no mana ability.
	Avail int32
	// Effect is the spell ability's class and amount (see EffectSpec).
	Effect EffectSpec
}

// KeywordNames are the gorge spellings of kop.KwMask bits, for keywords a
// permanent gains (an until-end-of-turn grant, a static): bit i of a mask
// spells KeywordNames[i]; "" for a bit gorge never projects as a keyword.
var KeywordNames = func() [64]string {
	var n [64]string
	set := func(m kop.KwMask, s string) {
		for i := 0; i < 64; i++ {
			if m == 1<<i {
				n[i] = s
			}
		}
	}
	set(kop.KwFlying, "Flying")
	set(kop.KwReach, "Reach")
	set(kop.KwFirstStrike, "First Strike")
	set(kop.KwDoubleStrike, "Double Strike")
	set(kop.KwDeathtouch, "Deathtouch")
	set(kop.KwLifelink, "Lifelink")
	set(kop.KwTrample, "Trample")
	set(kop.KwVigilance, "Vigilance")
	set(kop.KwHaste, "Haste")
	set(kop.KwMenace, "Menace")
	set(kop.KwDefender, "Defender")
	set(kop.KwFlash, "Flash")
	set(kop.KwHexproof, "Hexproof")
	set(kop.KwIndestructible, "Indestructible")
	set(kop.KwProwess, "Prowess")
	set(kop.KwHexproofFromInstants, "Hexproof:Instant")
	set(kop.KwPreventCombatDamageTo, "Prevent all combat damage that would be dealt to CARDNAME.")
	set(kop.KwProtBlack, "Protection from black")
	set(kop.KwProtRed, "Protection from red")
	set(kop.KwCantBlock, "HIDDEN CARDNAME can't block.")
	set(kop.KwFlashback, "Flashback") // granted to a graveyard card (Sphinx of Forgotten Lore)
	return n
}()

// counterNames are gorge's counter kinds for kop.CounterKind (Counters map
// keys): the kernel's counter array index i is kind counterNames[i].
var counterNames = [dirac.FeatCtrKinds]string{
	kop.CtrP1P1: "P1P1", kop.CtrM1M1: "M1M1", kop.CtrLoyalty: "LOYALTY", kop.CtrStun: "STUN",
	kop.CtrShield: "SHIELD", kop.CtrFlying: "FLYING", kop.CtrOther: "OTHER",
	kop.CtrIncubation: "INCUBATION", kop.CtrBait: "BAIT", kop.CtrSoul: "SOUL",
	kop.CtrFellowship: "FELLOWSHIP", kop.CtrRevival: "REVIVAL", kop.CtrStash: "STASH",
	dirac.FeatCtrDeathtouched: "Deathtouched", // gorge's deathtouch mark (state.InternalCounterMarker)
}

// counterOrder is the counter kinds in ascending name order (gorge reads a
// counter map in sorted key order).
var counterOrder = func() [dirac.FeatCtrKinds]uint8 {
	var o [dirac.FeatCtrKinds]uint8
	for i := range o {
		o[i] = uint8(i)
	}
	for i := 1; i < len(o); i++ {
		for j := i; j > 0 && counterNames[o[j]] < counterNames[o[j-1]]; j-- {
			o[j], o[j-1] = o[j-1], o[j]
		}
	}
	return o
}()

// Pool is the per-CardID precomputation for one card table. It is built
// once (NewPool, SetCreatureTypes) and only read afterwards: one Pool is
// shared by every encoder of a table, encoding concurrently, so nothing an
// encode reaches may write it (an encoder memoises what the pool did not
// precompute itself: Encoder.typeLine, Encoder.renamed).
type Pool struct {
	cards     []card
	specs     []CardSpec
	ren       []renamedCard // renamed views (renamed)
	tys       []*typeView   // derived type lines (addTypeLine)
	subNames  []string      // the table's subtype names
	allTypes  []string      // gorge's creature-type vocabulary (SetCreatureTypes)
	tbl       *kop.Table
	none      card       // an id past the table
	kwSpelled kop.KwMask // the bits KeywordNames spells
	kw        [64]kwInfo // granted keyword spellings by KwMask bit
	ctr       [dirac.FeatCtrKinds][]byte
}

type kwInfo struct {
	raw     []byte
	lower   []byte
	bit     int8 // keywordBits index, -1 = other
	present bool
	rows    [3]uint16 // "mz|<z>|kw|<lower>" for bf-me, bf-opp, hand
	crRows  [2]uint16 // "mz|<z>|cr|kw|<lower>" for bf-me, bf-opp
}

// mz zone contexts (the per-card row tables' first index).
const (
	zcBfMe = iota
	zcBfOpp
	zcHand
)

var zcNames = [3]string{"bf-me", "bf-opp", "hand"}

type card struct {
	name         []byte
	types        [][]byte // the type line's words
	untab        string   // the printed subtypes the table leaves out (untabledSubs)
	fold, exact  uint16   // typeMasks of the type line
	creature     bool     // hasTypeWord(types, "Creature") (case-insensitive)
	land         bool
	instantType  bool
	mv           int32
	colours      [6]bool
	colourPips   [12]float32 // the v2 block's colour and pip lanes (entColourOff..entCategory-1)
	removalAPI   bool        // the spell API carries a removal marker (removalSpell's printed half)
	rampName     bool        // the name carries a mana-rock marker (rampCard's printed half)
	api          []byte
	kws          []kwInfo
	kwMask       kop.KwMask // the kernel's printed keywords (to tell grants apart)
	flying       bool       // printed keyword flags (hasKeyword, case-insensitive)
	haste        bool
	defender     bool
	flash        bool
	power, tough int32
	avail        int32
	eff          EffectSpec
	pstr         int32 // printed strength (printedStrength)
	// static rows
	nameTag [numZoneTags]uint16
	entName uint16
	entAPI  uint16
	optAPI  uint16
	optCard uint16 // "card|<name>"
	optNone uint16 // "card|<name>\x1fnone"
	// mz rows and prefixes
	typeRows     [3][]uint16 // "mz|<z>|type|<t>" per type word (bf-me, bf-opp, hand)
	castRows     []uint16    // "mz|hand|castable|type|<t>"
	stackTypes   [2][]uint16 // "mz|stack|<me,opp>|type|<t>"
	stackAPI     [2]uint16   // "mz|stack|<me,opp>|api|<api>"
	handAPI      uint16      // "mz|hand|api|<api>"
	handMv       uint16      // "mz|hand|mv<min(mv,8)>"
	attachedHost [2]uint16   // "mz|<z>|attached|<name>" (this card as the host)
	nameP        [2]uint64   // "mz|<name>|<z>|p"
	onPrefix     uint64      // "mz|<name>|on|"
	hDecor       uint64      // "<name>|t"
}

// NewPool builds the pool of tbl from specs (indexed by CardID).
func NewPool(tbl *kop.Table, specs []CardSpec) *Pool {
	p := &Pool{cards: make([]card, len(specs)), tbl: tbl, subNames: tbl.Subtypes}
	for i, s := range KeywordNames {
		if s != "" {
			p.kw[i] = newKw(s)
			p.kwSpelled |= 1 << i
		}
	}
	for i, n := range counterNames {
		p.ctr[i] = []byte(n)
	}
	p.specs = specs
	for i, sp := range specs {
		var m kop.KwMask
		if i < len(tbl.Cards) {
			m = tbl.Cards[i].Keywords
		}
		buildCard(&p.cards[i], sp, m)
		p.cards[i].untab = p.untabledSubs(&p.cards[i])
	}
	// every renamed card the table's overrides can make (FeatNameCard),
	// built now: an encode then never allocates, and a Pool shared by
	// several encoders is only read
	for i := range tbl.Overrides {
		if nc := tbl.Overrides[i].NameCard; nc != 0 {
			for c := range specs {
				p.renamed(kop.CardID(c), kop.CardID(nc-1))
			}
		}
	}
	p.precomputeTypeLines()
	return p
}

// hiddenAbility names a stack ability whose source the viewer can't see, as gorge's
// view does.
var hiddenAbility = func() *card {
	const name = "Ability"
	c := &card{name: []byte(name), entName: row(add(add(off, "ent|name|"), name))}
	for z, tag := range zoneTags {
		c.nameTag[z] = row(add(add(add(off, name), "|"), tag))
	}
	return c
}()

// Effect returns the card's EffectSpec (the zero value for an unknown id).
func (p *Pool) Effect(id kop.CardID) EffectSpec { return p.card(id).eff }

// PrintedStrength returns the card's printed strength (0 for an unknown id).
func (p *Pool) PrintedStrength(id kop.CardID) int32 { return p.card(id).pstr }

func (p *Pool) card(id kop.CardID) *card {
	if int(id) < len(p.cards) {
		return &p.cards[id]
	}
	return &p.none
}

func newKw(s string) kwInfo {
	k := kwInfo{raw: []byte(s), lower: []byte(strings.ToLower(s)), bit: -1, present: true}
	for z, zn := range zcNames {
		k.rows[z] = row(addB(add(off, "mz|"+zn+"|kw|"), k.lower))
		if z < 2 {
			k.crRows[z] = row(addB(add(off, "mz|"+zn+"|cr|kw|"), k.lower))
		}
	}
	for i, w := range keywordBits {
		if strings.EqualFold(s, w) {
			k.bit = int8(i)
			break
		}
	}
	return k
}

// Zone tags of the v1 sparse bag ("<name>|<tag>").
const (
	ztHand = iota
	ztBattleMe
	ztBattleOpp
	ztGraveMe
	ztGraveOpp
	ztExileMe
	ztExileOpp
	ztStack
	numZoneTags
)

var zoneTags = [numZoneTags]string{"hand", "battlefield", "battlefield-opp", "graveyard", "graveyard-opp", "exile", "exile-opp", "stack"}

// ---- policynet's pinned vocabularies (copied; a golden test holds both) ----

var keywordBits = [...]string{
	"Flying", "Reach", "Haste", "Vigilance", "Deathtouch", "Trample",
	"Lifelink", "First Strike", "Double Strike", "Flash", "Indestructible",
	"Devoid", "Defender", "Menace", "Fear", "Shadow", "Ward", "Hexproof",
	"Protection", "Undying", "Persist", "Evolve", "Exalted", "Dethrone",
	"Prowess", "Storm", "Affinity", "Kicker", "Flashback", "Convoke",
	"Cycling",
}

var cardTypeBits = [...]string{
	"Land", "Creature", "Artifact", "Enchantment", "Instant", "Sorcery",
	"Planeswalker", "Battle", "Legendary", "Basic",
}

const (
	tbLand uint16 = 1 << iota
	tbCreature
	tbArtifact
	tbEnchantment
	tbInstant
	tbSorcery
	tbPlaneswalker
	tbBattle
	tbLegendary
	tbBasic
)

func typeMasks(types string) (fold, exact uint16) {
	typeMasksInto(types, &fold, &exact)
	return
}

// typeMasksInto ors the typeMasks of s's fields into fold and exact.
func typeMasksInto(s string, fold, exact *uint16) {
	for w, rest := nextField(s); w != ""; w, rest = nextField(rest) {
		for i, t := range cardTypeBits {
			if w == t {
				*exact |= 1 << i
				*fold |= 1 << i
			} else if strings.EqualFold(w, t) {
				*fold |= 1 << i
			}
		}
	}
}

// buildCard fills c's view and rows from its spec (kwMask: the kernel's
// printed keywords).
func buildCard(c *card, sp CardSpec, kwMask kop.KwMask) {
	c.name = []byte(sp.Name)
	for _, w := range strings.Fields(sp.Types) {
		c.types = append(c.types, []byte(w))
	}
	c.fold, c.exact = typeMasks(sp.Types)
	c.creature = c.fold&tbCreature != 0
	c.land = c.fold&tbLand != 0
	c.instantType = c.fold&tbInstant != 0
	var mv int
	c.colours, mv = manaCostBits(sp.ManaCost)
	c.mv = int32(mv)
	c.eff = sp.Effect
	c.pstr = printedStrength(sp, kwMask, c.mv)
	c.colourPips = v2ColourPips(sp.ManaCost)
	c.removalAPI = containsAnyFold(sp.SpellAPI, removalAPIMarkers)
	c.rampName = containsAnyFold(sp.Name, rampNameMarkers)
	c.api = []byte(sp.SpellAPI)
	for _, k := range sp.Keywords {
		c.kws = append(c.kws, newKw(k))
		switch {
		case strings.EqualFold(k, "Flying"):
			c.flying = true
		case strings.EqualFold(k, "Haste"):
			c.haste = true
		case strings.EqualFold(k, "Defender"):
			c.defender = true
		case strings.EqualFold(k, "Flash"):
			c.flash = true
		}
	}
	c.kwMask = kwMask
	c.power, c.tough, c.avail = sp.Power, sp.Toughness, sp.Avail
	for z, tag := range zoneTags {
		c.nameTag[z] = row(add(add(add(off, sp.Name), "|"), tag))
	}
	c.entName = row(add(add(off, "ent|name|"), sp.Name))
	c.entAPI = row(add(add(off, "ent|api|"), sp.SpellAPI))
	c.optAPI = row(add(add(off, "mz|api|"), sp.SpellAPI))
	c.optCard = row(add(add(off, "card|"), sp.Name))
	c.optNone = row(add(add(off, "card|"+sp.Name), "\x1fnone"))
	for z, zn := range zcNames {
		for _, t := range c.types {
			c.typeRows[z] = append(c.typeRows[z], row(addB(add(off, "mz|"+zn+"|type|"), t)))
		}
	}
	for _, t := range c.types {
		c.castRows = append(c.castRows, row(addB(add(off, "mz|hand|castable|type|"), t)))
		c.stackTypes[0] = append(c.stackTypes[0], row(addB(add(off, "mz|stack|me|type|"), t)))
		c.stackTypes[1] = append(c.stackTypes[1], row(addB(add(off, "mz|stack|opp|type|"), t)))
	}
	c.stackAPI[0] = row(add(add(off, "mz|stack|me|api|"), sp.SpellAPI))
	c.stackAPI[1] = row(add(add(off, "mz|stack|opp|api|"), sp.SpellAPI))
	c.handAPI = row(add(add(off, "mz|hand|api|"), sp.SpellAPI))
	c.handMv = row(addInt(add(off, "mz|hand|mv"), clampI(c.mv, 8)))
	for z := 0; z < 2; z++ {
		c.attachedHost[z] = row(add(add(off, "mz|"+zcNames[z]+"|attached|"), sp.Name))
		c.nameP[z] = add(add(add(off, "mz|"+sp.Name), "|"+zcNames[z]), "|p")
	}
	c.onPrefix = add(add(off, "mz|"+sp.Name), "|on|")
	c.hDecor = add(add(off, sp.Name), "|t")
}

// renamed is the view of card orig renamed by an override (Witness
// Protection): gorge shows the new name with the printed type line, cost
// and abilities. Built once per (orig, name) pair, at construction
// (NewPool); an encode reads them through Encoder.renamed.
func (p *Pool) renamed(orig, name kop.CardID) *card {
	if c, ok := p.findRenamed(orig, name); ok {
		return c
	}
	c := p.buildRenamed(orig, name)
	p.ren = append(p.ren, renamedCard{orig, name, c})
	return c
}

// findRenamed is the precomputed renamed view of (orig, name), or the card
// itself when either id is past the table (ok both ways); it only reads the
// pool.
func (p *Pool) findRenamed(orig, name kop.CardID) (*card, bool) {
	for i := range p.ren {
		if p.ren[i].orig == orig && p.ren[i].name == name {
			return p.ren[i].c, true
		}
	}
	if int(orig) >= len(p.specs) || int(name) >= len(p.specs) {
		return p.card(orig), true
	}
	return nil, false
}

// buildRenamed builds the renamed view of (orig, name) (both in the table)
// without recording it.
func (p *Pool) buildRenamed(orig, name kop.CardID) *card {
	sp := p.specs[orig]
	sp.Name = p.specs[name].Name
	c := &card{}
	buildCard(c, sp, p.cards[orig].kwMask)
	c.untab = p.cards[orig].untab
	return c
}

type renamedCard struct {
	orig, name kop.CardID
	c          *card
}

// typeView is a derived type line gorge's view shows instead of the printed
// one: when a layer-4 effect changed the permanent's type words (gorge
// rules.Engine.ViewTypes) -- Imprisoned in the Moon's "Land", Soulstone
// Sanctuary's "Land Creature" with every creature type, Witness
// Protection's "Creature Citizen", Infernal Vessel's added Demon, Mild-
// Mannered Librarian become a Werewolf. The fields are card's type-line
// ones.
type typeView struct {
	types    kop.TypeMask
	subs     kop.SubMask
	all      bool
	extra    string // the printed subtypes outside the table it keeps, joined by " "
	words    []string
	fold     uint16
	exact    uint16
	creature bool
	land     bool
	typeRows [3][]uint16
	stackTy  [2][]uint16 // "mz|stack|<me|opp>|type|<t>"
}

// typeKey normalises a derived line's shape for (types, subs, all, setSub)
// of card c: an every-creature-type line spells the vocabulary, not the
// mask; the line keeps c's printed subtypes the table does not carry unless
// an effect set its subtypes (setSub) or gave it every creature type.
func typeKey(c *card, types kop.TypeMask, subs kop.SubMask, all, setSub bool) (kop.TypeMask, kop.SubMask, string) {
	types &= lineTypes
	if all {
		subs = kop.SubMask{}
	}
	extra := ""
	if !all && !setSub {
		extra = c.untab
	}
	return types, subs, extra
}

// findTypeLine is the pool's precomputed line of the shape, or nil; it only
// reads the pool.
func (p *Pool) findTypeLine(types kop.TypeMask, subs kop.SubMask, all bool, extra string) *typeView {
	for _, t := range p.tys {
		if t.types == types && t.subs == subs && t.all == all && t.extra == extra {
			return t
		}
	}
	return nil
}

// addTypeLine records the derived line of card c for (types, subs, all,
// setSub) in the pool: construction time only (SetCreatureTypes), for every
// shape the table's overrides can make. An encode reads the shapes through
// Encoder.typeLine, which memoises any other shape per encoder.
func (p *Pool) addTypeLine(c *card, types kop.TypeMask, subs kop.SubMask, all, setSub bool) {
	types, subs, extra := typeKey(c, types, subs, all, setSub)
	if p.findTypeLine(types, subs, all, extra) != nil {
		return
	}
	t := &typeView{types: types, subs: subs, all: all, extra: extra}
	p.fillTypeLine(t, nil)
	p.tys = append(p.tys, t)
}

// tyMemo is an encoder's memo of the derived lines its pool did not
// precompute, and of renamed views likewise (the pool is shared and only
// read). Reserve sizes its backing up front so a first sight builds into
// it without allocating; past that it makes its storage. Entries are never
// reused while the encoder keeps its pool, so a line's pointer stays its
// shape's identity (the content caches compare it).
type tyMemo struct {
	tys   []*typeView
	buf   []typeView // tys' reserved backing
	arena tyArena
	ren   []renamedCard
}

// tyMemo reservations: the overflow lines an encoder keeps and their words.
const (
	tyMemoLines = 64
	tyMemoWords = 1024
)

func (m *tyMemo) reserve() {
	if cap(m.tys) < tyMemoLines {
		m.tys = make([]*typeView, 0, tyMemoLines)
		m.buf = make([]typeView, 0, tyMemoLines)
	}
	if cap(m.arena.words) < tyMemoWords {
		m.arena.words = make([]string, 0, tyMemoWords)
		m.arena.rows = make([]uint16, 0, 5*tyMemoWords)
	}
}

// reset forgets every entry (SetPool: they spell another pool's lines),
// keeping the backing.
func (m *tyMemo) reset() {
	clear(m.buf[:cap(m.buf)])
	m.tys, m.buf = m.tys[:0], m.buf[:0]
	m.arena.words, m.arena.rows = m.arena.words[:0], m.arena.rows[:0]
	m.ren = m.ren[:0]
}

// typeLine is the derived type line view of card c for (types, subs, all,
// setSub): the pool's when it precomputed the shape, else the encoder's,
// built on first sight into the reserved memo. nil when the line spells
// exactly c's printed words.
func (e *Encoder) typeLine(c *card, types kop.TypeMask, subs kop.SubMask, all, setSub bool) *typeView {
	p := e.pool
	types, subs, extra := typeKey(c, types, subs, all, setSub)
	if t := p.findTypeLine(types, subs, all, extra); t != nil {
		return p.unlessPrinted(t, c)
	}
	m := &e.tym
	for _, t := range m.tys {
		if t.types == types && t.subs == subs && t.all == all && t.extra == extra {
			return p.unlessPrinted(t, c)
		}
	}
	var t *typeView
	if len(m.buf) < cap(m.buf) {
		m.buf = m.buf[:len(m.buf)+1]
		t = &m.buf[len(m.buf)-1]
	} else {
		t = new(typeView)
	}
	*t = typeView{types: types, subs: subs, all: all, extra: extra}
	p.fillTypeLine(t, &m.arena)
	m.tys = append(m.tys, t)
	return p.unlessPrinted(t, c)
}

// renamed is the pool's renamed view of (orig, name), or the encoder's
// when the pool's overrides did not make it (built on first sight).
func (e *Encoder) renamed(orig, name kop.CardID) *card {
	if c, ok := e.pool.findRenamed(orig, name); ok {
		return c
	}
	for i := range e.tym.ren {
		if e.tym.ren[i].orig == orig && e.tym.ren[i].name == name {
			return e.tym.ren[i].c
		}
	}
	c := e.pool.buildRenamed(orig, name)
	e.tym.ren = append(e.tym.ren, renamedCard{orig, name, c})
	return c
}

// tyArena is the backing storage a derived line's words and rows are cut
// from (an encoder's, reserved up front); a nil or full arena makes them.
type tyArena struct {
	words []string
	rows  []uint16
}

func (a *tyArena) takeWords(n int) []string {
	if a == nil || len(a.words)+n > cap(a.words) {
		return make([]string, n)
	}
	w := a.words[len(a.words) : len(a.words)+n : len(a.words)+n]
	a.words = a.words[:len(a.words)+n]
	return w
}

func (a *tyArena) takeRows(n int) []uint16 {
	if a == nil || len(a.rows)+n > cap(a.rows) {
		return make([]uint16, n)
	}
	r := a.rows[len(a.rows) : len(a.rows)+n : len(a.rows)+n]
	a.rows = a.rows[:len(a.rows)+n]
	return r
}

// lineTypeWords are the type-mask bits a derived line spells, in its order.
var lineTypeWords = [...]struct {
	m kop.TypeMask
	s string
}{{kop.TLegendary, "Legendary"}, {kop.TBasic, "Basic"}, {kop.TKindred, "Kindred"}, {kop.TLand, "Land"},
	{kop.TArtifact, "Artifact"}, {kop.TEnchantment, "Enchantment"}, {kop.TCreature, "Creature"},
	{kop.TPlaneswalker, "Planeswalker"}, {kop.TBattle, "Battle"}, {kop.TInstant, "Instant"}, {kop.TSorcery, "Sorcery"}}

// fillTypeLine spells t's words and rows from its shape (types, subs, all,
// extra), cutting their storage from a: with an arena of room left it
// allocates nothing.
func (p *Pool) fillTypeLine(t *typeView, a *tyArena) {
	n := 0
	for _, w := range lineTypeWords {
		if t.types&w.m != 0 {
			n++
		}
	}
	if t.all {
		n += len(p.allTypes)
	} else {
		for w, rest := nextField(t.extra); w != ""; w, rest = nextField(rest) {
			n++
		}
		for i := range p.subNames {
			if t.subs.Has(i) {
				n++
			}
		}
	}
	words := a.takeWords(n)[:0]
	for _, w := range lineTypeWords {
		if t.types&w.m != 0 {
			words = append(words, w.s)
		}
	}
	if t.all {
		words = append(words, p.allTypes...)
	} else {
		for w, rest := nextField(t.extra); w != ""; w, rest = nextField(rest) {
			words = append(words, w)
		}
		for i, s := range p.subNames {
			if t.subs.Has(i) {
				words = append(words, s)
			}
		}
	}
	t.words = words
	// the line's masks: its words joined by " " read back as fields
	t.fold, t.exact = 0, 0
	for _, w := range words {
		typeMasksInto(w, &t.fold, &t.exact)
	}
	t.creature = t.fold&tbCreature != 0
	t.land = t.fold&tbLand != 0
	rows := a.takeRows(5 * n)
	for z, zn := range zcNames {
		r := rows[z*n : z*n : (z+1)*n]
		h := add(add(add(off, "mz|"), zn), "|type|")
		for _, w := range words {
			r = append(r, row(add(h, w)))
		}
		t.typeRows[z] = r
	}
	for side, sn := range [2]string{"me", "opp"} {
		r := rows[(3+side)*n : (3+side)*n : (4+side)*n]
		h := add(add(add(off, "mz|stack|"), sn), "|type|")
		for _, w := range words {
			r = append(r, row(add(h, w)))
		}
		t.stackTy[side] = r
	}
}

// nextField is strings.Fields' next field of s and what follows it ("" at
// the end), without building the field slice.
func nextField(s string) (w, rest string) {
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	i := strings.IndexFunc(s, unicode.IsSpace)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i:]
}

// typeWords are the card types and supertypes a type line spells.
var typeWords = [...]string{"Legendary", "Basic", "Kindred", "Land", "Artifact", "Enchantment", "Creature",
	"Planeswalker", "Battle", "Instant", "Sorcery", "Snow", "World", "Tribal"}

// untabledSubs is c's printed subtypes the table's subtype names leave out,
// in printed order (built at construction into card.untab).
func (p *Pool) untabledSubs(c *card) string {
	var b []byte
words:
	for _, w := range c.types {
		for _, t := range typeWords {
			if string(w) == t {
				continue words
			}
		}
		for _, n := range p.subNames {
			if string(w) == n {
				continue words
			}
		}
		if len(b) > 0 {
			b = append(b, ' ')
		}
		b = append(b, w...)
	}
	return string(b)
}

// has reports whether the line spells word w.
func (t *typeView) has(w []byte) bool {
	for _, x := range t.words {
		if x == string(w) {
			return true
		}
	}
	return false
}

// unlessPrinted is t, or nil when t spells exactly c's printed words.
func (p *Pool) unlessPrinted(t *typeView, c *card) *typeView {
	if t.all || len(t.words) != len(c.types) {
		return t
	}
	n := 0
	for i := range c.types {
		for j := range c.types[:i] {
			if string(c.types[j]) == string(c.types[i]) {
				return t // a repeated printed word: keep the derived line
			}
		}
		if t.has(c.types[i]) {
			n++
		}
	}
	if n == len(c.types) {
		return nil
	}
	return t
}

// lineTypes are the type bits a type line spells (card types and
// supertypes; the subtype-like bits are read from the subtype mask).
const lineTypes = kop.TLand | kop.TCreature | kop.TArtifact | kop.TEnchantment | kop.TPlaneswalker |
	kop.TInstant | kop.TSorcery | kop.TBattle | kop.TKindred | kop.TLegendary | kop.TBasic

// SetCreatureTypes gives the pool gorge's creature-type vocabulary (an
// "all creature types" line lists it) and rebuilds the derived type lines
// the table's overrides can make. Construction time only: before any
// encoder of the pool encodes.
func (p *Pool) SetCreatureTypes(words []string) {
	p.allTypes = append([]string(nil), words...)
	p.precomputeTypeLines()
}

// precomputeTypeLines builds the derived type lines the table's single
// overrides make with a card-type change (the other shapes, stacked
// overrides and subtype-only changes, an encoder memoises: Encoder.typeLine).
func (p *Pool) precomputeTypeLines() {
	p.tys = p.tys[:0]
	if p.tbl == nil {
		return
	}
	const cardTypes = kop.TLand | kop.TCreature | kop.TArtifact | kop.TEnchantment | kop.TPlaneswalker |
		kop.TInstant | kop.TSorcery | kop.TBattle | kop.TKindred
	for i := range p.tbl.Overrides {
		o := &p.tbl.Overrides[i]
		if !o.HasTypes && o.AddTypes == 0 && o.RemoveTypes == 0 {
			continue
		}
		for c := range p.tbl.Cards {
			cd := &p.tbl.Cards[c]
			types, subs, all := cd.Types, cd.Subtypes, false
			if o.HasTypes {
				types = types&^cardTypes | o.SetTypes
			}
			types = types&^o.RemoveTypes | o.AddTypes
			if o.HasSub {
				subs = subs.And(o.KeepSub).Or(o.SetSub)
			}
			subs = subs.Or(o.AddSub)
			all = o.AllCreatureTypes
			if (types^cd.Types)&cardTypes != 0 {
				p.addTypeLine(&p.cards[c], types, subs, all, o.HasSub)
			}
		}
	}
}
