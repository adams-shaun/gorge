package feat

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac"
)

// parts.go is the state encoding as parts, for an evaluator that sums
// rather than materialises: the network reads the sparse bag only through
// its sum of table rows (policynet's bag: Σ value·table[row], linear), and
// each entity card only through its card vector. EncodeParts hands over
// the same multiset of sparse entries EncodeState produces -- each listed
// object's value-1 rows, cached per object and versioned, plus the entries
// computed fresh -- and the entity cards in entity order, each cached card
// with its version, so the evaluator keeps one row sum and one card vector
// per (seat, object, version) and recomputes only what changed.
//
// The partition is inc.go's (EncodeStateInc, asserted equal to the full
// encoder in lockstep): an object's value-1 rows are a pure function of its
// signature (objSig); counter rows, castability rows, the stack and the
// aggregates are fresh. The entries are not sorted -- a sum needs no order
// -- so the network's input is equal up to float summation order
// (TestPartsMatchFull checks the bag and the logits).

// EntCastable is the entity raw index of the castable bit, which a hand
// card's cached Card leaves 0: the caller sets it from PartEnt.Castable.
const EntCastable = entCastable

// PartObj is one listed object's cached value-1 sparse rows.
type PartObj struct {
	Obj  dirac.Obj
	Idx  uint16 // the cache slot holding them (< PartSlots): a caller's per-slot cache checks Ver
	Ver  uint32 // changes whenever Rows do (per seat); one version names one content
	Rows []Feature
}

// PartEnt is one entity card, in entity order.
type PartEnt struct {
	Group    uint8
	Obj      dirac.Obj
	Idx      uint16 // the cache slot holding it (PartObj.Idx)
	Ver      uint32 // 0: a card computed for this encode only (a stack card)
	Castable bool   // a hand card's castable bit (raw[EntCastable])
	Card     *Card
}

// PartSlots bounds PartObj.Idx and PartEnt.Idx.
const PartSlots = pcSets*pcWays + partOvf

// Parts is one encode's parts (views of the encoder's storage, valid until
// its next encode for the same seat).
type Parts struct {
	Dense [DenseWidth]float32
	Objs  []PartObj
	Fresh []Feature
	Ents  []PartEnt
	// Ext is the extension block (SetExt), empty when off
	Ext []Feature
}

// partCache is one content's cached parts for one seat.
type partCache struct {
	ver   uint32
	idx   uint16    // set·pcWays + way
	big   int16     // the big row buffer it holds (-1 none)
	group uint8     // its entity card's group
	ent   *Card     // its entity card (partWays.ents)
	small []Feature // its own row buffer (partRowsSmall)
	rows  []Feature // small or a big buffer
}

// The parts cache is set-associative over content: pcSets sets of pcWays
// parts, a part's set chosen by its key's top bits (partKey). Each part
// keeps partRowsSmall rows of its own; a longer list (an every-creature-
// type line spells ~360) borrows one of partBig shared buffers, taken
// round-robin from the part that held it.
const (
	pcSetBits     = 9
	pcSets        = 1 << pcSetBits
	pcWays        = 4
	partRowsSmall = 64
	partBig       = 32
	partOvf       = 16 // parts an encode builds beside the cache (a set full of listed parts)
)

// partWays is one set: key 0 marks an empty way; the keys and use stamps
// sit together so a lookup reads one cache line.
type partWays struct {
	key  [pcWays]uint64
	use  [pcWays]uint32
	w    [pcWays]partCache
	ents [pcWays]Card // read on a rebuild and by a caller computing a card vector
}

type partsSeat struct {
	set      [pcSets]partWays
	use      uint32
	ver      uint32
	big      [partBig][]Feature
	bigOwner [partBig]int32 // set<<2 | way of the part holding it (-1 none)
	bigNext  int
	start    uint32 // use at the current encode's start: a way used since holds a listed part
	ovf      [partOvf]partCache
	ovfEnts  [partOvf]Card
	novf     int
	stack    [dirac.MaxStack]Card
	p        Parts
}

func newPartsSeat() *partsSeat {
	ps := &partsSeat{}
	arena := make([]Feature, pcSets*pcWays*partRowsSmall)
	for i := range ps.set {
		for j := range ps.set[i].w {
			w := &ps.set[i].w[j]
			w.small, w.big, w.idx = arena[:0:partRowsSmall], -1, uint16(i*pcWays+j)
			w.ent = &ps.set[i].ents[j]
			arena = arena[partRowsSmall:]
		}
	}
	ovfArena := make([]Feature, partOvf*maxCardRows)
	for i := range ps.ovf {
		w := &ps.ovf[i]
		w.small, w.big, w.idx = ovfArena[:0:maxCardRows], -1, uint16(pcSets*pcWays+i)
		w.ent = &ps.ovfEnts[i]
		ovfArena = ovfArena[maxCardRows:]
	}
	bigArena := make([]Feature, partBig*maxCardRows)
	for i := range ps.big {
		ps.big[i] = bigArena[:0:maxCardRows]
		bigArena = bigArena[maxCardRows:]
		ps.bigOwner[i] = -1
	}
	ps.p = Parts{
		Objs:  make([]PartObj, 0, dirac.MaxObj),
		Fresh: make([]Feature, 0, 1024),
		Ents:  make([]PartEnt, 0, dirac.MaxObj+dirac.MaxStack),
	}
	return ps
}

