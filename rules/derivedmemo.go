package rules

import (
	"fmt"
	"slices"

	"github.com/adams-shaun/gorge/state"
)

// Derived memo for one legal-actions walk.
//
// legalActionsPriced (the priority offer walk and its potential-action twin)
// asks Derived for the same objects over and over: every cost/restriction
// static's ValidCard$ match goes through matchesSpec, which reads the
// candidate's derived keywords, and HasKeyword, derivedKeywordParam and the
// granted-cycling/cumulative/mana-ability collectors each derive the same
// object again. Measured on botbench -pairs all -games 4 -seed 13 (27 decks),
// 39.3M of the 68.9M derivedWith calls ran inside a legal-actions walk and
// 29.7M of those (75.5%) repeated an object already derived earlier in the
// SAME walk; outside the walk the repeat rate is ~11%, so the memo is scoped
// to the walk and nowhere else.
//
// Why the walk scope and not a global per-epoch cache: Derived's inputs are
// the board (e.G, moved only by events.Apply, so len(e.L.Events) keys it),
// e.continuous (keyed by continuousVersion, exactly active()'s cache key), and
// a set of NON-event engine runtime fields that the layer walk reads
// transitively -- the rename table and its renameBuilding guard, the layer-4
// type table and typesBuilding, the effectMatchOverride observer binding,
// goadProbe, derivingColors*, activeDepth/derivedDepth, and whatever an
// EvalCount host method reads for a layer-7 amount. A global cache would have
// to prove every one of those constant across an epoch (and tests mutate
// e.G directly with no event at all). legalActionsPriced is documented and
// written as a pure read -- it emits no event and writes no state field -- so
// inside one walk every one of those inputs is fixed, and the memo only has to
// be invalidated by the walk's own boundaries:
//
//   - derivedMemoGen is bumped on every OUTERMOST scope entry, so an entry
//     built by an earlier walk is never served (whatever changed between the
//     two walks, event-backed or not, is irrelevant);
//   - each entry also records (len(e.L.Events), continuousVersion,
//     len(e.G.Objs)) and must match all three, so even an emit or a
//     continuous-registry write inside a walk (none exists today) would miss
//     rather than serve a stale result;
//   - derivedMemoUsable bypasses the memo (neither read nor written) whenever
//     one of the runtime guards above is live, i.e. for any Derived that is
//     nested inside another derivation, an active() build, a rename/type
//     table build, a goad probe or an Effect observer match. The atStack
//     zone-override path (convoke's AffectedZone$ Stack read) is never
//     memoized either: derivedWith only consults the memo for atStack == 0.
//
// Ownership: a cached result's Keywords/Types are copied out of the shared
// derivedKW/derivedTypes scratch into the entry's own backing arrays, so a
// later Derived of ANOTHER object (which rewrites the scratch) can never
// mutate a slice a caller is still ranging. An entry's arrays are rewritten
// only when the same ObjID misses again, which within one walk cannot happen
// (the key cannot move), and callers never retain a Derived result past the
// walk (Derived's documented contract). The arrays are reused across walks,
// so the steady state stays allocation-free.
//
// derivedMemoVerify (set by the rules test binary, or at link time through
// derivedMemoVerifyFlag for a botbench run) recomputes every hit and panics
// on any difference -- the empirical half of the argument above.

// derivedMemoVerifyFlag turns verify mode on in a non-test binary:
// go build -ldflags "-X github.com/adams-shaun/gorge/rules.derivedMemoVerifyFlag=1".
var derivedMemoVerifyFlag string

var derivedMemoVerify = derivedMemoVerifyFlag != ""

type derivedMemoEntry struct {
	gen  uint64
	ep   int
	ver  int
	objs int
	d    Derived
	kw   []string
	ty   []string
}

// beginDerivedMemo opens a memo scope; endDerivedMemo closes it. Only the
// outermost entry starts a new generation, so a nested walk shares the outer
// walk's entries.
func (e *Engine) beginDerivedMemo() {
	if e.derivedMemoDepth == 0 {
		e.derivedMemoGen++
	}
	e.derivedMemoDepth++
}

func (e *Engine) endDerivedMemo() { e.derivedMemoDepth-- }

// derivedMemoUsable reports whether this Derived call is a top-level read
// whose every non-event input is the walk's fixed state (see above).
func (e *Engine) derivedMemoUsable() bool {
	return e.derivedDepth == 0 && e.activeDepth == 0 && !e.renameBuilding && !e.typesBuilding &&
		!e.effectMatchOverride && e.goadProbe == 0 && !e.derivingColorsSet
}

func (e *Engine) derivedMemoized(id state.ObjID) Derived {
	if id == 0 || int(id) > len(e.G.Objs) {
		return e.derivedCompute(id, 0)
	}
	if n := len(e.G.Objs) + 1; len(e.derivedMemo) < n {
		e.derivedMemo = append(e.derivedMemo, make([]derivedMemoEntry, n-len(e.derivedMemo))...)
	}
	m := &e.derivedMemo[id]
	ep, ver, objs := len(e.L.Events), e.continuousVersion, len(e.G.Objs)
	if m.gen == e.derivedMemoGen && m.ep == ep && m.ver == ver && m.objs == objs {
		if derivedMemoVerify {
			e.verifyDerivedMemo(id, m.d)
		}
		return m.d
	}
	if m.gen == e.derivedMemoGen {
		// Same walk, moved key (unreachable today: the walk emits nothing).
		// A caller of this walk may still be ranging the old arrays, so
		// the rebuild gets fresh ones instead of rewriting them in place.
		m.kw, m.ty = nil, nil
	}
	d := e.derivedCompute(id, 0)
	m.kw = append(m.kw[:0], d.Keywords...)
	m.ty = append(m.ty[:0], d.Types...)
	d.Keywords, d.Types = m.kw, m.ty
	m.d = d
	m.gen, m.ep, m.ver, m.objs = e.derivedMemoGen, ep, ver, objs
	return d
}

func (e *Engine) verifyDerivedMemo(id state.ObjID, got Derived) {
	want := e.derivedCompute(id, 0)
	if got.Power != want.Power || got.Toughness != want.Toughness || got.Name != want.Name ||
		got.Colors != want.Colors || !slices.Equal(got.Keywords, want.Keywords) || !slices.Equal(got.Types, want.Types) {
		panic(fmt.Sprintf("rules: derived memo stale for obj %d: cached %+v, fresh %+v", id, got, want))
	}
}
