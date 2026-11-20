package dirac

import (
	"encoding/binary"
	"math/bits"
	"unsafe"
)

// The failed-search memo of filtScript. One priority walk asks a play's
// payment search up to three times for the same game: the real walk, then
// scriptOffers' two plain re-walks (plainGapsOptions, mergeBareGaps), each
// under its own availFor mode. The search is deterministic in the game, and
// the three modes differ only in bits the search never reads (see
// filtMemoMode), so a verdict "no script" proved once stands for the rest of
// the transition. The memo is a fixed array in the ForkPool, keyed by the
// whole game's hash, so an entry can only answer for a game that is the same
// down to the last byte; it is emptied at the transition boundary (stepRaw,
// Apply) and nothing outlives a transition.

// filtMemoMax is the failed verdicts one transition remembers; a full memo
// overwrites its oldest entry.
const filtMemoMax = 32

// filtMemoEnt is one remembered failure: play a of player p, searched under
// the read availX bits av, on the game hashing to h.
type filtMemoEnt struct {
	h  uint64
	a  Action
	av uint16
	p  uint8
}

// filtMemo is the pool's failed-verdict table. hits and notes count over the
// pool's life (tests read them); only n and next are transition state.
type filtMemo struct {
	n, next     uint8
	ent         [filtMemoMax]filtMemoEnt
	hits, notes uint32
}

// DebugFiltNoMemo turns the memo off: every search runs (mutation testing and
// the memo's equivalence test only).
var DebugFiltNoMemo bool

// DebugFiltMemoCheck makes a memo hit run its search anyway and panic when it
// finds a script (the equivalence test only).
var DebugFiltMemoCheck bool

// filtMemoMode is the availX bits a filter search does not read: availGaps and
// availNoFilt (the offer census's modes) and availPlain, which the search
// reads only beside a live granted mana ability (noGrant's walks) -- a table
// with grants keeps the bit in the key. Every other bit (availWalk, availHyp,
// availAny, the window and life modes, availNoActScript) stays in the key, so a
// walk in one of those modes never answers for another.
func filtMemoMode(k *prep) uint16 {
	m := availGaps | availNoFilt | availPlain
	if k.gen == nil || len(k.gen.grantPC) != 0 {
		m &^= availPlain
	}
	return m
}

// filtMemoProbe is filtScript's lookup. live reports that the search's answer
// may be remembered (a play probe on the real game: no apply, no fork
// outstanding, no search or census window running); hit that this game
// already failed it.
type filtMemoProbe struct {
	ent       filtMemoEnt
	live, hit bool
}

func (s *State) filtMemoLookup(k *prep, a Action, p uint8, apply bool) (m filtMemoProbe) {
	pl := s.pool
	if apply || pl == nil || pl.depth != 0 || pl.inFilt != 0 || pl.census || DebugFiltNoMemo {
		return m
	}
	av := s.g.availX
	s.g.availX = 0 // the mode is in the key, not the hash
	m.ent = filtMemoEnt{h: s.gameHash(), a: a, av: av &^ filtMemoMode(k), p: p}
	s.g.availX = av
	m.live = true
	for i := 0; i < int(pl.fmemo.n); i++ {
		if pl.fmemo.ent[i] == m.ent {
			pl.fmemo.hits++
			m.hit = true
			break
		}
	}
	return m
}

// filtMemoNote remembers the failure m was probed for.
func (s *State) filtMemoNote(m *filtMemoProbe) {
	if !m.live || m.hit {
		return
	}
	fm := &s.pool.fmemo
	i := fm.next
	if fm.n < filtMemoMax {
		fm.n++
	}
	fm.ent[i] = m.ent
	fm.next = (i + 1) % filtMemoMax
	fm.notes++
}

// filtMemoReset empties the memo: the game is about to change.
func (s *State) filtMemoReset() {
	if p := s.pool; p != nil {
		p.fmemo.n, p.fmemo.next = 0, 0
	}
}

// gameHash hashes the game: stateCore, the used perms and the cold arrays.
// Four lanes of a multiplicative hash over the 8-byte words; two games that
// differ in one word always differ in the hash. Padding bytes may differ
// between equal games, which only costs a miss.
func (s *State) gameHash() uint64 {
	h := hashBytes(0x9e3779b97f4a7c15, unsafe.Slice((*byte)(unsafe.Pointer(&s.stateCore)), unsafe.Sizeof(s.stateCore)))
	if s.permHi != 0 {
		h = hashBytes(h, unsafe.Slice((*byte)(unsafe.Pointer(&s.perms[0])), int(s.permHi)*int(unsafe.Sizeof(perm{}))))
	}
	if len(s.cold.buf) != 0 {
		h = hashBytes(h, unsafe.Slice((*byte)(unsafe.Pointer(&s.cold.buf[0])), 8*len(s.cold.buf)))
	}
	return h
}

func hashBytes(seed uint64, b []byte) uint64 {
	const m0, m1, m2, m3 = 0x9e3779b97f4a7c15, 0xc2b2ae3d27d4eb4f, 0x165667b19e3779f9, 0x27d4eb2f165667c5
	h0, h1, h2, h3 := seed, seed^m1, seed^m2, seed^m3
	n := len(b)
	for ; len(b) >= 32; b = b[32:] {
		h0 = (h0 ^ binary.LittleEndian.Uint64(b)) * m0
		h1 = (h1 ^ binary.LittleEndian.Uint64(b[8:])) * m1
		h2 = (h2 ^ binary.LittleEndian.Uint64(b[16:])) * m2
		h3 = (h3 ^ binary.LittleEndian.Uint64(b[24:])) * m3
	}
	for ; len(b) >= 8; b = b[8:] {
		h0 = bits.RotateLeft64(h0, 23) ^ binary.LittleEndian.Uint64(b)*m1
	}
	for _, c := range b {
		h1 = (h1 ^ uint64(c)) * m2
	}
	h := bits.RotateLeft64(h0, 11) ^ bits.RotateLeft64(h1, 27) ^ bits.RotateLeft64(h2, 41) ^ h3 ^ uint64(n)
	h ^= h >> 32
	h *= m0
	h ^= h >> 29
	return h
}

// takePaySeen returns filtPay's visited table: the pool's own, kept across
// searches (a pay search's table is paySearchNodes long, ~90 KB), or a fresh
// one when a search on the same pool already holds it. release is the func
// that gives it back.
func (p *ForkPool) takePaySeen() (tbl []filtKey, own bool) {
	if p.payBusy {
		return make([]filtKey, paySearchNodes), false
	}
	if p.paySeen == nil {
		p.paySeen = make([]filtKey, paySearchNodes)
	}
	p.payBusy = true
	return p.paySeen, true
}
