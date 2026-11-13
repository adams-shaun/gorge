package feat

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// inc.go is the incremental state encoder: EncodeStateInc produces exactly
// EncodeState's output (the lockstep asserts it at every decision) while
// regenerating only the cards whose encoding inputs changed.
//
// Every card's contribution is a pure function of a small signature (its
// card, role, battlefield status, P/T, counters, keywords, attachment and
// the few board facts it reads); the encoder caches, per seat and object,
// the value-1 sparse rows and the entity card the signature produced, and
// rebuilds an object only when its signature differs. Nothing is keyed on
// the order states are encoded in, so a search that jumps between sibling
// worlds hits the cache exactly as a linear game does, and the kernel needs
// no dirty-tracking hook.
//
// The sparse bag is kept as a per-row count of value-1 entries plus an
// occupancy bitset; the entries whose value is not 1 (counter rows, the
// numeric aggregates) are emitted fresh in policynet's emission order and
// merged in. A row holding both kinds would need the emission order between
// them; that state is encoded by the full rebuild instead (Stats.Fallback).

// Card roles (the zone list a card is listed in, from the seat's side).
const (
	roleNone uint8 = iota
	roleBfMe
	roleBfOpp
	roleHand
	roleGraveMe
	roleGraveOpp
	roleExileMe
	roleExileOpp
)

// objSig is everything one listed card's encoding reads.
type objSig struct {
	card      kop.CardID
	role      uint8
	flags     uint8 // tapped, sick, attacking, blocked, blocking, printedKw, attachMine, castable
	host      uint8 // the attached-to card (255 none, 254 attached to something off the battlefield)
	hosts     uint8
	damage    int32
	power     int32
	tough     int32
	kw        kop.KwMask
	kwRep     dirac.FeatKwReps
	ctr       [dirac.FeatCtrKinds]uint16
	ctrListed uint16
	entZoneOK bool   // listed in the entity walk
	rel       uint64 // the card's relational entity slots
	// the pool entries its rows read the names of: its own and its
	// host's (an override can rename either, FeatNameCard, without the
	// card id changing)
	cc, hostc *card
	tyv       *typeView // a derived type line
}

const maxCardRows = 512 // an "all creature types" line alone spells ~360 type rows

// objCache is one object's cached contribution.
type objCache struct {
	sig   objSig
	valid bool
	nrows uint16
	rows  [maxCardRows]uint16 // value-1 sparse rows
	ent   Card
	ver   uint32 // bumped on every rebuild
	gen   uint32 // the encode that last listed it
}

// seatInc is one seat's incremental state.
type seatInc struct {
	obj   [dirac.MaxObj]objCache
	cnt   [TableRows]uint16
	occ   [TableRows / 64]uint64 // the rows with cnt > 0
	fresh []Feature              // this encode's global entries, stably sorted by row
	gen   uint32
	live  []uint8 // the objects with a valid cache
	// entPos[i] is (obj, ver) of the entity card at output position i.
	entObj []uint8
	entVer []uint32
	cards  []Card
	sp     []Feature
	st     State
}

// IncStats counts the incremental encoder's work.
type IncStats struct {
	Encodes, Rebuilt, Reused, Fallback int
	BaseBuilds                         int // views built against the base (its objects' hashes)
	CardHits, CardBuilds               int // card views from the content cache; built on its misses
	BuildsZ                            [5]int
}

func (si *seatInc) addRow(r uint16) {
	if si.cnt[r] == 0 {
		si.occ[r>>6] |= 1 << (r & 63)
	}
	si.cnt[r]++
}

func (si *seatInc) delRow(r uint16) {
	si.cnt[r]--
	if si.cnt[r] == 0 {
		si.occ[r>>6] &^= 1 << (r & 63)
	}
}

// EncodeStateInc is EncodeState, incrementally. The returned state is valid
// until the next encode for the same seat.
func (e *Encoder) EncodeStateInc(s *dirac.State, d *dirac.Decision) *State {
	return e.EncodeStateIncFor(s, d, d.Player)
}

