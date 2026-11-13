package feat

import (
	"encoding/binary"
	"strconv"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// mzxgraph.go: a research sidecar. A replica of MageZero's
// XMage state encoder (WillWroble/mage branch graph-encoder,
// Mage.Player.AI/.../encoder/StateEncoder.java processState and
// FeatureGraph.getGraphArrays) emitted from a kernel State: the same node
// types, the same names, the same hashing (hash64 / indexFor /
// stringToUUID), the same shared-leaf DAG (a leaf is one node per distinct
// name, a numeric leaf one node per name@value), the same edge labels and
// the same label-overwrite order, so a NetGraph trained on it sees the
// structure it was designed for. The gaps, where the kernel cannot supply
// a fact XMage encodes or supplies it differently: no watcher counts; no
// micro-decision nodes (a record is at the first pick of a multi-pick); no
// InPayManaMode or Activating (autopay); CanActivate only on the deciding
// seat's options at a priority decision; decision text off priority is the
// decision kind's name; token names are the kernel's script keys. Strings
// XMage takes from its card objects (ability rule text, effect text) are
// synthesised from the kernel's card table (cost symbols, op names), so
// they play the same role (shared tokens across cards with the same
// ability) without being XMage's English.
//
// Off by default; a research path, it allocates.

// Java's StateEncoder hash (vocab.feature_id in MageZero's Python).
const (
	mzxGolden = 0x9E3779B185EBCA87
	mzxTable  = 1<<31 - 1
)

func mzxMix64(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// MZXHash64 is StateEncoder.hash64 over s's bytes (UTF-8).
func MZXHash64(s []byte) uint64 {
	h := mzxMix64(mzxGolden ^ (uint64(len(s)) * mzxGolden))
	i := 0
	for ; i+8 <= len(s); i += 8 {
		k := binary.LittleEndian.Uint64(s[i:])
		h ^= mzxMix64(k)
		h = (h<<27|h>>37)*mzxGolden + 0x165667B19E3779F9
	}
	var k uint64
	for j := 0; i+j < len(s); j++ {
		k ^= uint64(s[i+j]) << (8 * j)
	}
	h ^= mzxMix64(k)
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return h
}

// MZXIndexFor is StateEncoder.indexFor (Java long arithmetic).
func MZXIndexFor(h uint64) int32 {
	x := int64(h)
	if x < 0 {
		x = -x
	}
	return int32(x % mzxTable)
}

// MZXFeatureID is indexFor(hash64(s)).
func MZXFeatureID(s string) int32 { return MZXIndexFor(MZXHash64([]byte(s))) }

// MageZero node types (NetGraph NodeType) as written by AppendMZX.
const (
	mzxRoot = 0
)

var mzxTypeName = [...]string{MZRoot: "ROOT", MZPlayer: "PLAYER", MZZone: "ZONE", MZStack: "STACK_OBJECT",
	MZPerm: "PERMANENT", MZCard: "CARD", MZAbility: "ABILITY"}

var mzxTypeID = func() (ids [8]int32) {
	for t, n := range mzxTypeName {
		if t != MZRoot {
			ids[t] = MZXFeatureID(n)
		}
	}
	return
}()

// MZX option kinds (per used option of a record).
const (
	MZXOptPriority = 0
	MZXOptTarget   = 1
	MZXOptUseNo    = 2
	MZXOptUseYes   = 3
	MZXOptNone     = 4
)

// MZX is one encoder user's scratch for AppendMZX.
type MZX struct {
	nodes  map[uint64]int32
	edges  map[uint64]int32
	typ    []uint8
	hid    []uint8
	id     []int32
	val    []int32
	ec, ep []int32
	el     []int32
	optN   []int32
	optK   []uint8
	optP   []uint8
	sb     []byte
	hidden uint8
	// the deciding seat's activatable abilities at a priority decision
	canCast [256]bool
	canAct  map[uint32]bool
	// label ids
	lNone, lName, lStatic, lAttach, lBlocker, lDefender, lSource int32
	lTarget                                                      [8]int32
	// PerfectInfo encodes the opponent's hand (MageZero's config default,
	// hidden_info.see_opponent_hand: true); its nodes carry the hidden flag.
	PerfectInfo bool
}

// NewMZX returns an AppendMZX scratch.
func NewMZX() *MZX {
	g := &MZX{nodes: map[uint64]int32{}, edges: map[uint64]int32{}, canAct: map[uint32]bool{}, PerfectInfo: true}
	g.lNone, g.lName, g.lStatic = MZXFeatureID("NONE"), MZXFeatureID("name"), MZXFeatureID("static")
	g.lAttach, g.lBlocker, g.lDefender = MZXFeatureID("attachment"), MZXFeatureID("blocker"), MZXFeatureID("defender")
	g.lSource = MZXFeatureID("DecisionSource")
	for i := range g.lTarget {
		g.lTarget[i] = MZXFeatureID("TARGET@" + strconv.Itoa(i))
	}
	return g
}

// keys ("UUIDs")
func mzxKeyS(s string) uint64 { return MZXHash64([]byte(s)) }

func mzxKeyObj(kind byte, a, b int) uint64 {
	return mzxMix64(uint64(kind)<<56 ^ uint64(a)<<24 ^ uint64(b) ^ 0x5a5a5a5a)
}

var (
	mzxKRoot  = mzxKeyS("GameRootId")
	mzxKExile = mzxKeyS("ExileId")
	mzxKMainX = mzxKeyS("MainExileZone")
	mzxKStack = mzxKeyS("StackId")
	mzxKStop  = mzxKeyS("StopChoosingNodeId")
	mzxKPass  = mzxKeyS("PassAbilityId")
)

func (g *MZX) newNode(key uint64, typ uint8, id, val int32) (int32, bool) {
	if n, ok := g.nodes[key]; ok {
		return n, false
	}
	n := int32(len(g.typ))
	g.nodes[key] = n
	g.typ = append(g.typ, typ)
	g.hid = append(g.hid, g.hidden)
	g.id = append(g.id, id)
	g.val = append(g.val, val)
	return n, true
}

// edge is FeatureGraph's parent.children.put(child, label): a repeated
// child keeps one edge and takes the latest label.
func (g *MZX) edge(child, parent int32, label int32) {
	k := uint64(uint32(child))<<32 | uint64(uint32(parent))
	if e, ok := g.edges[k]; ok {
		g.el[e] = label
		return
	}
	g.edges[k] = int32(len(g.ec))
	g.ec = append(g.ec, child)
	g.ep = append(g.ep, parent)
	g.el = append(g.el, label)
}

// feature is addFeature(name, parent, edge).
func (g *MZX) feature(name []byte, parent int32, label int32) {
	h := MZXHash64(name)
	n, _ := g.newNode(h, MZLeaf, MZXIndexFor(h), 0)
	if parent >= 0 {
		g.edge(n, parent, label)
	}
}

func (g *MZX) featureS(name string, parent int32) { g.feature([]byte(name), parent, g.lNone) }

// numeric is addNumericFeature(name, num, parent).
func (g *MZX) numeric(name string, num int, parent int32) {
	g.sb = append(append(append(g.sb[:0], name...), '@'), strconv.Itoa(num)...)
	key := MZXHash64(g.sb)
	n, _ := g.newNode(key, MZLeaf, MZXFeatureID(name), int32(num))
	g.edge(n, parent, g.lNone)
}

// node is addNode(type, name, key, parent, label): false when the node
// existed (the edge is still added).
func (g *MZX) node(typ uint8, name []byte, key uint64, parent int32, label int32) (int32, bool) {
	n, fresh := g.newNode(key, typ, mzxTypeID[typ], 0)
	if parent >= 0 {
		g.edge(n, parent, label)
	}
	if fresh {
		g.feature(name, n, g.lName)
	}
	return n, fresh
}

// ---- kernel facts as XMage strings ----

var mzxStepName = [...]string{"UNTAP", "UPKEEP", "DRAW", "PRECOMBAT_MAIN", "BEGIN_COMBAT", "DECLARE_ATTACKERS",
	"DECLARE_BLOCKERS", "COMBAT_DAMAGE", "END_COMBAT", "POSTCOMBAT_MAIN", "END_TURN", "CLEANUP"}

var mzxCardTypes = [...]struct {
	m kop.TypeMask
	n string
}{{kop.TArtifact, "ARTIFACT"}, {kop.TBattle, "BATTLE"}, {kop.TCreature, "CREATURE"}, {kop.TEnchantment, "ENCHANTMENT"},
	{kop.TInstant, "INSTANT"}, {kop.TKindred, "KINDRED"}, {kop.TLand, "LAND"}, {kop.TPlaneswalker, "PLANESWALKER"},
	{kop.TSorcery, "SORCERY"}}

const mzxPermTypes = kop.TArtifact | kop.TBattle | kop.TCreature | kop.TEnchantment | kop.TLand | kop.TPlaneswalker

var mzxColour = [...]string{"WhiteCard", "BlueCard", "BlackCard", "RedCard", "GreenCard"}
var mzxPip = [...]string{"{W}", "{U}", "{B}", "{R}", "{G}", "{C}"}
var mzxManaName = [...]string{"WhiteMana", "BlueMana", "BlackMana", "RedMana", "GreenMana", "ColorlessMana"}

var mzxTrigName = map[kop.TrigEvent]string{
	kop.TrigETB: "ENTERS_THE_BATTLEFIELD", kop.TrigDies: "ZONE_CHANGE", kop.TrigLeaves: "ZONE_CHANGE",
	kop.TrigToGraveyard: "ZONE_CHANGE", kop.TrigAttacks: "ATTACKER_DECLARED", kop.TrigAttackersDeclared: "DECLARED_ATTACKERS",
	kop.TrigSpellCast: "SPELL_CAST", kop.TrigLifeGained: "GAINED_LIFE", kop.TrigLifeLost: "LOST_LIFE",
	kop.TrigDamageDone: "DAMAGED_PLAYER", kop.TrigDamageDealtOnce: "COMBAT_DAMAGE_STEP_POST", kop.TrigBecomesTarget: "TARGETED",
	kop.TrigDrawn: "DREW_CARD", kop.TrigDiscarded: "DISCARDED_CARD", kop.TrigCounterAddedOnce: "COUNTERS_ADDED",
	kop.TrigUntaps: "UNTAPPED", kop.TrigBlocks: "BLOCKER_DECLARED", kop.TrigBecomesBlocked: "CREATURE_BLOCKED",
	kop.TrigSacrificed: "SACRIFICED_PERMANENT", kop.TrigTaps: "TAPPED", kop.TrigTokenCreated: "CREATED_TOKEN",
}

func mzxCounterName(k int) string {
	switch kop.CounterKind(k) {
	case kop.CtrP1P1:
		return "+1/+1"
	case kop.CtrM1M1:
		return "-1/-1"
	}
	if k < len(counterNames) {
		b := []byte(counterNames[k])
		for i := range b {
			if b[i] >= 'A' && b[i] <= 'Z' {
				b[i] += 'a' - 'A'
			}
		}
		return string(b)
	}
	return "counter"
}

// subtype name as XMage's SubType enum constant (Faerie -> FAERIE).
func mzxSubName(s string) []byte {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			b = append(b, c-'a'+'A')
		case c == ' ' || c == '-':
			b = append(b, '_')
		case c == '\'':
		default:
			b = append(b, c)
		}
	}
	return b
}

