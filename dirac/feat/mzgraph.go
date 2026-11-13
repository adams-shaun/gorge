package feat

import (
	"encoding/binary"
	"math"

	"github.com/adams-shaun/gorge/dirac"
)

// mzgraph.go: a research sidecar. It writes the position the
// last EncodeState built as a MageZero-style typed state graph
// (github.com/WillWroble/MageZero, branch graph-encoder-test, NetGraph):
// typed nodes ROOT, PLAYER, ZONE, STACK_OBJECT, PERMANENT, CARD and
// ABILITY, LEAF nodes carrying (feature id, small integer value), and
// labelled child -> parent edges. The hierarchy is
//
//	ABILITY -> CARD / PERMANENT / STACK_OBJECT -> ZONE -> PLAYER -> ROOT
//
// plus cross edges (an aura to its host, a blocker to its attacker, a
// stack entry's targets to the entry, an ability's source to its stack
// entry, an option's object to the option). An ABILITY node is one option
// of the decision the corpus record stores (its used options, in the
// record's order), carrying the option encoding as leaves, so the graph
// net and the MLP see the same information. The leaves are the facts the
// MLP reads: each nonzero entity raw slot (as an integer: the slot's
// normalisation undone), the entity identity rows, the dense vector and
// the sparse bag (under ROOT), and per-player scalars. Hidden information
// is the deciding seat's view's: the opponent's hand and both libraries
// are counts only.
//
// Nothing here runs unless a caller asks; it allocates
// only into the caller's MZGraph buffers.

// MageZero node types (NetGraph NodeType).
const (
	MZRoot    = 0
	MZPlayer  = 1
	MZZone    = 2
	MZStack   = 3
	MZPerm    = 4
	MZCard    = 5
	MZAbility = 6
	MZLeaf    = 7
)

// Leaf families: a leaf's feature id is family<<24 | index, and the edge
// from a leaf to its node is labelled with the family.
const (
	lfRaw    = 1  // entity raw slot
	lfRow    = 2  // entity identity row (name, spell API)
	lfDense  = 3  // state dense slot
	lfBag    = 4  // state sparse bag row
	lfPlayer = 5  // player scalar
	lfZone   = 6  // zone name (index) and size (16 + index)
	lfStep   = 7  // step name
	lfStack  = 9  // stack entry facts
	lfOSlot  = 10 // option slot row
	lfOHash  = 11 // option hashed row
	lfODense = 12 // option dense slot
	lfOBot   = 13 // the option is the heuristic bot's answer
)

// Structural edge labels (above every leaf family).
const (
	elIn       = 20         // object -> its zone
	elZone     = 21         // zone -> player
	elPlayer   = 22         // player -> root
	elAttach   = 23         // attached permanent -> host
	elBlocks   = 24         // blocker -> the attacker it blocks
	elTarget   = 25         // targeted object -> stack entry
	elTPlayer  = 26         // targeted player -> stack entry
	elSrc      = 27         // ability source -> its stack entry
	elOpt      = 28         // option -> its object (or player)
	elOptSrc   = 29         // the option's object -> option (cross edge)
	elRel      = 30         // the option's related object (a blocked attacker) -> option
	elStackZ   = 31         // the stack zone -> root
	elLibrary  = 32         // unused
	mzMagic    = 0x31475A4D // "MZG1"
	mzZoneKind = 6          // battlefield, hand, graveyard, exile, library, stack
)

// rawScale undoes each entity raw slot's normalisation (state.go
// entityCard) so a leaf value is the integer fact.
var rawScale = func() [EntityRawWidth]float32 {
	var s [EntityRawWidth]float32
	for i := range s {
		s[i] = 1
	}
	for _, i := range []int{entPower, entToughness, entCtrLoyalty, entCombatFoePow, entCombatFoeTgh} {
		s[i] = 4
	}
	for _, i := range []int{entCtrPlus, entCtrMinus, entCtrOther, entHosts, entCombatFoes} {
		s[i] = 2
	}
	s[entManaValue], s[entStackX] = 5, 5
	for k := 0; k < 6; k++ {
		s[entPipsOff+k] = pipMax
	}
	return s
}()

