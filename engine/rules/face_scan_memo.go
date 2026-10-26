package rules

import (
	"fmt"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// faceScan is a set of text-scan verdicts about one compiled face. Each bit
// is a pure function of the face's own content (its SVars, keywords and
// ability/trigger/replacement/static text), which is fixed once the face is
// built: the pay-time converge/cast-spend capture gates and the auto-pay
// shape gate re-ran these substring scans over every battlefield face on
// every cast and every planner query scope. A configured face's verdicts
// live in the shared compiled face table (walkFaceFacts.scan); for any other
// face faceScanHas memoises them per face pointer in faceScanMemo, shared by
// every engine. In the rules test binary faceScanVerify recomputes every hit
// and panics on a difference.
type faceScan uint16

const (
	faceScanComputed faceScan = 1 << iota
	// faceScanConvergeReader: an SVar reads TriggeredCard$Converge (any
	// case) -- triggeredConvergeReaderOut's per-face test.
	faceScanConvergeReader
	// faceScanCastSpendReader: the face mentions
	// TriggeredCard$CastTotalManaSpent (objectReadsTriggeredCastSpend).
	faceScanCastSpendReader
	// faceScanMentionsSunburst: Mentions("Sunburst") (sunburstGrantOut).
	faceScanMentionsSunburst
	// faceScanSunburstGrantPlan: faceGrantsSunburstForPlan.
	faceScanSunburstGrantPlan
	// faceScanReadsManaSpent: faceReadsManaSpent.
	faceScanReadsManaSpent
	// faceScanModalCost: some SVar names a mode printing a ModeCost$
	// (faceHasModeCost).
	faceScanModalCost
	// faceScanTargetedCastSpend: the face mentions
	// Targeted$CastTotalManaSpent, or TriggerRemembered$CastTotalManaSpent
	// (Mana Sculpt's delayed read of the spell it countered)
	// (targetedCastSpendReaderOut).
	faceScanTargetedCastSpend
	// faceScanAscend: faceMentionsGrantWord(f, "Ascend") (ascendPossible).
	faceScanAscend
	// faceScanStoried: faceMentionsGrantWord(f, "Storied")
	// (storiedPossible).
	faceScanStoried
	// faceScanGrantsRepl: faceGrantsRepl (replGrantsPossible).
	faceScanGrantsRepl
)

// faceScanVerify: see derivedMemoVerify. Set by the rules test binary.
var faceScanVerify = derivedMemoVerifyFlag != ""

func computeFaceScan(f *cards.Face) faceScan {
	s := faceScanComputed
	if faceConvergeSVarReader(f) {
		s |= faceScanConvergeReader
	}
	if f.Mentions("TriggeredCard$CastTotalManaSpent") {
		s |= faceScanCastSpendReader
	}
	if f.Mentions("Sunburst") {
		s |= faceScanMentionsSunburst
	}
	if pay.FaceGrantsSunburstForPlan(f) {
		s |= faceScanSunburstGrantPlan
	}
	if pay.FaceReadsManaSpent(f) {
		s |= faceScanReadsManaSpent
	}
	if faceHasModeCost(f) {
		s |= faceScanModalCost
	}
	if f.Mentions("Targeted$CastTotalManaSpent") || f.Mentions("TriggerRemembered$CastTotalManaSpent") {
		s |= faceScanTargetedCastSpend
	}
	if faceMentionsGrantWord(f, "Ascend") {
		s |= faceScanAscend
	}
	if faceMentionsGrantWord(f, "Storied") {
		s |= faceScanStoried
	}
	if faceGrantsRepl(f) {
		s |= faceScanGrantsRepl
	}
	return s
}

// faceMentionsGrantWord reports whether f prints word in a keyword or names
// it in any SVar, static, ability or trigger (or trigger effect) parameter --
// a grant of it. Deliberately over-inclusive: a false positive only costs
// the full scan its caller gates.
func faceMentionsGrantWord(f *cards.Face, word string) bool {
	params := func(m map[string]string) bool {
		for _, v := range m { // membership test only; order cannot matter.
			if strings.Contains(v, word) {
				return true
			}
		}
		return false
	}
	for _, k := range f.Keywords {
		if strings.Contains(k, word) {
			return true
		}
	}
	if params(f.SVars) {
		return true
	}
	for _, st := range f.Statics {
		if params(st.Params) {
			return true
		}
	}
	for _, a := range f.Abilities {
		if a != nil && params(a.Params) {
			return true
		}
	}
	for _, tr := range f.Triggers {
		if params(tr.Params) || (tr.Effect != nil && params(tr.Effect.Params)) {
			return true
		}
	}
	return false
}

// cardScanHas reports whether some face of c carries bit (faceScanHas). A
// nil card carries none.
func (e *Engine) cardScanHas(c *cards.Card, bit faceScan) bool {
	if c == nil {
		return false
	}
	for _, f := range c.Faces {
		if e.faceScanHas(f, bit) {
			return true
		}
	}
	return false
}

// faceScanHas reports whether f's scan carries bit. A nil face carries none.
func (e *Engine) faceScanHas(f *cards.Face, bit faceScan) bool {
	if f == nil {
		return false
	}
	// A configured face's verdicts were computed once with the shared
	// compiled text (walkFaceFacts.scan), so a fresh clone does not rescan
	// every face it meets.
	if ff := e.walkFaceFactsOf(f); ff != nil && ff.fullyCurrent(f) {
		if faceScanVerify {
			if fresh := computeFaceScan(f); fresh != ff.scan {
				panic(fmt.Sprintf("rules: compiled face scan for %q is stale (%b vs %b)", f.Name, ff.scan, fresh))
			}
		}
		return ff.scan&bit != 0
	}
	slot := &faceScanMemo[(uintptr(unsafe.Pointer(f))>>4)%uintptr(len(faceScanMemo))]
	if en := slot.Load(); en != nil && en.face == f {
		if faceScanVerify {
			if fresh := computeFaceScan(f); fresh != en.scan {
				panic(fmt.Sprintf("rules: face scan memo for %q is stale (%b vs %b)", f.Name, en.scan, fresh))
			}
		}
		return en.scan&bit != 0
	}
	en := &faceScanEntry{face: f, scan: computeFaceScan(f)}
	slot.Store(en)
	return en.scan&bit != 0
}

// faceScanMemo memoises computeFaceScan for the faces outside the compiled
// face table, shared by every engine: the search clones that ask (the
// ascend/storied/replacement-grant proofs restart per clone) would otherwise
// each rescan every face they meet. A direct-mapped table of immutable
// entries: an entry holds its face, so the address cannot be reused while it
// is cached, and a collision only costs a recomputation.
var faceScanMemo [4096]atomic.Pointer[faceScanEntry]

type faceScanEntry struct {
	face *cards.Face
	scan faceScan
}

// faceConvergeSVarReader is triggeredConvergeReaderOut's per-face test: some
// SVar reads TriggeredCard$Converge in any case (an any-SVar OR, so the map
// order cannot matter).
func faceConvergeSVarReader(f *cards.Face) bool {
	for _, v := range f.SVars {
		if strings.Contains(strings.ToLower(v), "triggeredcard$converge") {
			return true
		}
	}
	return false
}

// faceHasModeCost is the auto-pay shape gate's modal-cost test: some SVar of
// f resolves to a mode with a ModeCost$, priceable or not (modeCost's
// present). An any-SVar OR, so the map order cannot matter.
func faceHasModeCost(f *cards.Face) bool {
	for name := range f.SVars {
		if _, present, _ := modeCost(f, name); present || modeCostUnparseable(f, name) {
			return true
		}
	}
	return false
}