// setRows stores rows as the rows of part w (set si, way wi).
func (ps *partsSeat) setRows(si, wi int, w *partCache, rows []Feature) {
	if w.big >= 0 {
		ps.bigOwner[w.big] = -1
		w.big = -1
	}
	if len(rows) <= partRowsSmall {
		w.rows = append(w.small[:0], rows...)
		return
	}
	b := ps.bigNext
	for n := 0; n < partBig; n++ {
		// not one a part this encode listed holds
		if o := ps.bigOwner[b]; o < 0 || ps.set[o>>2].use[o&3] < ps.start {
			break
		}
		b = (b + 1) % partBig
	}
	ps.bigNext = (b + 1) % partBig
	if o := ps.bigOwner[b]; o >= 0 {
		// the part holding it is dropped
		ow := &ps.set[o>>2]
		ow.key[o&3] = 0
		ow.w[o&3].big, ow.w[o&3].rows = -1, nil
	}
	ps.bigOwner[b], w.big = int32(si<<2|wi), int16(b)
	w.rows = append(ps.big[b][:0], rows[:min(len(rows), maxCardRows)]...)
}

// PartsStats counts EncodeParts' work.
type PartsStats struct {
	Encodes, Rebuilt, Reused int
	ByRole                   [8]int
	Changed, Live, Listed    int // diagnostics: objects the kernel marked changed since the base, encodes with derived characteristics live, objects listed
}

// partKey keys an object's parts by what they read: its card view (its
// content key), its role, and the per-encode facts its signature adds
// (objSig: the host it is attached to and whether the seat controls it,
// how many permanents are attached to it, the blocked and declaration
// bits).
func (e *Encoder) partKey(c *vcard, role uint8, seat uint8) uint64 {
	f := uint64(role) | uint64(seat)<<8
	if role == roleBfMe || role == roleBfOpp {
		f |= uint64(e.v.hosts[c.obj]) << 16
	}
	if c.blocked {
		f |= 1 << 24
	}
	if c.attacking {
		f |= 1 << 25
	}
	if c.blocks != noObj {
		f |= 1 << 26
	}
	k := mix64(c.key, f)
	k = mix64(k, c.rel.stamp())
	if (role == roleBfMe || role == roleBfOpp) && c.attached != noObj {
		k = mix64(k, 0x686f7374)
		if h := e.bfCard(c.attached); h != nil {
			k = mix64(k, h.key)
			if h.ctrl == seat {
				k = mix64(k, 1)
			}
		}
	}
	return k | 1 // 0 marks an empty way
}