// MZGraph is one encoder user's graph scratch, reused every record.
type MZGraph struct {
	typ  []uint8
	id   []uint32
	val  []int16
	ec   []uint16
	ep   []uint16
	el   []uint8
	on   [256]int32 // object -> node + 1, stamped by gen
	og   [256]uint32
	gen  uint32
	optN []uint16
	optP []uint8
	card Card
}

// NewMZGraph returns a graph scratch sized for any kernel position.
func NewMZGraph() *MZGraph {
	const n = 1 << 13
	return &MZGraph{
		typ: make([]uint8, 0, n), id: make([]uint32, 0, n), val: make([]int16, 0, n),
		ec: make([]uint16, 0, n), ep: make([]uint16, 0, n), el: make([]uint8, 0, n),
		optN: make([]uint16, 0, dirac.MaxOptions), optP: make([]uint8, 0, dirac.MaxOptions),
	}
}

func (g *MZGraph) node(t uint8) int {
	g.typ = append(g.typ, t)
	g.id = append(g.id, 0)
	g.val = append(g.val, 0)
	return len(g.typ) - 1
}

func (g *MZGraph) edge(child, parent int, label uint8) {
	g.ec = append(g.ec, uint16(child))
	g.ep = append(g.ep, uint16(parent))
	g.el = append(g.el, label)
}

func clip16(x int64) int16 {
	if x > math.MaxInt16 {
		return math.MaxInt16
	}
	if x < math.MinInt16 {
		return math.MinInt16
	}
	return int16(x)
}

func (g *MZGraph) leaf(owner int, fam uint8, index uint32, v int64) {
	n := g.node(MZLeaf)
	g.id[n] = uint32(fam)<<24 | index&0xFFFFFF
	g.val[n] = clip16(v)
	g.edge(n, owner, fam)
}

func (g *MZGraph) setObj(o uint8, n int) {
	g.on[o], g.og[o] = int32(n)+1, g.gen
}

func (g *MZGraph) objNode(o uint8) int {
	if o == noObj || g.og[o] != g.gen {
		return -1
	}
	return int(g.on[o]) - 1
}

// entityLeaves hangs c's entity encoding (raw slots and identity rows)
// under node n.
func (e *Encoder) entityLeaves(g *MZGraph, n int, c *vcard, zone int, seat uint8, avail int32) {
	g.card = Card{}
	e.entityCard(&g.card, c, zone, seat, avail)
	for i, x := range g.card.Raw {
		if x != 0 {
			g.leaf(n, lfRaw, uint32(i), int64(math.Round(float64(x*rawScale[i]))))
		}
	}
	for i := 0; i < int(g.card.NRows); i++ {
		r := g.card.Rows[i]
		g.leaf(n, lfRow, uint32(r.Row), int64(math.Round(float64(r.Value))))
	}
}

