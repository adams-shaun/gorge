package dirac

import "unsafe"

// The per-decision memo of manaPCGen (docs/design/dirac-static-source-cache.md).
//
// One priority walk asks manaPCGen for the same battlefield slot many times:
// the census walks (availForMode) under each availFor mode, the filter checks
// (filtOtherMana), the tap probes (canTapForMana) and the planner's reads
// (manaPCAt). Each call re-derives the slot's view from the static sources
// (granted abilities, land-type grants, reflected colours, gates), though
// nothing it reads changes inside one walk. The memo keeps the answer per
// slot for the walk running on the State that opened it.
//
// The memo lives in the ForkPool, not State (TestNoNewFixedStateFields), and
// holds no pointers: an entry names its view's place in the table's prep
// (pcTab, idx).
// An entry answers only when all of these hold:
//   - the memo is open (pcMemoOpen at stepRaw's priority arm; pcMemoClose after);
//   - the asking State is the one that opened it, and no fork of it is out
//     (pool depth 0), no filter search (inFilt) or census window is running,
//     and no in-place probe (pcMemoHold) is mutating it;
//   - the entry was written in this opening (gen) at the same object-write
//     count (State.touchN, marks.go: every object or slot write bumps it) and
//     under the same availFor mode bits (State.g.availX).
//
// Anything else recomputes, so a miss is never wrong, only slower.
// DebugPCMemoShadow recomputes on every hit and panics when the answers differ.

// pcMemoEnt is one remembered view: slot sl's manaPCGen answer at object-write
// count touch, under availFor bits avx, in opening gen. The view is kept as its
// place in the table's prep (pcTab, idx), not as a pointer: a pointer stored in
// the pool would make the prep's contents escape for escape analysis, and with
// them the locals of every caller up to buildStep (a heap move per X ask).
type pcMemoEnt struct {
	key uint64 // pcMemoKey: touch, gen and avx in one word
	idx int32
	tab pcTab // pcNone: no entry
}

// pcMemoKey packs an entry's key: the object-write count, the opening and the
// availFor bits.
func pcMemoKey(touch uint32, gen, avx uint16) uint64 {
	return uint64(touch)<<32 | uint64(gen)<<16 | uint64(avx)
}

// pcTab is the prep slice a remembered view lies in.
type pcTab uint8

const (
	pcNone   pcTab = iota
	pcNoMana       // &noManaPC
	pcCards        // k.cards
	pcVar          // k.gen.varPC
	pcGrant        // k.gen.grantPC
	pcSelf         // k.gen.selfPC
	pcBlg          // k.gen.blgPC
	pcAlt          // k.gen.altPC
	pcGate         // k.gen.gatePC
	pcSet          // k.gen.setPC
)

// pcMemo is the pool's per-slot view table. hits counts over the pool's life
// (tests read it).
type pcMemo struct {
	// owner is the address of the State that opened the memo: an identity,
	// never dereferenced. A *State here would make every State escape the
	// functions that open the memo (escape analysis), moving their locals to
	// the heap.
	owner uintptr // 0: the memo is shut
	// live is owner while no in-place probe holds the memo, else 0: the
	// one word manaPCGen compares with the asking State's address
	live uintptr
	gen  uint16
	hold uint8 // in-place probes running (pcMemoHold)
	ent  [MaxPerm]pcMemoEnt
	hits uint32
}

// DebugPCMemoOff turns the memo off: every manaPCGen call computes (the
// equivalence test and benchmarks of the uncached path).
var DebugPCMemoOff bool

// DebugPCMemoShadow makes every hit recompute and panic when the remembered
// view differs from the computed one (the shadow check).
var DebugPCMemoShadow bool

// debugPCMemoNoInval drops the memo's invalidation (pcMemoOpen's generation
// bump and the object-write count in the key): the mutation that
// TestStaticCacheShadowMutation proves the shadow check catches.
var debugPCMemoNoInval bool

// prepPCMemo reports whether the memo pays on table p: some slot's view is
// more than its card's own pcard, so a call does more than the loss check (a
// granted ability, a land-type grant or setter, a CantBeActivated lock, a
// variant view). On a table with none of them a call is cheaper than the
// memo's bookkeeping, and manaPCGen pays one flag test (58 of the 64 two-seat
// FDN bench tables).
func prepPCMemo(p *prep) bool {
	g := p.gen
	if g == nil {
		return false
	}
	if len(g.grantPC) > 0 || g.blgOn || g.landSet || g.anyCantAct {
		return true
	}
	for _, f := range g.flags {
		if f&gfManaVar != 0 {
			return true
		}
	}
	return false
}

// pcMemoOpen opens the memo for one priority walk on s. A State without a pool
// gets one here, as its first fork would (once per State).
func (s *State) pcMemoOpen(k *prep) {
	if DebugPCMemoOff || !k.pcMemo {
		return
	}
	p := s.pool
	if p == nil {
		p = new(ForkPool)
		s.pool = p
	}
	m := &p.pcm
	m.next()
	m.owner, m.live, m.hold = s.addr(), s.addr(), 0
}

// next starts a new generation: no entry written before answers after.
func (m *pcMemo) next() {
	if debugPCMemoNoInval {
		return
	}
	m.gen++
	if m.gen == 0 {
		clear(m.ent[:]) // a wrapped generation must not meet an entry 65536 generations old
		m.gen = 1
	}
}

