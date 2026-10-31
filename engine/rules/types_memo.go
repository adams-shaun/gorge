package rules

import (
	"slices"
	"strconv"

	"github.com/adams-shaun/gorge/engine/rules/chars"
	"github.com/adams-shaun/gorge/engine/state"
)

// Derived-types memo: the layer-4 walk's answer per object, across reads.
//
// The layer-4 walk (chars.TypesAndLandSubtypeSet) runs every live type
// effect's Affected$ match against the object, and it is reached from two
// top-level reads that both run it from scratch on every call: derivedTypesOf
// (IsCreature/IsLand/IsPlaneswalker, the station and mana-ability
// candidate scans, chars.Reader.DerivedTypes) and every Derived build that
// misses the walk-scoped Derived memo (derivedmemo.go) -- which outside a
// legal-actions walk or a BeginDerivedReads scope is every one of them: a
// resolution's IsCreature census, the resolution kernel's commit, a host projection.
// Measured on dirac's station lockstep test (the station spacecraft's
// `Card.Self+counters_GE<N>_CHARGE` AddType$ Creature statics are live
// layer-4 effects matched against every object), the walk was 57% of the
// test's CPU, almost all of it re-deriving an unchanged object at an
// unchanged board.
//
// The key is the Derived memo's own cross-walk key, so this memo is exactly
// as sound as that one (see derivedMemoServes and derived_transparent.go):
//
//   - derivedSeq, read after bringing active() up to date: it moves with
//     every active() rebuild (any non-layer-inert event, a continuous
//     registry write, an object-count move, a static memo content change,
//     the explicit retireCrossWalkMemo invalidations around faceprobe.go's
//     and castprobe.go's eventless face flips and statics_costmods.go's
//     exclusion) EXCEPT one derived_transparent.go proves leaves every
//     derivation unchanged -- and that proof retires the entries of each
//     object an event of its run named (derivedRebuildTransparent), which
//     retires this memo's entries too;
//   - continuousVersion and the arena length, with the cross-walk memo's
//     transparent-growth allowance (derivedPrevObjs);
//   - derivedMemoUsable at both the store and the serve: no derivation,
//     active() build, rename/type table build, goad probe, Effect observer
//     match or layer-5 colour stash is in flight, so every non-event runtime
//     input of the walk is at rest.
//
// Two further conditions narrow what is ever stored, beyond the Derived
// memo's own:
//
//   - every live layer-4 effect is local (derivedEffectLocal: its Affected$
//     reads only the candidate's own characteristics, counters, controller
//     and the effect's source and remembered set -- all event-backed), so no
//     non-event input can reach a stored answer through a filter;
//   - the object's face carries no characteristic-defining static
//     (faceHasCDAStatic), and active() did not rebuild during the walk.
//
// A Clone, a kernel restore (which adopts a clone), a restart and an entry
// preview all start with an empty table (clone tag reset; isolatePreviewMemos).
// The memo is derived data, never game state: nothing here is hashed, logged
// or emitted. In the rules test binary derivedMemoVerify recomputes every
// hit and panics on any difference.

// typesMemoEntry is one object's memoized layer-4 answer. seq 0 is "no
// answer" (derivedSeq is never 0 once a build has stamped it).
type typesMemoEntry struct {
	seq     uint64
	ver     int
	objs    int
	ty      []string
	all     bool
	landSet bool
}

// typesMemoTable is the per-ObjID table: a dense index into the entries of
// objects actually derived, the shape derivedMemoTable uses (a zero index is
// "no slot").
type typesMemoTable struct {
	idx     []int32
	entries []typesMemoEntry
}

func (t *typesMemoTable) slot(id state.ObjID, n, hint int) *typesMemoEntry {
	if len(t.idx) < n {
		if cap(t.idx) < n {
			grown := make([]int32, len(t.idx), max(n, hint, 2*cap(t.idx)))
			copy(grown, t.idx)
			t.idx = grown
		}
		t.idx = t.idx[:n]
	}
	k := t.idx[id]
	if k == 0 {
		t.entries = append(t.entries, typesMemoEntry{})
		k = int32(len(t.entries))
		t.idx[id] = k
	}
	return &t.entries[k-1]
}

func (t *typesMemoTable) at(id state.ObjID) *typesMemoEntry {
	if int(id) >= len(t.idx) || t.idx[id] == 0 {
		return nil
	}
	return &t.entries[t.idx[id]-1]
}

// release clears the table's arrays for reuse by another engine (Spare).
func (t *typesMemoTable) release() typesMemoTable {
	idx, entries := t.idx[:cap(t.idx)], t.entries[:cap(t.entries)]
	clear(idx)
	for i := range entries {
		entries[i] = typesMemoEntry{ty: entries[i].ty[:0]}
	}
	return typesMemoTable{idx: idx[:0], entries: entries[:0]}
}

// retire drops id's entry from reuse (derivedRebuildTransparent's touched
// objects).
func (t *typesMemoTable) retire(id state.ObjID) {
	if m := t.at(id); m != nil {
		m.seq = 0
	}
}

// layer4Top is the layer-4 answer for id in its live zone, in the framing
// derivedCompute's walk runs it (charsWalk.Depth raised, the engine's
// active() list), served from the memo when the memo holds it. ok is false
// when the memo does not apply -- an id outside the arena, a faceless
// object, a read nested in a derivation or another guarded build
// (derivedMemoUsable), an engine with no log -- and the caller then runs its
// own uncached walk exactly as before. The returned Types is owned by the
// memo entry or the walk: read-only, not retained past the caller's own
// read.
func (e *Engine) layer4Top(id state.ObjID) (chars.Layer4, bool) {
	if !e.derivedMemoUsable() {
		return chars.Layer4{}, false
	}
	return e.layer4Memo(id)
}