// AppendMZGraph appends the graph of the position the last EncodeState
// built (for decision d, whose options EncodeOptions* last encoded into
// opts) to dst: the record's used options become ABILITY nodes, bot names
// the heuristic bot's answers. seq is the record's decision sequence.
func (e *Encoder) AppendMZGraph(dst []byte, g *MZGraph, d *dirac.Decision, opts []Option, used, bot []bool, seq uint64) []byte {
	v := &e.v
	st := &e.st
	seat := v.seat
	g.typ, g.id, g.val = g.typ[:0], g.id[:0], g.val[:0]
	g.ec, g.ep, g.el = g.ec[:0], g.ep[:0], g.el[:0]
	g.optN, g.optP = g.optN[:0], g.optP[:0]
	g.gen++
	if g.gen == 0 {
		g.og = [256]uint32{}
		g.gen = 1
	}
	me := &v.players[dirac.SeatIx(seat)]

	root := g.node(MZRoot)
	if v.step >= 0 {
		g.leaf(root, lfStep, uint32(v.step), 1)
	}
	for i, x := range st.Dense {
		if x != 0 && i != 1 {
			g.leaf(root, lfDense, uint32(i), int64(math.Round(float64(x))))
		}
	}
	for _, f := range st.Sparse {
		g.leaf(root, lfBag, uint32(f.Row), int64(math.Round(float64(f.Value))))
	}

	// players (absolute seats) and their zones
	var pnode [dirac.MaxSeats]int
	var zone [dirac.MaxSeats][mzZoneKind]int
	for k := 0; k < int(v.n); k++ {
		p := uint8(k)
		pl := &v.players[k]
		n := g.node(MZPlayer)
		pnode[k] = n
		g.edge(n, root, elPlayer)
		if p == seat {
			g.leaf(n, lfPlayer, 0, 1)
		}
		if p == v.active {
			g.leaf(n, lfPlayer, 1, 1)
		}
		if p == v.priority {
			g.leaf(n, lfPlayer, 2, 1)
		}
		g.leaf(n, lfPlayer, 3, int64(pl.life))
		g.leaf(n, lfPlayer, 4, int64(pl.handN))
		g.leaf(n, lfPlayer, 5, int64(pl.libN))
		g.leaf(n, lfPlayer, 6, int64(pl.graveN))
		for c := 0; c < 6; c++ {
			if pl.pool[c] != 0 {
				g.leaf(n, lfPlayer, uint32(7+c), int64(pl.pool[c]))
			}
		}
		g.leaf(n, lfPlayer, 13, int64(pl.avail))
		sizes := [mzZoneKind - 1]int{len(pl.bf), int(pl.handN), len(pl.grave), len(pl.exile), int(pl.libN)}
		for z := 0; z < mzZoneKind-1; z++ {
			zn := g.node(MZZone)
			zone[k][z] = zn
			g.edge(zn, n, elZone)
			g.leaf(zn, lfZone, uint32(z), 1)
			g.leaf(zn, lfZone, uint32(16+z), int64(sizes[z]))
		}
	}
	stackZ := g.node(MZZone)
	g.edge(stackZ, root, elStackZ)
	g.leaf(stackZ, lfZone, 5, 1)
	g.leaf(stackZ, lfZone, 16+5, int64(len(v.stack)))

	// objects
	for k := 0; k < int(v.n); k++ {
		pl := &v.players[k]
		for _, ci := range pl.bf {
			c := &v.cards[ci]
			n := g.node(MZPerm)
			g.edge(n, zone[k][0], elIn)
			e.entityLeaves(g, n, c, entZoneBf, seat, me.avail)
			g.setObj(c.obj, n)
		}
		lists := [3][]int32{pl.hand, pl.grave, pl.exile}
		for z, l := range lists {
			for _, ci := range l {
				c := &v.cards[ci]
				n := g.node(MZCard)
				g.edge(n, zone[k][1+z], elIn)
				zc := -1
				if z == 0 {
					zc = entZoneHand
				}
				e.entityLeaves(g, n, c, zc, seat, me.avail)
				g.setObj(c.obj, n)
			}
		}
	}
	// the stack, bottom first
	var snode [dirac.MaxStack]int
	for i := range v.stack {
		se := &e.kp.Stack[i]
		sv := &v.stack[i]
		n := g.node(MZStack)
		snode[i] = n
		g.edge(n, stackZ, elIn)
		g.leaf(n, lfStack, 0, int64(len(v.stack)-1-i))
		g.leaf(n, lfStack, 1+uint32(se.Kind), 1)
		if sv.ctrl == seat {
			g.leaf(n, lfStack, 8, 1)
		}
		if sv.name != nil && len(sv.name.name) > 0 {
			g.leaf(n, lfRow, uint32(sv.name.entName), 1)
		}
		if se.Kind == 0 {
			if sv.cv >= 0 {
				c := &v.cards[sv.cv]
				e.entityLeaves(g, n, c, entZoneStack, seat, me.avail)
				g.setObj(c.obj, n)
			}
		} else if src := g.objNode(uint8(se.Source)); src >= 0 {
			g.edge(src, n, elSrc)
		}
	}
	// cross edges: attachments, blocks, stack targets
	for k := 0; k < int(v.n); k++ {
		for _, ci := range v.players[k].bf {
			c := &v.cards[ci]
			n := g.objNode(c.obj)
			if h := g.objNode(c.attached); h >= 0 && h != n {
				g.edge(n, h, elAttach)
			}
			if a := g.objNode(c.blocks); a >= 0 && a != n {
				g.edge(n, a, elBlocks)
			}
		}
	}
	for i := range v.stack {
		se := &e.kp.Stack[i]
		add := func(t uint8) {
			switch {
			case dirac.TCode(t) == dirac.NoTarget:
			case dirac.TCode(t) >= dirac.PlayerTarget0:
				if p := int(dirac.TCode(t) - dirac.PlayerTarget0); p < 2 {
					g.edge(pnode[p], snode[i], elTPlayer)
				}
			default:
				if o := g.objNode(t); o >= 0 && o != snode[i] {
					g.edge(o, snode[i], elTarget)
				}
			}
		}
		for _, t := range se.Targets {
			add(uint8(t))
		}
		for j := 0; j < int(se.NExtra); j++ {
			add(uint8(se.Extra[j]))
		}
	}
	// options: the record's used ones, in decision order
	for j := 0; j < int(d.N); j++ {
		if !used[j] {
			continue
		}
		a := d.Options[j]
		o := e.optionOf(d, a)
		n := g.node(MZAbility)
		op := &opts[j]
		for k := 0; k < int(op.NSlots); k++ {
			g.leaf(n, lfOSlot, uint32(op.Slots[k].Row), int64(math.Round(float64(op.Slots[k].Value))))
		}
		for k := 0; k < int(op.NHashed); k++ {
			g.leaf(n, lfOHash, uint32(op.Hashed[k].Row), int64(math.Round(float64(op.Hashed[k].Value))))
		}
		for k, x := range op.Dense {
			if x != 0 {
				g.leaf(n, lfODense, uint32(k), int64(math.Round(float64(x*4))))
			}
		}
		if bot[j] {
			g.leaf(n, lfOBot, 0, 1)
		}
		parent, isObj := pnode[dirac.SeatIx(seat)], false
		if o.player && a.Target() >= dirac.PlayerTarget0 && int(a.Target()-dirac.PlayerTarget0) < int(v.n) {
			parent = pnode[a.Target()-dirac.PlayerTarget0]
		} else if on := g.objNode(o.obj); on >= 0 {
			parent, isObj = on, true
		}
		g.edge(n, parent, elOpt)
		if isObj {
			g.edge(parent, n, elOptSrc)
		}
		if r := g.objNode(o.attacker); r >= 0 {
			g.edge(r, n, elRel)
		}
		if r := g.objNode(o.battle); r >= 0 {
			g.edge(r, n, elRel)
		}
		pass := uint8(0)
		if a.Kind() == dirac.AkPass {
			pass = 1
		}
		g.optN = append(g.optN, uint16(n))
		g.optP = append(g.optP, pass)
	}

	le := binary.LittleEndian
	dst = le.AppendUint32(dst, mzMagic)
	dst = le.AppendUint32(dst, uint32(seq))
	dst = append(dst, d.Player, uint8(d.Kind))
	dst = le.AppendUint16(dst, uint16(len(g.optN)))
	dst = le.AppendUint16(dst, uint16(len(g.typ)))
	dst = le.AppendUint16(dst, uint16(len(g.ec)))
	dst = append(dst, g.typ...)
	for _, x := range g.id {
		dst = le.AppendUint32(dst, x)
	}
	for _, x := range g.val {
		dst = le.AppendUint16(dst, uint16(x))
	}
	for _, x := range g.ec {
		dst = le.AppendUint16(dst, x)
	}
	for _, x := range g.ep {
		dst = le.AppendUint16(dst, x)
	}
	dst = append(dst, g.el...)
	for _, x := range g.optN {
		dst = le.AppendUint16(dst, x)
	}
	return append(dst, g.optP...)
}
