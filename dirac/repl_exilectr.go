package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A card exiled with a counter on it (Dauthi Voidwalker's VOID, Draugr
// Necromancer's ICE from a dies-to-exile replacement; Grolnok's CROAK and
// Etrata's HIT from an "exile it with a counter" move) keeps the counter in
// exile. The kernel stores counters on battlefield permanents only
// (s.perms[].ctr), so the exiled card's counter lives in a two-byte
// per-object cold record: no hot State bytes. The record is version-keyed to
// its entry move (CR 400.7: a zone change makes a new object and bumps ver),
// so it answers only for the object that entered exile with it — a card that
// leaves exile by any path (the cast push writes zone and bumps ver
// directly, pushBuild) never lets its old record answer for a later exile of
// a new object. The modelled kinds are the exileCtrKinds table; kgen refuses
// a dies-to-exile body with any other kind or a count above 3 (exileBody,
// lower_repl.go), and an exile move's count past maxExileCtrCount is a
// capacity stall (stampExileMoveCounter).
const maxExileCtrCount = 15

// exileCtr is one exiled card's counter record, packed in two bytes: bits 0-7
// are the kind's 1-based index in exileCtrKinds, bits 8-11 the entry ver (the
// object's ver just before the move that brought it to exile, one bump, taken
// mod 16), bits 12-15 the count 1..maxExileCtrCount (0: no record). The
// record answers only for the object that entered exile with it; a later
// object in exile carries a different ver and reads nothing. clearExileCounter
// zeroes it on the leave paths that run it, so the 4-bit ver only has to
// separate the leave paths that do not (the cast push).
type exileCtr uint16

func packExileCtr(ver uint8, code uint8, n uint8) exileCtr {
	return exileCtr(code) | exileCtr(ver&15)<<8 | exileCtr(n&15)<<12
}

func (e exileCtr) ver() uint8 { return uint8(e >> 8 & 15) }

// kind is the record's counter kind (0 when the record is empty).
func (e exileCtr) kind() kop.CounterKind {
	code := int(e & 0xff)
	if e.n() == 0 || code == 0 || code > len(exileCtrKinds) {
		return 0
	}
	return exileCtrKinds[code-1]
}

func (e exileCtr) n() uint8 { return uint8(e >> 12) }

// exileCtrKinds is the code table: a record's low byte is 1 + the index here.
var exileCtrKinds = [...]kop.CounterKind{kop.CtrVoid, kop.CtrIce, kop.CtrBlood, kop.CtrHit, kop.CtrCroak}

// exileCtrCodeOf maps a counter kind to its 1-based code (0: not modelled).
func exileCtrCodeOf(kind kop.CounterKind) uint8 {
	for i, k := range exileCtrKinds {
		if k == kind {
			return uint8(i + 1)
		}
	}
	return 0
}

// useExileCtr: some card's ReplExileOnLeave leaves a counter on the exiled
// card, or some exile move carries a modelled counter rider (OpExileEx). A registered per-table flag (genneed.go), not a genPrep field.
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
	return tableExilesWithCounter(t)
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
	if k.gen == nil || !k.gen.uses.has(useExileCtr) || s.objs[o].zone != kop.ZoneExile {
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

// abilityExilesWithCounter: a's program has an OpExileEx with a modelled
// counter rider.
func abilityExilesWithCounter(a *kop.Ability) bool {
	for i := range a.Prog {
		if in := &a.Prog[i]; in.Op == kop.OpExileEx && exileCtrCodeOf(kop.CounterKind(in.B.N)) != 0 {
			return true
		}
	}
	return false
}

// tableExilesWithCounter: some card's exile move (OpExileEx) carries a counter
// rider of a kind the record models. Stash and Time keep their own stores and
// are not in the code table.
func tableExilesWithCounter(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if abilityExilesWithCounter(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

// DebugExileCounterDropped (TestExileWithCounterRecordMutation): an exile move
// keeps no counter for a kind outside Stash and Time, the old behaviour.
var DebugExileCounterDropped bool

// exileMoveCounter handles the counter rider of an OpExileEx move that just
// put o in exile (CR 122.1: the card keeps the counters it was exiled with):
// Stash marks the object, any other modelled kind is recorded. Time (suspend)
// has its own arm. A count past the record's field is a capacity stall, never
// a clamp.
func (s *State) exileMoveCounter(k *prep, in *kop.Instr, o Obj) {
	kind := kop.CounterKind(in.B.N)
	n := s.value(k, in.A)
	if n <= 0 {
		return
	}
	if kind == kop.CtrStash {
		s.stashSet(k, o)
		return
	}
	if DebugExileCounterDropped || exileCtrCodeOf(kind) == 0 {
		return
	}
	if n > maxExileCtrCount {
		s.capFull(StallBoard)
		return
	}
	s.stampExileCounterMoved(o, kind, n)
}