// pcMemoDrop forgets every entry when s, the memo's owner, is overwritten
// whole (CopyFrom: a probe's restore, or another game copied in). The
// object-write count is copied with the game, so it alone cannot tell the
// games apart.
func (s *State) pcMemoDrop() {
	if p := s.pool; p != nil && p.pcm.owner == s.addr() {
		p.pcm.next()
	}
}

// pcMemoClose closes the memo opened by pcMemoOpen. The transition
// boundaries (filtMemoReset: stepRaw, Apply) and InitNStart close it too, so a
// walk a panic left open never answers past it.
func (s *State) pcMemoClose() {
	if p := s.pool; p != nil {
		p.pcm.owner, p.pcm.live = 0, 0
	}
}

// pcMemoHold marks an in-place probe on s (a write it will undo without
// marking the object, mana_filter.go filtBoundPaysWalk): no entry is read or
// written until the matching pcMemoRelease.
func (s *State) pcMemoHold() {
	if p := s.pool; p != nil {
		p.pcm.hold++
		p.pcm.live = 0
	}
	s.tyCacheHold() // static_typecache.go
}

func (s *State) pcMemoRelease() {
	if p := s.pool; p != nil && p.pcm.hold > 0 { // (a pool the probe made has no hold to release)
		if p.pcm.hold--; p.pcm.hold == 0 {
			p.pcm.live = p.pcm.owner
		}
	}
	s.tyCacheRelease()
}

// addr is s's address, the memo's owner identity.
func (s *State) addr() uintptr { return uintptr(unsafe.Pointer(s)) }

// pcMemoLive is the memo if it may answer for s now, nil otherwise.
func (s *State) pcMemoLive() *pcMemo {
	p := s.pool
	if p == nil || p.pcm.live != s.addr() || p.depth|p.inFilt != 0 || p.census {
		return nil
	}
	return &p.pcm
}

// manaPCGen is the planner's view of slot sl as a mana source on a generated
// table (manaPCGenCalc), through the per-decision memo where the table has one.
// Small enough to inline: a table without the memo pays one flag test.
func (s *State) manaPCGen(k *prep, sl Slot) *pcard {
	if k.pcMemo {
		return s.manaPCGenMemo(k, sl)
	}
	return s.manaPCGenCalc(k, sl)
}

// manaPCGenMemo is manaPCGen on a table with the memo.
func (s *State) manaPCGenMemo(k *prep, sl Slot) *pcard {
	m := s.pcMemoLive()
	if m == nil {
		return s.manaPCGenCalc(k, sl)
	}
	e := &m.ent[sl]
	touch := s.touchN
	if debugPCMemoNoInval {
		touch = 0
	}
	key := pcMemoKey(touch, m.gen, s.g.availX)
	if e.key == key && e.tab != pcNone {
		m.hits++
		pc := pcAt(k, e.tab, e.idx)
		if DebugPCMemoShadow {
			if calc := s.manaPCGenCalc(k, sl); calc != pc {
				panic("dirac: manaPCGen memo answered a stale view")
			}
		}
		return pc
	}
	pc := s.manaPCGenCalc(k, sl)
	tab, idx := pcPlace(k, pc)
	*e = pcMemoEnt{key: key, idx: idx, tab: tab}
	return pc
}

// pcAt is the view at place (tab, idx) of k's prep.
func pcAt(k *prep, tab pcTab, idx int32) *pcard {
	g := k.gen
	switch tab {
	case pcCards:
		return &k.cards[idx]
	case pcVar:
		return &g.varPC[idx]
	case pcGrant:
		return &g.grantPC[idx]
	case pcSelf:
		return &g.selfPC[idx]
	case pcBlg:
		return &g.blgPC[idx]
	case pcAlt:
		return &g.altPC[idx]
	case pcGate:
		return &g.gatePC[idx]
	case pcSet:
		return &g.setPC[idx]
	}
	return &noManaPC
}

// pcPlace finds view pc in k's prep: the slice it lies in and its index.
// A view in none of them is not remembered (pcNone): the next ask computes it
// again, so a new manaPCGen arm with a slice of its own is never answered
// wrongly, only uncached until it is listed here.
func pcPlace(k *prep, pc *pcard) (pcTab, int32) {
	if pc == &noManaPC {
		return pcNoMana, 0
	}
	g := k.gen
	if i, ok := pcIn(pc, k.cards); ok {
		return pcCards, i
	}
	if i, ok := pcIn(pc, g.varPC); ok {
		return pcVar, i
	}
	if i, ok := pcIn(pc, g.grantPC); ok {
		return pcGrant, i
	}
	if i, ok := pcIn(pc, g.selfPC[:]); ok {
		return pcSelf, i
	}
	if i, ok := pcIn(pc, g.blgPC); ok {
		return pcBlg, i
	}
	if i, ok := pcIn(pc, g.altPC); ok {
		return pcAlt, i
	}
	if i, ok := pcIn(pc, g.gatePC); ok {
		return pcGate, i
	}
	if i, ok := pcIn(pc, g.setPC); ok {
		return pcSet, i
	}
	return pcNone, 0
}

// pcIn reports pc's index in t, if t holds it. Addresses are compared as
// integers only; no integer becomes a pointer again.
func pcIn(pc *pcard, t []pcard) (int32, bool) {
	if len(t) == 0 {
		return 0, false
	}
	off := uintptr(unsafe.Pointer(pc)) - uintptr(unsafe.Pointer(&t[0]))
	if i := off / unsafe.Sizeof(pcard{}); off%unsafe.Sizeof(pcard{}) == 0 && i < uintptr(len(t)) {
		return int32(i), true
	}
	return 0, false
}