func mzxManaCostText(dst []byte, m *kop.ManaCost) []byte {
	if m.X > 0 {
		dst = append(dst, "{X}"...)
	}
	if m.Generic > 0 {
		dst = append(append(append(dst, '{'), strconv.Itoa(int(m.Generic))...), '}')
	}
	for c, n := range m.Pips {
		for j := 0; j < int(n); j++ {
			dst = append(dst, mzxPip[c]...)
		}
	}
	return dst
}

func mzxMV(m *kop.ManaCost) int {
	n := int(m.Generic)
	for _, p := range m.Pips {
		n += int(p)
	}
	for _, h := range m.Hyb {
		if h != 0 {
			n++
		}
	}
	return n
}

func mzxCostEmpty(c *kop.Cost) bool {
	return c.Flags == 0 && mzxMV(&c.Mana) == 0 && c.Mana.X == 0
}

// manaCosts is processManaCosts: ManaValue and each symbol's text.
func (g *MZX) manaCosts(m *kop.ManaCost, parent int32) {
	g.numeric("ManaValue", mzxMV(m), parent)
	if m.X > 0 {
		g.featureS("{X}", parent)
	}
	if m.Generic > 0 {
		g.featureS("{"+strconv.Itoa(int(m.Generic))+"}", parent)
	}
	for c, n := range m.Pips {
		if n > 0 {
			g.featureS(mzxPip[c], parent)
		}
	}
	for _, h := range m.Hyb {
		if h != 0 {
			s := []byte{'{'}
			first := true
			for c := 0; c < 5; c++ {
				if h&(1<<c) != 0 {
					if !first {
						s = append(s, '/')
					}
					s = append(s, mzxPip[c][1])
					first = false
				}
			}
			g.feature(append(s, '}'), parent, g.lNone)
		}
	}
}

