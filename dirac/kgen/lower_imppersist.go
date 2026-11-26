package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// lower_imppersist.go: an Imprinted list kept on the source (CardDef.ImpPersist)
// rather than in the resolution's set. gorge writes Object.Imprinted on the
// source and reads it from a later ability, a trigger or a static; the kernel
// keeps the list in the per-source memory (zone_srcmem.go), seeded at the
// start of the source's next resolution (zone_imprint.go). The shape is the
// reader/writer split: a writer on an activated ability or a battlefield
// trigger, a reader (IsImprinted, Defined$ Imprinted, Imprinted$) or a
// ClearImprinted$ elsewhere, and none of the riders the store cannot carry
// (ForgetImprinted, Unimprint, ImprintLast, ImprintFound and the rest).

// impWriterSA reports whether sa adds to the source's imprinted list.
func impWriterSA(sa *cards.SA) bool {
	if sa == nil {
		return false
	}
	if impExileWriterSA(sa) {
		return true
	}
	p := sa.Params
	if v, ok := p["ImprintCards"]; ok {
		// Pump ImprintCards$ TriggeredCard: the trigger's object joins the
		// source's Imprinted list (gorge imprint on the trigger body,
		// effects/context.go; CounterAddedAll's sibling shapes do not use it).
		return sa.API == "Pump" && strings.TrimSpace(v) == "TriggeredCard"
	}
	switch sa.API {
	case "Token":
		return strings.TrimSpace(p["ImprintTokens"]) == "True"
	case "Mill":
		return strings.TrimSpace(p["Imprint"]) == "True"
	}
	return false
}

// impExileWriterSA is the ChangeZone exile arm of impWriterSA. Its exiled
// card is also in Object.ExiledWith, which the token and mill writers' objects
// are not (the fold below relies on the two lists coinciding).
func impExileWriterSA(sa *cards.SA) bool {
	if sa == nil || sa.API != "ChangeZone" {
		return false
	}
	p := sa.Params
	if strings.TrimSpace(p["Imprint"]) != "True" || strings.TrimSpace(p["Destination"]) != "Exile" {
		return false
	}
	if strings.TrimSpace(p["Origin"]) == "Hand" && strings.TrimSpace(p["Hidden"]) == "True" && strings.TrimSpace(p["Defined"]) == "" {
		return true // a hand pick into exile (Parallax Nexus; lower_imprintzone.go handMoveImprint)
	}
	return (strings.TrimSpace(p["ValidTgts"]) != "" || strings.TrimSpace(p["Defined"]) != "") &&
		strings.TrimSpace(p["Hidden"]) == ""
}

// impWalk is one fail-closed walk over a face's script: its Sub chains and
// the SVars its parameters name. A fresh walk per root keeps a writer reached
// from two roots visible to both, so a writer in an SP$ cannot hide behind a
// trigger that also names its SVar.
type impWalk struct {
	f             *cards.Face
	seenSA        map[*cards.SA]bool
	seenSVar      map[string]bool
	writer        bool
	nonExileWrite bool // a writer that is not a ChangeZone exile (token or mill)
	reader        bool
	digImprint    bool
	definedImp    bool
	imprinted     bool
	clear         bool
	branch        bool // the chain repeats or branches (RepeatEach, a RollDice result list)
	sawImprint    bool // the walk saw an ImprintCards$ param
	badImprint    bool // ... and it is not the Pump TriggeredCard shape kgen lowers
}

func newImpWalk(f *cards.Face) *impWalk {
	return &impWalk{f: f, seenSA: map[*cards.SA]bool{}, seenSVar: map[string]bool{}}
}

// foldImprintExiledWith reads a paired ExiledWithSource on an ImpPersist
// face whose writers are all ChangeZone exiles: the writer's exiled card is
// exactly an IsImprinted candidate, so the kernel reads the one persistent
// list (lower_imppersist.go) instead of a second link. A face with a token or
// mill writer keeps the conjunct: its objects are in the imprint association
// but not in Object.ExiledWith, and gorge requires both words. Only the
// clause that itself carries the IsImprinted word is folded; another clause's
// ExiledWithSource keeps its link.
func (lc *lowerCtx) foldImprintExiledWith(c *lClause) {
	if !lc.impFoldExiled || c.Flags&fExiledWith == 0 || c.CmpStat != cmpWord7 || c.CmpN != int(kop.W7Imprinted) {
		return
	}
	c.Flags &^= fExiledWith
}