// layer4ForTable is buildDerivedTypesWalk's per-object layer-4 read (act is
// active()'s current list). The table build runs with typesBuilding set,
// which derivedMemoUsable refuses because a general derivation's ordinary
// filters bind the half-built table (ctx_new.go's DerivedTypes). The layer-4
// walk itself never reads that table -- every Affected$ match of a layer walk
// clears it (matchesWithCharsPTSlow) -- and with every live layer-4 effect
// local (layer4Local) no match reaches another object's ordinary filter, so
// the build's answer for an object is the answer any later top-level read
// gets, and the memo serves and stores it here under every other guard.
// Otherwise the walk runs uncached, exactly as before. The full-walk
// reference (buildDerivedTypesFull) never comes here, so layer4PrecheckVerify
// still holds each memo-served table to an uncached walk.
func (e *Engine) layer4ForTable(act []ContinuousEffect, id state.ObjID) ([]string, bool) {
	if e.typesBuilding && e.charsWalk.Depth == 0 && e.activeDepth == 0 && !e.renameBuilding &&
		!e.effectMatchOverride && e.goadProbe == 0 && !e.charsWalk.ColorsSet {
		if local, _ := e.layer4Local(act); !local {
			return chars.TypesAndAllCreatureTypes(asChars(e), act, id, 0)
		}
		if l4, ok := e.layer4Memo(id); ok {
			return l4.Types, l4.AllCreatureTypes
		}
	}
	return chars.TypesAndAllCreatureTypes(asChars(e), act, id, 0)
}

// layer4Memo is layer4Top past its derivedMemoUsable guard (the caller has
// checked its own).
func (e *Engine) layer4Memo(id state.ObjID) (chars.Layer4, bool) {
	if id == 0 || int(id) > len(e.G.Objs) {
		return chars.Layer4{}, false
	}
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return chars.Layer4{}, false
	}
	if e.L == nil {
		return chars.Layer4{}, false
	}
	act := e.active()
	local, live := e.layer4Local(act)
	if !live {
		// No layer-4 effect is live: the walk is chars' printed-list fast
		// path, cheaper than a memo read.
		return chars.Layer4{}, false
	}
	seq, ver, objs := e.derivedSeq, e.continuousVersion, len(e.G.Objs)
	if seq != 0 {
		if m := e.typesMemo.at(id); m != nil && m.seq == seq && m.ver == ver &&
			(m.objs == objs || (m.objs < objs && objs == e.derivedPrevObjs)) {
			got := chars.Layer4{Types: m.ty, AllCreatureTypes: m.all, LandSet: m.landSet}
			if derivedMemoVerify {
				e.verifyTypesMemo(id, got)
			}
			return got, true
		}
	}
	builds := e.activeBuildSeq
	l4 := e.layer4Fresh(id)
	if seq == 0 || e.activeBuildSeq != builds || e.derivedSeq != seq || faceHasCDAStatic(o) || !local {
		return l4, true
	}
	m := e.typesMemo.slot(id, len(e.G.Objs)+1, cap(e.G.Objs)+1)
	m.ty = memoOwned(&m.ty, l4.Types)
	m.all, m.landSet = l4.AllCreatureTypes, l4.LandSet
	m.seq, m.ver, m.objs = seq, ver, objs
	l4.Types = m.ty
	return l4, true
}

// layer4Fresh is the uncached layer-4 walk in derivedCompute's framing.
func (e *Engine) layer4Fresh(id state.ObjID) chars.Layer4 {
	e.charsWalk.Depth++
	ty, all, landSet := chars.TypesAndLandSubtypeSet(asChars(e), e.active(), id, 0)
	e.charsWalk.Depth--
	return chars.Layer4{Types: ty, AllCreatureTypes: all, LandSet: landSet}
}

// layer4Local reports whether every layer-4 effect of act (active()'s
// current list) is local (derivedEffectLocal) and whether any is live,
// memoized per active() build.
func (e *Engine) layer4Local(act []ContinuousEffect) (local, live bool) {
	if e.typesMemoLocalSeq == e.activeBuildSeq && e.typesMemoLocalOK {
		return e.typesMemoLocal, e.typesMemoAny
	}
	local = true
	for i := range act {
		if act[i].Layer != LType {
			continue
		}
		live = true
		if !derivedEffectLocal(&act[i]) {
			local = false
			break
		}
	}
	e.typesMemoLocal, e.typesMemoAny = local, live
	e.typesMemoLocalSeq, e.typesMemoLocalOK = e.activeBuildSeq, true
	return local, live
}

// verifyTypesMemo recomputes a served answer and panics on any difference
// (nil-ness included: a nil list and an empty one are different answers to
// Compute's binding).
func (e *Engine) verifyTypesMemo(id state.ObjID, got chars.Layer4) {
	want := e.layer4Fresh(id)
	if (got.Types == nil) != (want.Types == nil) || !slices.Equal(got.Types, want.Types) ||
		got.AllCreatureTypes != want.AllCreatureTypes || got.LandSet != want.LandSet {
		panic("rules: derived-types memo stale for obj " + strconv.FormatUint(uint64(id), 10))
	}
}