func mzxCostTexts(c *kop.Cost, f func(string)) {
	n := strconv.Itoa(int(c.N))
	fl := c.Flags
	if fl&kop.CostTap != 0 {
		f("{T}")
	}
	if fl&kop.CostUntap != 0 {
		f("{Q}")
	}
	if fl&kop.CostSacSelf != 0 {
		f("Sacrifice {this}")
	}
	if fl&kop.CostSac != 0 {
		f("Sacrifice " + n)
	}
	if fl&kop.CostRemoveCtr != 0 {
		f("Remove " + n + " " + mzxCounterName(int(c.Ctr)) + " counters from {this}")
	}
	if fl&kop.CostAddCtr != 0 {
		f("Put " + n + " " + mzxCounterName(int(c.Ctr)) + " counters on {this}")
	}
	if fl&kop.CostDiscard != 0 {
		f("Discard " + n)
	}
	if fl&kop.CostPayLife != 0 {
		f("Pay " + n + " life")
	}
	if fl&kop.CostTapX != 0 {
		f("Tap " + n + " untapped creatures you control")
	}
	if fl&kop.CostExileGrave != 0 {
		f("Exile " + n + " cards from your graveyard")
	}
}

func mzxOpText(in *kop.Instr) string {
	s := in.Op.String()
	if s == "Op(?)" {
		s = "Op" + strconv.Itoa(int(in.Op))
	}
	if in.A.K == kop.VConst && in.A.N != 0 {
		s += " " + strconv.Itoa(int(in.A.N))
	}
	return s
}

