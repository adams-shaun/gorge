package dirac

// The offer-verdict memo of filtSearch.offers. Every node of a play's prefix
// search asks whether the play is a legal option there (CR 601.2g-h: the
// floating pool and the untapped sources pay it), and the answer is a whole
// plain priority walk of the forked game. The same game recurs: the search
// re-reaches a node by a different order of taps (it dedups only after it
// priced the node, by a colour-only key), and one priority walk re-runs a
// play's search per offer and per re-walk (mana_filtmemo.go). One hosted-cmdr
// game asked 840 000 walks, 60% of them for a game already walked.
//
// The walk reads the game, the play, its player, the search root's Exhaust
// fact (withRootExhaust) and the pool's census / deferred-turn-up modes; it
// writes nothing back but availX, which it always leaves 0. An entry is keyed
// by the whole game's hash and those inputs, so it answers only for a game
// equal down to the last byte. The table is emptied at a real transition (a
// depth-0 stepRaw or Apply); a fork's own steps keep it, as the hash already
// tells their games apart.

// offersMemoSize is the direct-mapped table's entries.
const (
	offersMemoBits = 12
	offersMemoSize = 1 << offersMemoBits
)

type offersMemoEnt struct {
	h   uint64
	gen uint32
	a   Action
	key uint32 // the non-game inputs (offersMemoKey)
	ok  bool
}

// offersMemo is the pool's verdict table. gen is the live generation (0: not
// yet opened); hits counts over the pool's life (tests read it).
type offersMemo struct {
	gen  uint32
	hits uint32
	ent  [offersMemoSize]offersMemoEnt
}

// DebugOffersNoMemo turns the memo off: every node walks (the equivalence test
// and mutation testing only).
var DebugOffersNoMemo bool

// DebugOffersMemoCheck makes a hit walk anyway and panic when the walk
// disagrees (the equivalence test only); DebugOffersMemoChecked counts the
// hits it re-walked.
var (
	DebugOffersMemoCheck   bool
	DebugOffersMemoChecked int
)

// offersMemoReset starts a new generation: every entry goes stale.
func (m *offersMemo) reset() {
	m.gen++
	if m.gen == 0 {
		m.ent = [offersMemoSize]offersMemoEnt{}
		m.gen = 1
	}
}

func offersMemoKey(t *State, p uint8) uint32 {
	k := uint32(p) | uint32(t.rootTurnF()&turnExhaustMask(p))<<8
	if pl := t.pool; pl.census {
		k |= 1 << 30
	}
	if t.pool.inDeferTU != 0 {
		k |= 1 << 31
	}
	return k
}

// offersMemoSlot is the table index of play a on the game hashing to h. The
// product's top bits depend on every bit of both.
func offersMemoSlot(h uint64, a Action) uint64 {
	play := uint64(a.Kind())<<40 | uint64(a.Source())<<24 | uint64(a.Target())<<8 | uint64(a.Mode())
	return (h ^ play) * 0x9e3779b97f4a7c15 >> (64 - offersMemoBits)
}

// offersMemoed is offers through the memo.
func (fs *filtSearch) offersMemoed(k *prep, t *State) bool {
	pl := t.pool
	if pl == nil || DebugOffersNoMemo {
		return fs.offers(k, t)
	}
	m := &pl.offm
	if m.gen == 0 {
		m.gen = 1
	}
	h := t.gameHash()
	key := offersMemoKey(t, fs.p)
	e := &m.ent[offersMemoSlot(h, fs.a)]
	if e.gen == m.gen && e.h == h && e.a == fs.a && e.key == key {
		m.hits++
		if DebugOffersMemoCheck {
			DebugOffersMemoChecked++
			if fs.offers(k, t) != e.ok {
				panic("dirac: offers memo answered a verdict the walk does not give")
			}
		}
		t.g.availX = 0 // as the walk leaves it (plainOptions)
		return e.ok
	}
	ok := fs.offers(k, t)
	*e = offersMemoEnt{h: h, gen: m.gen, a: fs.a, key: key, ok: ok}
	return ok
}