// EncodeParts encodes s at d from seat's view as parts.
func (e *Encoder) EncodeParts(s *dirac.State, d *dirac.Decision, seat uint8) *Parts {
	seat, me, opp := e.prelude(s, d, seat)
	if e.parts[dirac.SeatIx(seat)] == nil {
		e.parts[dirac.SeatIx(seat)] = newPartsSeat()
	}
	ps := e.parts[dirac.SeatIx(seat)]
	p := &ps.p
	p.Dense = e.st.Dense
	ps.start, ps.novf = ps.use+1, 0
	p.Objs, p.Fresh, p.Ents = p.Objs[:0], p.Fresh[:0], p.Ents[:0]
	e.PartsStats.Encodes++
	if e.hbase != nil {
		for _, w := range s.ChangedMask() {
			e.PartsStats.Changed += bits.OnesCount64(w)
		}
	}
	if e.ctx != 0 {
		e.PartsStats.Live++
	}
	e.PartsStats.Listed += len(e.v.cards)
	v := &e.v
	refresh := func(ci int32, role uint8) *partCache {
		c := &v.cards[ci]
		key := e.partKey(c, role, seat)
		si := int(key >> (64 - pcSetBits))
		ws := &ps.set[si]
		ps.use++
		j, lru := -1, 0
		for i := range ws.key {
			if ws.key[i] == key {
				j = i
				break
			}
			if ws.use[i] < ws.use[lru] {
				lru = i
			}
		}
		fresh := j < 0
		var oc *partCache
		ovf := false
		if fresh && ws.key[lru] != 0 && ws.use[lru] >= ps.start {
			// every way holds a part this encode listed: build this one
			// beside the cache, for this encode only
			oc, ovf = &ps.ovf[ps.novf%partOvf], true
			ps.novf++
		} else {
			if fresh {
				j = lru
				ws.key[j] = key
			}
			ws.use[j] = ps.use
			oc = &ws.w[j]
		}
		if !fresh {
			e.PartsStats.Reused++
		} else {
			e.PartsStats.Rebuilt++
			e.PartsStats.ByRole[role]++
			ps.ver++
			oc.ver = ps.ver
			e.sp = e.sp[:0]
			switch role {
			case roleBfMe, roleBfOpp:
				tag, zi := ztBattleMe, zcBfMe
				if role == roleBfOpp {
					tag, zi = ztBattleOpp, zcBfOpp
				}
				e.battle(c, tag)
				e.permRows(zi, c, false)
			case roleHand:
				e.push(c.c.nameTag[ztHand], 1)
				e.handCard(c, -1)
			case roleGraveMe:
				e.push(c.c.nameTag[ztGraveMe], 1)
			case roleGraveOpp:
				e.push(c.c.nameTag[ztGraveOpp], 1)
			case roleExileMe:
				e.push(c.c.nameTag[ztExileMe], 1)
			case roleExileOpp:
				e.push(c.c.nameTag[ztExileOpp], 1)
			}
			if ovf {
				oc.rows = append(oc.small[:0], e.sp[:min(len(e.sp), maxCardRows)]...)
			} else {
				ps.setRows(si, j, oc, e.sp)
			}
			switch role {
			case roleBfMe, roleBfOpp, roleHand:
				zone := entZoneBf
				if role == roleHand {
					zone = entZoneHand
				}
				oc.group = [...]uint8{roleBfMe: 0, roleBfOpp: 1, roleHand: 2}[role]
				*oc.ent = Card{Group: oc.group}
				e.entityCard(oc.ent, c, zone, seat, -1)
			}
		}
		p.Objs = append(p.Objs, PartObj{Obj: c.obj, Idx: oc.idx, Ver: oc.ver, Rows: oc.rows})
		e.slot[c.obj] = oc
		return oc
	}
	for _, ci := range me.hand {
		refresh(ci, roleHand)
	}
	for _, ci := range me.bf {
		refresh(ci, roleBfMe)
	}
	for _, ci := range opp.bf {
		refresh(ci, roleBfOpp)
	}
	for _, ci := range me.grave {
		refresh(ci, roleGraveMe)
	}
	for _, ci := range opp.grave {
		refresh(ci, roleGraveOpp)
	}
	for _, ci := range me.exile {
		refresh(ci, roleExileMe)
	}
	for _, ci := range opp.exile {
		refresh(ci, roleExileOpp)
	}
	// the fresh entries (inc.go's): the v1 stack tags, the counter rows,
	// the castability rows, the mz stack rows and the aggregates
	e.sp = e.sp[:0]
	for i := range v.stack {
		e.push(v.stack[i].name.nameTag[ztStack], 1)
	}
	var mine, theirs agg
	for zi, l := range [2][]int32{me.bf, opp.bf} {
		zc := &zctxs[zi]
		for _, ci := range l {
			c := &v.cards[ci]
			if c.nctr > 0 {
				for _, k := range counterOrder {
					if c.ctrListed&(1<<k) != 0 {
						e.h(addB(zc.ctr, e.pool.ctr[k]), float32(c.ctr[k]))
					}
				}
			}
			if zi == 0 {
				mine.add(c)
			} else {
				theirs.add(c)
			}
		}
	}
	for _, ci := range me.hand {
		c := &v.cards[ci]
		if !c.c.land && c.c.mv <= me.avail {
			e.push(rowHandCastable, 1)
			for _, r := range c.c.castRows {
				e.push(r, 1)
			}
			if c.c.instantType || c.hasFlash() {
				e.push(rowHandCastI, 1)
			}
		}
	}
	e.mzStack(seat)
	e.mzGlobal(seat, me, opp, &mine, &theirs)
	p.Fresh = append(p.Fresh, e.sp...)
	// entity cards in entity order
	e.gen++
	if e.gen == 0 {
		e.seen = [objSlots]uint32{}
		e.gen = 1
	}
	for li, l := range [3][]int32{me.bf, opp.bf, me.hand} {
		for _, ci := range l {
			c := &v.cards[ci]
			if e.seen[c.obj] == e.gen {
				continue
			}
			e.seen[c.obj] = e.gen
			oc := e.slot[c.obj]
			pe := PartEnt{Group: oc.group, Obj: c.obj, Idx: oc.idx, Ver: oc.ver, Card: oc.ent}
			if li == 2 {
				pe.Castable = !c.c.land && c.c.mv <= me.avail
			}
			p.Ents = append(p.Ents, pe)
		}
	}
	ns := 0
	for i := range v.stack {
		ci := v.stack[i].cv
		if ci < 0 {
			continue
		}
		c := &v.cards[ci]
		if e.seen[c.obj] == e.gen {
			continue
		}
		e.seen[c.obj] = e.gen
		sc := &ps.stack[ns]
		ns++
		*sc = Card{Group: 3}
		e.entityCard(sc, c, entZoneStack, seat, me.avail)
		p.Ents = append(p.Ents, PartEnt{Group: 3, Obj: c.obj, Card: sc})
	}
	p.Ext = e.extFor(seat, me, opp)
	return p
}