// abilityName is the ability's XMage-like toString: "Cast X" / "Play X"
// for a card's spell ability, else a rule text synthesised from its cost
// and program.
func mzxAbilityName(cd *kop.CardDef, ab *kop.Ability, cardName []byte) []byte {
	var b []byte
	switch ab.Kind {
	case kop.AbSpell:
		if cd.Types&kop.TLand != 0 {
			return append(append(b, "Play "...), cardName...)
		}
		return append(append(b, "Cast "...), cardName...)
	case kop.AbTriggered:
		b = append(b, "When "...)
		if n, ok := mzxTrigName[ab.Trig.Event]; ok {
			b = append(b, n...)
		} else {
			b = append(b, "TRIGGER"+strconv.Itoa(int(ab.Trig.Event))...)
		}
		b = append(b, ", "...)
	case kop.AbStatic, kop.AbReplacement:
		b = append(append(b, cardName...), ": static "...)
	case kop.AbAltCost:
		b = append(b, "Flashback "...)
		b = mzxManaCostText(b, &cd.AltCost)
		return b
	default:
		b = mzxManaCostText(b, &ab.Cost.Mana)
		first := len(b) == 0
		mzxCostTexts(&ab.Cost, func(t string) {
			if !first {
				b = append(b, ", "...)
			}
			b = append(b, t...)
			first = false
		})
		b = append(b, ": "...)
	}
	if ab.Kind == kop.AbMana {
		b = append(b, "Add "...)
		for c := 0; c < 6; c++ {
			if ab.ManaColors&(1<<c) != 0 {
				b = append(b, mzxPip[c]...)
			}
		}
		return append(b, '.')
	}
	for i := range ab.Prog {
		if i > 0 {
			b = append(b, ". "...)
		}
		b = append(b, mzxOpText(&ab.Prog[i])...)
	}
	return append(b, '.')
}

// ability is addNode(ABILITY) + processAbility / processActivatedAbility /
// processTriggeredAbility.
func (g *MZX) ability(cd *kop.CardDef, abi int, name []byte, key uint64, parent int32, canAct bool) {
	n, fresh := g.node(MZAbility, name, key, parent, g.lNone)
	if !fresh {
		return
	}
	var ab *kop.Ability
	if abi >= 0 && abi < len(cd.Abilities) {
		ab = &cd.Abilities[abi]
	}
	if abi == -1 { // a card's spell ability: its mana cost and its spell's program
		g.manaCosts(&cd.Cost, n)
		for i := range cd.Abilities {
			if cd.Abilities[i].Kind == kop.AbSpell {
				for j := range cd.Abilities[i].Prog {
					g.featureS(mzxOpText(&cd.Abilities[i].Prog[j]), n)
				}
			}
		}
	} else if abi == -3 { // flashback
		g.manaCosts(&cd.AltCost, n)
	} else if ab != nil {
		if !mzxCostEmpty(&ab.Cost) {
			if mzxMV(&ab.Cost.Mana) > 0 || ab.Cost.Mana.X > 0 {
				g.manaCosts(&ab.Cost.Mana, n)
			}
			mzxCostTexts(&ab.Cost, func(t string) { g.featureS(t, n) })
		}
		for j := range ab.Prog {
			g.featureS(mzxOpText(&ab.Prog[j]), n)
		}
		if ab.Kind == kop.AbTriggered {
			if t, ok := mzxTrigName[ab.Trig.Event]; ok {
				g.featureS(t, n)
			}
		}
		if ab.Kind == kop.AbMana {
			g.featureS("ManaAbility", n)
		}
	}
	if canAct {
		g.featureS("CanActivate", n)
	}
}

// card is processCard: counters, PermanentType, card types, colours,
// subtypes, mana costs (the printed card).
func (e *Encoder) mzxCard(g *MZX, c *vcard, cd *kop.CardDef, n int32) {
	for k, v := range c.ctr {
		if v != 0 && k < dirac.FeatCtrDeathtouched {
			g.numeric(mzxCounterName(k), int(v), n)
		}
	}
	if cd.Types&mzxPermTypes != 0 {
		g.feature([]byte("PermanentType"), n, g.lStatic)
	}
	for _, t := range mzxCardTypes {
		if cd.Types&t.m != 0 {
			g.feature([]byte(t.n), n, g.lStatic)
		}
	}
	g.colours(cd.Colors, n, g.lNone)
	e.mzxSubtypes(g, cd.Subtypes, n, g.lStatic)
	g.manaCosts(&cd.Cost, n)
}

func (g *MZX) colours(m kop.ColorMask, n int32, label int32) {
	k := 0
	for c := 0; c < 5; c++ {
		if m&(1<<c) != 0 {
			g.feature([]byte(mzxColour[c]), n, label)
			k++
		}
	}
	if k == 0 {
		g.feature([]byte("ColorlessCard"), n, label)
	}
	if k > 1 {
		g.feature([]byte("MultiColored"), n, label)
	}
}

func (e *Encoder) mzxSubtypes(g *MZX, m kop.SubMask, n int32, label int32) {
	for i, s := range e.pool.subNames {
		if m.Has(i) {
			g.feature(mzxSubName(s), n, label)
		}
	}
}