// svar resolves an SVar name once per walk.
func (w *impWalk) svar(name string) *cards.SA {
	name = strings.TrimSpace(name)
	if name == "" || w.seenSVar[name] {
		return nil
	}
	sub := cards.ResolveSVar(w.f.SVars, name)
	if sub != nil {
		w.seenSVar[name] = true
	}
	return sub
}

// svarNames returns the SVar names a parameter value may name: its
// comma-separated entries, a ResultSubAbilities$ entry's `lo-hi:NAME` reduced
// to NAME.
func svarNames(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if i := strings.LastIndexByte(part, ':'); i >= 0 {
			part = part[i+1:]
		}
		if n := strings.TrimSpace(part); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// chain walks sa and every ability its parameters name, recording the shape
// facts a writer or reader needs.
func (w *impWalk) chain(sa *cards.SA) {
	for ; sa != nil; sa = sa.Sub {
		if w.seenSA[sa] {
			return
		}
		w.seenSA[sa] = true
		if impWriterSA(sa) {
			w.writer = true
			w.nonExileWrite = w.nonExileWrite || !impExileWriterSA(sa)
		}
		if sa.API == "Dig" && strings.TrimSpace(sa.Params["Imprint"]) == "True" {
			w.digImprint = true
		}
		for k, v := range sa.Params {
			if k == "RepeatSubAbility" || k == "ResultSubAbilities" {
				w.branch = true
			}
			if strings.Contains(v, "IsImprinted") || k == "Imprinted" || strings.Contains(v, "ImprintedSize") {
				w.imprinted = true
			}
			if k == "ImprintCards" {
				w.sawImprint = true
				if sa.API != "Pump" || strings.TrimSpace(v) != "TriggeredCard" {
					w.badImprint = true
				}
			}
			if k == "Defined" && strings.Contains(v, "Imprinted") {
				w.definedImp = true
			}
			if k == "ClearImprinted" || strings.Contains(v, "ClearImprinted") {
				w.clear = true
			}
			for _, name := range svarNames(v) {
				if sub := w.svar(name); sub != nil {
					w.chain(sub)
				}
			}
		}
	}
	w.reader = w.imprinted || w.definedImp
}

// impFacts is one whole-face walk for the reader, clear and Dig facts.
func (lc *lowerCtx) impFacts() (reader, clear, digImprint bool) {
	w := newImpWalk(lc.face)
	for _, a := range lc.face.Abilities {
		w.chain(a)
	}
	for _, t := range lc.face.Triggers {
		w.chain(t.Effect)
	}
	for i := range lc.face.Statics {
		// a static has no body; its parameters may name an SVar ability
		w.chain(w.svar(lc.face.Statics[i].Params["AddAbility"]))
	}
	for i := range lc.face.Repls {
		w.chain(lc.face.Repls[i].With)
	}
	return w.imprinted || w.definedImp, w.clear, w.digImprint
}

// paramsMention reports whether any value of a parameter map contains needle.
// Map order does not reach the answer, only the boolean.
func paramsMention(p map[string]string, needle string) bool {
	for _, v := range p {
		if strings.Contains(v, needle) {
			return true
		}
	}
	return false
}

// leaveExileTrigger reports whether a ChangesZone trigger's zone pair is the
// leave-exile shape the kernel's TrigLeaveExile fires (Origin$ Exile, no
// Destination$ or Any, the battlefield trigger zone fireLeaveExile scans).
func leaveExileTrigger(p map[string]string) bool {
	if o := strings.TrimSpace(p["Origin"]); o != "Exile" || strings.Contains(o, ",") {
		return false
	}
	d := strings.TrimSpace(p["Destination"])
	if d != "" && d != "Any" {
		return false
	}
	z := strings.TrimSpace(p["TriggerZones"])
	return z == "" || z == "Battlefield"
}

// impPersistShape reports whether the face keeps its Imprinted list on the
// source for a later ability to read. The second result says whether an
// IsImprinted+ExiledWithSource clause may fold: every writer is a ChangeZone
// exile, so its exiled cards are exactly its imprinted ones.
func (lc *lowerCtx) impPersistShape() (persist, foldExiled bool) {
	f := lc.face
	if f == nil {
		return false, false
	}
	for _, bad := range []string{
		"ImprintFound", "ImprintRevealed", "ImprintOnHost",
		"ImprintPlayed", "sharesNameWith Imprinted", "ImprintedLKI",
	} {
		if f.Mentions(bad) || f.MentionsParam(bad) {
			return false, false
		}
	}
	// A reader in a static, a trigger condition or a replacement condition is
	// evaluated by the battlefield walk (predPermCold -> wordPerm(NoSlot)),
	// which has no filter source, and a trigger candidate's zone is judged
	// live, not as the LKI snapshot gorge hands the predicate. Refuse the
	// shape rather than lower a reader whose source or candidate zone is not
	// threaded.
	for i := range f.Statics {
		if paramsMention(f.Statics[i].Params, "IsImprinted") {
			return false, false
		}
		w := newImpWalk(f)
		w.chain(w.svar(f.Statics[i].Params["AddAbility"]))
		if w.reader {
			return false, false
		}
	}
	for i := range f.Triggers {
		if !paramsMention(f.Triggers[i].Params, "IsImprinted") {
			continue
		}
		// The one candidate shape the store's read carries: the leave-exile
		// line (Origin$ Exile, no Destination$) whose ValidCard$ gorge
		// judges on the event's LKI snapshot, still in exile — the kernel's
		// TrigLeaveExile fires the same way, the card still in exile
		// (placeFromK's hook; lower_changezonepairs.go). Any other trigger's
		// candidate is judged live and stays refused.
		if !leaveExileTrigger(f.Triggers[i].Params) {
			return false, false
		}
	}
	for i := range f.Repls {
		if paramsMention(f.Repls[i].Params, "IsImprinted") {
			return false, false
		}
	}

	badImprint := false // an ImprintCards$ spelling this change does not lower
	sawImprint := false // any ImprintCards$ at all (so the face-level guard below can tell)
	writers := 0
	outside := false      // a reader or a ClearImprinted$ outside every writer's chain
	writerBranch := false // a writer chain that repeats or branches
	nonExileWrite := false
	for _, a := range f.Abilities {
		w := newImpWalk(f)
		w.chain(a)
		rememberImpWalk(w, &badImprint, &sawImprint)
		if !w.writer {
			outside = outside || w.reader || w.clear
			continue
		}
		writers++
		writerBranch = writerBranch || w.branch
		nonExileWrite = nonExileWrite || w.nonExileWrite
		// No writer is in an SP$, and an activated writer must be usable
		// only from the battlefield.
		if a.Kind != "AB" {
			return false, false
		}
		if z := strings.TrimSpace(a.Params["ActivationZone"]); z != "" && z != "Battlefield" {
			return false, false
		}
	}
	for i := range f.Triggers {
		w := newImpWalk(f)
		w.chain(f.Triggers[i].Effect)
		rememberImpWalk(w, &badImprint, &sawImprint)
		if !w.writer {
			outside = outside || w.reader || w.clear
			continue
		}
		writers++
		writerBranch = writerBranch || w.branch
		nonExileWrite = nonExileWrite || w.nonExileWrite
		if z := strings.TrimSpace(f.Triggers[i].Params["TriggerZones"]); z != "" && z != "Battlefield" {
			return false, false
		}
	}
	if writers == 0 {
		return false, false
	}
	// A writer in a static or a replacement body is outside the shape.
	for i := range f.Statics {
		w := newImpWalk(f)
		w.chain(w.svar(f.Statics[i].Params["AddAbility"]))
		rememberImpWalk(w, &badImprint, &sawImprint)
		if w.writer {
			return false, false
		}
		outside = outside || w.reader || w.clear
	}
	for i := range f.Repls {
		w := newImpWalk(f)
		w.chain(f.Repls[i].With)
		rememberImpWalk(w, &badImprint, &sawImprint)
		if w.writer {
			return false, false
		}
		outside = outside || w.reader || w.clear
	}

	if !imprintCardsSupported(f, badImprint, sawImprint) {
		return false, false
	}
	_, _, digImprint := lc.impFacts()
	if digImprint {
		return false, false
	}
	// The persistent list matters only when a reader or a clear sits outside
	// the writer's chain, or the writer's chain repeats or branches (a
	// RepeatEach body, a RollDice result: the kernel's one-chain gates refuse
	// the clear there). A writer whose read and clear stay in its own straight
	// chain is the one-resolution shape the kernel already held, and an
	// uncleared writer with no reader outside its chain still refuses.
	persist = outside || writerBranch
	return persist, persist && !nonExileWrite
}

// rememberImpWalk folds a walk's ImprintCards facts into the face-level
// counters impPersistShape checks once every chain has been walked.
func rememberImpWalk(w *impWalk, bad, saw *bool) {
	*bad = *bad || w.badImprint
	*saw = *saw || w.sawImprint
}

// imprintCardsSupported reports whether every ImprintCards$ the face names is
// the Pump TriggeredCard writer this change lowers (saw). A face-level mention
// the walks did not reach stays unsupported.
func imprintCardsSupported(f *cards.Face, bad, saw bool) bool {
	return !bad && (!f.Mentions("ImprintCards") || saw)
}
