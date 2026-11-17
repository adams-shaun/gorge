package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A card exiled by a dies-to-exile replacement that puts a counter on it
// (Dauthi Voidwalker's VOID, Draugr Necromancer's ICE) keeps the counter in
// exile. The kernel stores counters on battlefield permanents only
// (s.perms[].ctr), so the exiled card's counter lives in a one-byte
// per-object cold record: no hot State bytes. The record is version-keyed to
// its entry move (CR 400.7: a zone change makes a new object and bumps ver),
// so it answers only for the object that entered exile with it — a card that
// leaves exile by any path (the cast push writes zone and bumps ver
// directly, pushBuild) never lets its old record answer for a later exile of
// a new object. The kinds the dies-to-exile bodies name are the
// modelled codes, and kgen refuses any other kind or a count above the field
// (exileBody, lower_repl.go).
const maxExileCtrCount = 3

// exileCtr is one exiled card's counter record, packed in a byte so the
// region is one byte per object (a pool at the State bar cannot pay more):
// bits 0-3 are the entry ver (the object's ver just before the move that
// brought it to exile, one bump, taken mod 16), bits 4-5 the kind's index in
// exileCtrKinds, bits 6-7 the count 1..maxExileCtrCount (0: no record). The
// record answers only for the object that entered exile with it; a later
// object in exile carries a different ver and reads nothing. clearExileCounter
// zeroes it on the leave paths that run it, so the 4-bit ver only has to
// separate the leave paths that do not (the cast push).
type exileCtr uint8

func packExileCtr(ver uint8, code uint8, n uint8) exileCtr {
	return exileCtr(ver&15 | (code-1)&3<<4 | n&3<<6)
}

func (e exileCtr) ver() uint8 { return uint8(e) & 15 }

// kind is the record's counter kind (0 when the record is empty).
func (e exileCtr) kind() kop.CounterKind {
	if e.n() == 0 {
		return 0
	}
	return exileCtrKinds[uint8(e)>>4&3]
}

func (e exileCtr) n() uint8 { return uint8(e) >> 6 }

// exileCtrKinds is the code table: a byte's low nibble is 1 + the index here.
var exileCtrKinds = [...]kop.CounterKind{kop.CtrVoid, kop.CtrIce, kop.CtrBlood, kop.CtrHit}

// exileCtrCodeOf maps a counter kind to its 1..4 code (0: not modelled).
func exileCtrCodeOf(kind kop.CounterKind) uint8 {
	for i, k := range exileCtrKinds {
		if k == kind {
			return uint8(i + 1)
		}
	}
	return 0
}

// useExileCtr: some card's ReplExileOnLeave leaves a counter on the exiled
// card. A registered per-table flag (genneed.go), not a genPrep field.
var useExileCtr = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplExileOnLeave {
				continue
			}
			if kind, _ := replExileCtr(a.Repl.A); kind != 0 {
				return true
			}
		}
	}
	return false
})

// exileCtrNeed sizes the cold region: only a table whose cards include an
// exile-counter writer pays it, one record per object.
func exileCtrNeed(g *genPrep) int {
	if g.uses.has(useExileCtr) {
		return MaxObj
	}
	return 0
}

// coldExCtr is the exiled cards' counters, one record per object. Only a
// table with a dies-to-exile replacement that leaves a counter sizes it; a
// counterless ExileOnLeave table (Dauthi's reader with no writer) pays
// nothing, so the reader needs no per-card precomputation in the hot State.
var coldExCtr = newCold[exileCtr]("exCtr", exileCtrNeed)

// exileCounters reports how many counters of kind the exiled object o
// carries. A card outside exile has no exile counters.
func (s *State) exileCounters(o Obj, kind kop.CounterKind) int {
	k, n := s.exileCounterAt(o)
	if k != kind {
		return 0
	}
	return int(n)
}