// cardAbilities is processCardInZone's ability part for zone z (zBattlefield,
// zHand, zGraveyard, zExile, zStack).
func (e *Encoder) mzxCardAbilities(g *MZX, o uint8, c *vcard, cd *kop.CardDef, z uint8, n int32) {
	name := c.c.name
	// static keyword abilities (Flash functions in every zone, the rest on the battlefield)
	if z == zBattlefield {
		e.eachKw(c, func(k *kwInfo) {
			g.ability(cd, -2, k.raw, mzxKeyObj('k', int(o), int(MZXHash64(k.raw)&0xffff)), n, false)
		})
	} else if c.hasFlash() {
		g.ability(cd, -2, []byte("Flash"), mzxKeyObj('k', int(o), 99), n, false)
	}
	for i := range cd.Abilities {
		ab := &cd.Abilities[i]
		zoneOK := false
		switch ab.Kind {
		case kop.AbActivated, kop.AbMana:
			az := ab.ActZone
			switch z {
			case zBattlefield:
				zoneOK = az == kop.ZoneNone || az == kop.ZoneBattlefield
			case zHand:
				zoneOK = az == kop.ZoneHand
			case zGraveyard:
				zoneOK = az == kop.ZoneGraveyard
			}
		case kop.AbTriggered, kop.AbStatic:
			zoneOK = z == zBattlefield
		}
		if zoneOK {
			g.ability(cd, i, mzxAbilityName(cd, ab, name), mzxKeyObj('a', int(o), i), n, g.canAct[uint32(o)<<8|uint32(i)])
		}
	}
	// the spell / land-play ability: playable from hand, encoded for graveyard
	// and exile cards (the permission branch) and battlefield permanents (the
	// "dynamic abilities" block)
	if z != zStack && cd.Types&kop.TToken == 0 {
		g.ability(cd, -1, mzxAbilityName(cd, &kop.Ability{Kind: kop.AbSpell}, name), mzxKeyObj('c', int(o), 0), n, g.canCast[o])
	}
	if z == zGraveyard && cd.Keywords&kop.KwFlashback != 0 {
		g.ability(cd, -3, mzxAbilityName(cd, &kop.Ability{Kind: kop.AbAltCost}, name), mzxKeyObj('f', int(o), 0), n, g.canCast[o])
	}
}