// EncodeStateIncFor is EncodeStateFor, incrementally.
func (e *Encoder) EncodeStateIncFor(s *dirac.State, d *dirac.Decision, seat uint8) *State {
	seat, me, opp := e.prelude(s, d, seat)
	if e.inc[dirac.SeatIx(seat)] == nil {
		e.inc[dirac.SeatIx(seat)] = &seatInc{fresh: make([]Feature, 0, 256),
			sp: make([]Feature, 0, 512), cards: make([]Card, 0, 64)}
	}
	si := e.inc[dirac.SeatIx(seat)]
	si.gen++
	e.IncStats.Encodes++
	v := &e.v
	si.st.Dense = e.st.Dense
	// Per-card rows, entities: refresh every listed card's cache.
	refresh := func(ci int32, role uint8) {
		c := &v.cards[ci]
		oc := &si.obj[c.obj]
		sig := e.sigOf(c, role, seat, me.avail)
		oc.gen = si.gen
		if oc.valid && oc.sig == sig {
			e.IncStats.Reused++
			return
		}
		e.IncStats.Rebuilt++
		if oc.valid {
			for _, r := range oc.rows[:oc.nrows] {
				si.delRow(r)
			}
		}
		if !oc.valid {
			si.live = append(si.live, c.obj)
		}
		oc.sig, oc.valid = sig, true
		oc.ver++
		// Generate this card's value-1 rows through the full encoder's own
		// emitters (counter rows excluded: they are emitted fresh).
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
			// castability depends on the seat's mana: its rows are fresh
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
		oc.nrows = uint16(min(len(e.sp), maxCardRows))
		for i := 0; i < int(oc.nrows); i++ {
			oc.rows[i] = e.sp[i].Row
			si.addRow(e.sp[i].Row)
		}
		switch role {
		case roleBfMe, roleBfOpp, roleHand:
			zone := entZoneBf
			if role == roleHand {
				zone = entZoneHand
			}
			oc.ent = Card{Group: [...]uint8{roleBfMe: 0, roleBfOpp: 1, roleHand: 2}[role]}
			e.entityCard(&oc.ent, c, zone, seat, -1) // castable is set at placement
		}
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
	// Objects no longer listed leave the bag.
	k := 0
	for _, o := range si.live {
		oc := &si.obj[o]
		if oc.gen != si.gen {
			for _, r := range oc.rows[:oc.nrows] {
				si.delRow(r)
			}
			oc.valid = false
			continue
		}
		si.live[k] = o
		k++
	}
	si.live = si.live[:k]
	// This encode's global rows: the v1 stack tags, the mz stack rows, the
	// aggregates; and the counter rows.
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
	// The fresh entries stably by row. A row a card's cached value-1 rows
	// also hold, with a fresh entry whose value is not 1, needs the full
	// emission order between them: full rebuild.
	fr := append(si.fresh[:0], e.sp...)
	stableSortByRow(fr, &e.sorts)
	si.fresh = fr
	for _, f := range fr {
		if f.Value != 1 && si.cnt[f.Row] > 0 {
			e.IncStats.Fallback++
			st := e.fullTail(seat, me, opp)
			si.st.Sparse = append(si.sp[:0], st.Sparse...)
			si.sp = si.st.Sparse
			si.st.Cards = append(si.cards[:0], st.Cards...)
			si.cards = si.st.Cards
			si.entObj, si.entVer = si.entObj[:0], si.entVer[:0]
			for range si.cards {
				si.entObj = append(si.entObj, noObj)
				si.entVer = append(si.entVer, 0)
			}
			si.st.Ext = e.extFor(seat, me, opp)
			return &si.st
		}
	}
	// The sorted bag: the cards' rows (each repeated by its count) merged
	// with the fresh entries, a card row first on a tie.
	sp := si.sp[:0]
	fi := 0
	for w := range si.occ {
		word := si.occ[w]
		for word != 0 {
			r := uint16(w<<6 | trailing(word))
			word &= word - 1
			for fi < len(fr) && fr[fi].Row < r {
				sp = append(sp, fr[fi])
				fi++
			}
			for n := si.cnt[r]; n > 0; n-- {
				sp = append(sp, Feature{Row: r, Value: 1})
			}
		}
	}
	sp = append(sp, fr[fi:]...)
	si.sp = sp
	si.st.Sparse = sp
	// Entity cards in entity order: my battlefield, the opponent's, my hand,
	// then the stack's cards (never cached: few, and their view is the
	// stack entry's). A position is copied only when its object or that
	// object's cache version changed.
	e.gen++
	if e.gen == 0 {
		e.seen = [256]uint32{}
		e.gen = 1
	}
	n := 0
	place := func(o uint8, src *Card, ver uint32) {
		if n < len(si.entObj) {
			if si.entObj[n] != o || si.entVer[n] != ver || ver == 0 {
				si.cards[n] = *src
				si.entObj[n], si.entVer[n] = o, ver
			}
		} else {
			si.cards = append(si.cards, *src)
			si.entObj = append(si.entObj, o)
			si.entVer = append(si.entVer, ver)
		}
		n++
	}
	for li, l := range [3][]int32{me.bf, opp.bf, me.hand} {
		for _, ci := range l {
			c := &v.cards[ci]
			if e.seen[c.obj] == e.gen {
				continue
			}
			e.seen[c.obj] = e.gen
			oc := &si.obj[c.obj]
			place(c.obj, &oc.ent, oc.ver)
			if li == 2 {
				var cast float32
				if !c.c.land && c.c.mv <= me.avail {
					cast = 1
				}
				si.cards[n-1].Raw[entCastable] = cast
			}
		}
	}
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
		var sc Card
		sc.Group = 3
		e.entityCard(&sc, c, entZoneStack, seat, me.avail)
		place(c.obj, &sc, 0)
	}
	si.cards = si.cards[:n]
	si.entObj, si.entVer = si.entObj[:n], si.entVer[:n]
	if n == 0 {
		si.st.Cards = nil
	} else {
		si.st.Cards = si.cards
	}
	si.st.Ext = e.extFor(seat, me, opp)
	return &si.st
}

// sigOf is c's signature in role for seat (avail: the seat's available
// mana, which a hand card's castability reads).
func (e *Encoder) sigOf(c *vcard, role uint8, seat uint8, avail int32) objSig {
	sg := objSig{card: c.card(), cc: c.c, tyv: c.tyv, role: role, damage: c.damage, power: c.power, tough: c.tough, kw: c.kw, ctr: c.ctr, ctrListed: c.ctrListed, host: 255, rel: c.rel.stamp()}
	if c.kwRep != nil {
		sg.kwRep = *c.kwRep
	}
	var f uint8
	if c.tapped {
		f |= 1
	}
	if c.sick {
		f |= 2
	}
	if c.attacking {
		f |= 4
	}
	if c.blocked {
		f |= 8
	}
	if c.blocks != noObj {
		f |= 16
	}
	if c.printedKw {
		f |= 32
	}
	switch role {
	case roleBfMe, roleBfOpp:
		if c.attached != noObj {
			sg.host = 254
			if h := e.bfCard(c.attached); h != nil {
				sg.host = uint8(h.card())
				sg.hostc = h.c
				if h.ctrl == seat {
					f |= 64
				}
			}
		}
		sg.hosts = e.v.hosts[c.obj]
	}
	sg.flags = f
	return sg
}

// card is the CardID of a view card (its pool entry's index).
func (c *vcore) card() kop.CardID { return c.cid }