// exileCounterAt is the live (kind, count) on the exiled object o, or (0, 0)
// when it carries none or is not the object the record's entry move made.
func (s *State) exileCounterAt(o Obj) (kop.CounterKind, uint8) {
	r := coldExCtr.of(s)
	if int(o) >= len(r) || s.objs[o].zone != kop.ZoneExile || s.objs[o].ver&15 != (r[o].ver()+1)&15 {
		return 0, 0
	}
	if r[o].n() == 0 {
		return 0, 0
	}
	return r[o].kind(), r[o].n()
}

// stampExileCounter records n counters of kind on o before its entry move:
// dieZone calls it while the object is still where it dies from, one bump
// ahead of its exile ver. An unmodelled kind or an out-of-range count writes
// nothing (kgen refuses those bodies).
func (s *State) stampExileCounter(o Obj, kind kop.CounterKind, n int) {
	s.stampExileCtr(o, kind, n, s.objs[o].ver)
}

// stampExileCounterMoved is stampExileCounter for a stamp made after the
// entry bump: exileAnyDest runs inside placeFromK, above the ver it stamped.
func (s *State) stampExileCounterMoved(o Obj, kind kop.CounterKind, n int) {
	s.stampExileCtr(o, kind, n, s.objs[o].ver-1)
}

func (s *State) stampExileCtr(o Obj, kind kop.CounterKind, n int, ver uint8) {
	r := coldExCtr.of(s)
	code := exileCtrCodeOf(kind)
	if int(o) >= len(r) || code == 0 || n <= 0 || n > maxExileCtrCount {
		return
	}
	r[o] = packExileCtr(ver, code, uint8(n))
}

// predCountersGECard is PredCountersGE for a card off the battlefield: a
// stash counter (Tinybones) or an exile counter a dies-to-exile rider left on
// the exiled card (Dauthi Voidwalker's VOID, Draugr Necromancer's ICE).
func (s *State) predCountersGECard(k *prep, o Obj, n int) bool {
	kind := kop.CounterKind(n & 0xff)
	if kind == kop.CtrStash && n>>8 <= 1 && o != NoObj && s.mObj(k, mechStash, o) {
		return true
	}
	return o != NoObj && s.objs[o].zone == kop.ZoneExile && s.exileCounters(o, kind) >= int(n>>8)
}

// projExileCounter writes o's exile counter into its projection.
func (s *State) projExileCounter(k *prep, o Obj, po *ProjObj) {
	if k.gen == nil || k.gen.exileOnLeave == nil || s.objs[o].zone != kop.ZoneExile {
		return
	}
	if kind, n := s.exileCounterAt(o); kind != 0 {
		projCtr(po, uint16(kind), uint16(n))
	}
}

// clearExileCounter drops o's exile-counter record when it leaves exile. The
// version check in exileCounterAt already kills the record on any move (the
// ver moves); the clear keeps a reused object id from ever meeting a record
// whose ver wrapped around (zone_objreuse.go).
func (s *State) clearExileCounter(o Obj) {
	r := coldExCtr.of(s)
	if int(o) < len(r) {
		r[o] = 0
	}
}

// exileCounterKind is Repl.A of card c's ReplExileOnLeave with filter f, the
// counter the replacement leaves on the exiled card (0 when it puts none).
// The per-card kind is read from the table, not stored in the hot State.
func exileCounterKind(t *kop.Table, c CardID, f kop.FilterID) (kop.CounterKind, uint8) {
	if t == nil || int(c) >= len(t.Cards) {
		return 0, 0
	}
	for i := range t.Cards[c].Abilities {
		ab := &t.Cards[c].Abilities[i]
		if ab.Kind == kop.AbReplacement && ab.Repl.Kind == kop.ReplExileOnLeave && ab.Repl.Filter == f && ab.Repl.A != 0 {
			return replExileCtr(ab.Repl.A)
		}
	}
	return 0, 0
}

// replExileCtr unpacks ReplExileOnLeave's A: the counter kind in the low byte
// and the count above it.
func replExileCtr(a int16) (kop.CounterKind, uint8) {
	return kop.CounterKind(uint8(a)), uint8(uint16(a) >> 8)
}