// AppendMZX appends the MageZero graph of the position the last EncodeState
// built (decision d, deciding seat d.Player = MageZero's myPlayer) with
// the record's used options mapped to their action nodes. kind is the
// record's gorge decision kind (the decisionsText leaf), names the two
// seats' player names.
func (e *Encoder) AppendMZX(dst []byte, g *MZX, s *dirac.State, d *dirac.Decision, used []bool, kind string, seq uint64) []byte {
	v := &e.v
	me := dirac.SeatIx(d.Player)
	clear(g.nodes)
	clear(g.edges)
	clear(g.canAct)
	g.canCast = [256]bool{}
	g.typ, g.hid, g.id, g.val = g.typ[:0], g.hid[:0], g.id[:0], g.val[:0]
	g.ec, g.ep, g.el = g.ec[:0], g.ep[:0], g.el[:0]
	g.optN, g.optK, g.optP = g.optN[:0], g.optK[:0], g.optP[:0]
	g.hidden = 0
	if d.Kind == dirac.DkPriority {
		for j := 0; j < int(d.N); j++ {
			a := d.Options[j]
			switch a.Kind() {
			case dirac.AkCast, dirac.AkPlayLand, dirac.AkCastCasualty, dirac.AkCastConspire, dirac.AkSneak, dirac.AkWebSling, dirac.AkMayFlash:
				g.canCast[a.Source()] = true
			case dirac.AkActivate:
				g.canAct[uint32(a.Source())<<8|uint32(a.Mode())] = true
			}
		}
	}
	tbl := e.pool.tbl
	cdOf := func(o uint8) *kop.CardDef { return &tbl.Cards[e.po(o).Card] }
	nameOf := func(o uint8) []byte { return e.pool.card(e.po(o).Card).name }

	root, _ := g.newNode(mzxKRoot, MZRoot, 0, 0)
	if v.step >= 0 {
		g.featureS(mzxStepName[v.step], root)
	}
	dt := "PRIORITY"
	switch d.Kind {
	case dirac.DkPriority:
	case dirac.DkTarget:
		dt = "CHOOSE_TARGET"
	case dirac.DkChoose:
		switch d.Sub {
		case dirac.ChOptional, dirac.ChOptDiscMay:
			dt = "CHOOSE_USE"
		case dirac.ChX, dirac.ChTrigX, dirac.ChMultikick:
			dt = "CHOOSE_NUM"
		case dirac.ChCostDiscard, dirac.ChCleanupDiscard, dirac.ChDiscard, dirac.ChCostSacrifice, dirac.ChSacrifice:
			dt = "CHOOSE_TARGET"
		default:
			dt = "MAKE_CHOICE"
		}
	default:
		dt = "MAKE_CHOICE"
	}
	g.featureS(dt, root)
	if d.Kind == dirac.DkPriority {
		g.featureS("priority", root)
	} else {
		g.featureS(kind, root)
	}
	stop, _ := g.node(MZCard, []byte("StopChoosing"), mzxKStop, root, g.lNone)

	// exile (both players' exiled cards in one main exile zone)
	exile, _ := g.node(MZZone, []byte("Exile"), mzxKExile, root, g.lNone)
	mainX, _ := g.node(MZZone, []byte("exile"), mzxKMainX, exile, g.lNone)
	for p := 0; p < int(v.n); p++ {
		for _, ci := range v.players[p].exile {
			c := &v.cards[ci]
			if n, fresh := g.node(MZCard, c.c.name, mzxKeyObj('o', int(c.obj), 0), mainX, g.lNone); fresh {
				cd := cdOf(c.obj)
				e.mzxCard(g, c, cd, n)
				e.mzxCardAbilities(g, c.obj, c, cd, zExile, n)
			}
		}
	}
	// players: the decision's player, then each other seat in turn order
	var pnode [dirac.MaxSeats]int32
	for k := int(v.n); k < len(pnode); k++ {
		pnode[k] = -1
	}
	for k := 0; k < int(v.n); k++ {
		p := v.seatFrom(me, k)
		pn, _ := g.node(MZPlayer, []byte([...]string{"PlayerA", "PlayerB", "PlayerC", "PlayerD"}[p]), mzxKeyObj('p', int(p), 0), root, g.lNone)
		pnode[dirac.SeatIx(p)] = pn
	}
	for k := 0; k < int(v.n); k++ {
		p := v.seatFrom(me, k)
		e.mzxPlayer(g, s, p, me, pnode[dirac.SeatIx(p)])
	}
	// stack, top first
	stackZ, _ := g.node(MZZone, []byte("Stack"), mzxKStack, root, g.lNone)
	g.numeric("StackSize", len(v.stack), stackZ)
	var snode [dirac.MaxStack]int32
	depth := 0
	for i := len(v.stack) - 1; i >= 0; i-- {
		depth++
		se := &e.kp.Stack[i]
		sv := &v.stack[i]
		src := uint8(se.Source)
		var nm []byte
		var key uint64
		if se.Kind == 0 {
			nm, key = nameOf(src), mzxKeyObj('s', int(src), 0)
		} else {
			cd := &tbl.Cards[se.Card]
			if int(se.Ability) < len(cd.Abilities) {
				nm = mzxAbilityName(cd, &cd.Abilities[se.Ability], e.pool.card(se.Card).name)
			} else {
				nm = e.pool.card(se.Card).name
			}
			key = mzxKeyObj('t', i, int(src)<<8|int(se.Ability))
		}
		n, _ := g.node(MZStack, nm, key, stackZ, g.lNone)
		snode[i] = n
		g.numeric("Depth", depth, n)
		if sv.ctrl == me {
			g.featureS("isController", n)
		}
		if src != noObj && int(src) < int(e.kp.NObj) {
			cn, fresh := g.node(MZCard, nameOf(src), mzxKeyObj('o', int(src), 0), n, g.lNone)
			if fresh && sv.cv >= 0 {
				c := &v.cards[sv.cv]
				cd := cdOf(src)
				e.mzxCard(g, c, cd, cn)
				e.mzxCardAbilities(g, src, c, cd, zStack, cn)
			}
		}
		if se.Kind != 0 {
			g.feature(nm, n, g.lNone)
		} else {
			cd := cdOf(src)
			g.feature(mzxAbilityName(cd, &kop.Ability{Kind: kop.AbSpell}, nameOf(src)), n, g.lNone)
		}
		kicked := 0
		if s.FeatStackKicked(i) {
			kicked = 1
		}
		g.numeric("Kicks", kicked, n)
		g.numeric("XValue", int(se.X), n)
	}
	// targets (after every node exists)
	for i := range v.stack {
		se := &e.kp.Stack[i]
		slot := 0
		add := func(t uint8) {
			if tn := g.targetNode(t, pnode, se.Kind == 0); tn >= 0 {
				g.edge(tn, snode[i], g.lTarget[slot&7])
			}
		}
		for _, t := range se.Targets {
			if dirac.TCode(t) != dirac.NoTarget {
				add(uint8(t))
			}
			slot++
		}
		for j := 0; j < int(se.NExtra); j++ {
			slot = int(se.ExtraSlot[j])
			add(uint8(se.Extra[j]))
		}
	}
	// the decision's source
	if d.Kind != dirac.DkPriority {
		if src := uint8(s.FeatSource()); src != noObj && int(src) < int(e.kp.NObj) {
			g.node(MZCard, nameOf(src), mzxKeyObj('o', int(src), 0), root, g.lSource)
		}
	}
	// the record's used options -> action nodes
	for j := 0; j < int(d.N); j++ {
		if !used[j] {
			continue
		}
		a := d.Options[j]
		node, k := int32(-1), uint8(MZXOptNone)
		switch d.Kind {
		case dirac.DkPriority:
			k = MZXOptPriority
			switch a.Kind() {
			case dirac.AkPass:
				node = g.nodes[mzxKPass]
			case dirac.AkCast, dirac.AkPlayLand, dirac.AkCastCasualty, dirac.AkCastConspire, dirac.AkSneak, dirac.AkWebSling, dirac.AkMayFlash:
				if n, ok := g.nodes[mzxKeyObj('c', int(a.Source()), 0)]; ok {
					node = n
				}
				if a.Mode()&2 != 0 { // flashback
					if n, ok := g.nodes[mzxKeyObj('f', int(a.Source()), 0)]; ok {
						node = n
					}
				}
			case dirac.AkActivate:
				if n, ok := g.nodes[mzxKeyObj('a', int(a.Source()), int(a.Mode()))]; ok {
					node = n
				}
			}
		case dirac.DkTarget:
			k = MZXOptTarget
			if t := a.Target(); a.Kind() == dirac.AkDone || t == dirac.NoTarget {
				node = stop
			} else {
				node = g.targetNode(uint8(t), pnode, true)
			}
		case dirac.DkChoose:
			switch d.Sub {
			case dirac.ChOptional, dirac.ChOptDiscMay:
				k, node = MZXOptUseNo, root
				if a.Mode() == 1 {
					k = MZXOptUseYes
				}
			case dirac.ChCostDiscard, dirac.ChCleanupDiscard, dirac.ChDiscard, dirac.ChCostSacrifice, dirac.ChSacrifice:
				k = MZXOptTarget
				if a.Kind() == dirac.AkDone {
					node = stop
				} else {
					node = g.targetNode(uint8(a.Source()), pnode, true)
				}
			}
		}
		if node < 0 {
			k = MZXOptNone
		}
		pass := uint8(0)
		if a.Kind() == dirac.AkPass {
			pass = 1
		}
		g.optN = append(g.optN, node)
		g.optK = append(g.optK, k)
		g.optP = append(g.optP, pass)
	}
	le := binary.LittleEndian
	dst = le.AppendUint32(dst, 0x31585A4D) // "MZX1"
	dst = le.AppendUint32(dst, uint32(seq))
	dst = append(dst, d.Player, uint8(d.Kind), uint8(d.Sub), 0)
	dst = le.AppendUint32(dst, uint32(len(g.optN)))
	dst = le.AppendUint32(dst, uint32(len(g.typ)))
	dst = le.AppendUint32(dst, uint32(len(g.ec)))
	dst = append(dst, g.typ...)
	dst = append(dst, g.hid...)
	for _, x := range g.id {
		dst = le.AppendUint32(dst, uint32(x))
	}
	for _, x := range g.val {
		dst = le.AppendUint32(dst, uint32(x))
	}
	for _, x := range g.ec {
		dst = le.AppendUint32(dst, uint32(x))
	}
	for _, x := range g.ep {
		dst = le.AppendUint32(dst, uint32(x))
	}
	for _, x := range g.el {
		dst = le.AppendUint32(dst, uint32(x))
	}
	for _, x := range g.optN {
		dst = le.AppendUint32(dst, uint32(x))
	}
	dst = append(dst, g.optK...)
	return append(dst, g.optP...)
}

// targetNode is the node a target t names: a player, a spell's stack
// object (spells: XMage targets the Spell), or the object's
// permanent/card node; -1 when it has none.
func (g *MZX) targetNode(t uint8, pnode [dirac.MaxSeats]int32, spellAsStack bool) int32 {
	if dirac.TCode(t) >= dirac.PlayerTarget0 {
		if p := int(dirac.TCode(t) - dirac.PlayerTarget0); p < dirac.MaxSeats && pnode[p] >= 0 {
			return pnode[p]
		}
		return -1
	}
	if spellAsStack {
		if n, ok := g.nodes[mzxKeyObj('s', int(t), 0)]; ok {
			return n
		}
	}
	if n, ok := g.nodes[mzxKeyObj('o', int(t), 0)]; ok {
		return n
	}
	return -1
}

// mzxPlayer is processPlayer(p) with myPlayer me.
func (e *Encoder) mzxPlayer(g *MZX, s *dirac.State, p, me uint8, pn int32) {
	v := &e.v
	pl := &v.players[p]
	g.node(MZAbility, []byte("PassAbility"), mzxKPass, pn, g.lNone)
	if v.active == p {
		g.featureS("IsActivePlayer", pn)
	}
	if p == me {
		g.featureS("IsDecisionPlayer", pn)
	}
	g.numeric("LifeTotal", int(pl.life), pn)
	if e.kp.Lands[p] == 0 {
		g.featureS("CanPlayLand", pn)
	}
	g.numeric("LibrarySize", int(pl.libN), pn)
	if n := s.Poison(p); n > 0 {
		g.numeric("poison", n, pn)
	}
	for _, c := range [6]int{4, 3, 1, 0, 2, 5} { // Green Red Blue White Black Colorless
		g.numeric(mzxManaName[c], int(pl.pool[c]), pn)
	}
	// graveyard
	gy, _ := g.node(MZZone, []byte("Graveyard"), mzxKeyObj('z', int(p), 1), pn, g.lNone)
	g.numeric("GraveyardSize", len(pl.grave), gy)
	for _, ci := range pl.grave {
		c := &v.cards[ci]
		if n, fresh := g.node(MZCard, c.c.name, mzxKeyObj('o', int(c.obj), 0), gy, g.lNone); fresh {
			cd := &e.pool.tbl.Cards[c.cid]
			e.mzxCard(g, c, cd, n)
			e.mzxCardAbilities(g, c.obj, c, cd, zGraveyard, n)
		}
	}
	// hand
	hz, _ := g.node(MZZone, []byte("Hand"), mzxKeyObj('z', int(p), 2), pn, g.lNone)
	g.numeric("HandSize", int(pl.handN), pn)
	if p == me {
		for _, ci := range pl.hand {
			e.mzxHandCard(g, &v.cards[ci], hz)
		}
	} else if g.PerfectInfo {
		g.hidden = 1
		objs := s.FeatZoneList(p, kop.ZoneHand, e.objs[:0])
		for _, o := range objs {
			vc := e.cardView(s, uint8(o), zHand)
			c := vcard{vcore: vc, obj: uint8(o), zone: zHand, attacking: false, blocks: noObj}
			e.mzxHandCard(g, &c, hz)
		}
		g.hidden = 0
	}
	g.node(MZZone, []byte("CommandZone"), mzxKeyObj('z', int(p), 3), pn, g.lNone)
	// battlefield (permanents p controls)
	bz, _ := g.node(MZZone, []byte("Battlefield"), mzxKeyObj('z', int(p), 4), pn, g.lNone)
	g.numeric("BattlefieldSize", len(pl.bf), bz)
	for _, ci := range pl.bf {
		c := &v.cards[ci]
		n, _ := g.node(MZPerm, c.c.name, mzxKeyObj('o', int(c.obj), 0), bz, g.lNone)
		e.mzxPerm(g, s, c, n, p)
	}
}

func (e *Encoder) mzxHandCard(g *MZX, c *vcard, hz int32) {
	if n, fresh := g.node(MZCard, c.c.name, mzxKeyObj('o', int(c.obj), 0), hz, g.lNone); fresh {
		cd := &e.pool.tbl.Cards[c.cid]
		e.mzxCard(g, c, cd, n)
		e.mzxCardAbilities(g, c.obj, c, cd, zHand, n)
	}
}

// mzxPerm is processPermBattlefield.
func (e *Encoder) mzxPerm(g *MZX, s *dirac.State, c *vcard, n int32, p uint8) {
	cd := &e.pool.tbl.Cards[c.cid]
	e.mzxCard(g, c, cd, n)
	e.mzxCardAbilities(g, c.obj, c, cd, zBattlefield, n)
	g.feature(c.c.name, n, g.lStatic)
	if c.tapped {
		g.featureS("Tapped", n)
	}
	// dynamic types and subtypes (the derived ones when a layer changed them)
	types, subs := cd.Types, cd.Subtypes
	if ty, sb, _, _, ok := s.FeatViewTypes(dirac.Obj(c.obj)); ok {
		types, subs = ty, sb
	}
	for _, t := range mzxCardTypes {
		if types&t.m != 0 {
			g.featureS(t.n, n)
		}
	}
	e.mzxSubtypes(g, subs, n, g.lNone)
	g.colours(cd.Colors, n, g.lStatic)
	// attachments: the permanents attached to this one
	v := &e.v
	for q := 0; q < int(v.n); q++ {
		for _, ai := range v.players[q].bf {
			a := &v.cards[ai]
			if a.attached == c.obj && a.obj != c.obj {
				g.node(MZPerm, a.c.name, mzxKeyObj('o', int(a.obj), 0), n, g.lAttach)
			}
		}
	}
	if c.creature() {
		if c.sick {
			g.featureS("SummoningSick", n)
		}
		if !c.tapped && (!c.sick || c.hasHaste()) && !c.hasDefender() {
			g.featureS("CanAttack", n)
		}
		if !c.tapped {
			g.featureS("CanBlock", n)
		}
		if c.attacking {
			g.featureS("Attacking", n)
			opp := 1 - p
			if v.n > 2 {
				if t, ok := e.st0.HBotAttackTarget(dirac.Obj(c.obj)); ok {
					opp = e.st0.FeatAttackDefender(dirac.TCode(t))
				}
			}
			g.edge(g.nodes[mzxKeyObj('p', int(opp), 0)], n, g.lDefender)
			for q := 0; q < int(v.n); q++ {
				for _, bi := range v.players[q].bf {
					b := &v.cards[bi]
					if b.blocks == c.obj {
						g.node(MZPerm, b.c.name, mzxKeyObj('o', int(b.obj), 0), n, g.lBlocker)
					}
				}
			}
		}
		g.numeric("Damage", int(c.damage), n)
		g.numeric("Power", int(c.power), n)
		g.numeric("Toughness", int(c.tough), n)
	}
}
