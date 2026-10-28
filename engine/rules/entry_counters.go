// entry_counters.go places the entry-characteristic counters a permanent
// enters the battlefield with (starting loyalty, Riot/Unleash's election, a
// Saga's lore counter, a Battle's defense counters) through REAL
// CounterChange events, so the CR 614 AddCounter replacement class and the
// CantPutCounter prohibition see them exactly like any other placement. The
// arithmetic lives in events.EntryCounterGrants. The final placements are
// prepared against the would-enter board, then folded with the move itself.
//
// Task agent-20260923T084704Z-b2386c25 adds the last leg: when a grant's
// AddCounter competition does not commute and the affected player must make
// CR 616.1's order choice, the ENTRY itself is staged behind the answer
// (entryCounterStage) instead of folding first and adjusting after. The
// staging is detected in Engine.emit's pre-pass, before any of the fold's
// observers run, so no observer -- an ETB trigger, a state-based action, a
// chapter queue -- ever sees the un-replaced entry. The completed stage is
// consumed by foldEntryMove, which folds the move with the finalized amounts
// in its Pairs payload exactly as an uncontested entry does.

package rules

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/resolve"
	"github.com/adams-shaun/gorge/engine/state"
)

// entryCounterStage parks a battlefield entry whose entry-characteristic
// counter grant is awaiting a CR 616.1 replacement-order answer. The move
// has NOT folded: no observer can see the un-replaced entry while the
// competition is outstanding. Resuming applies the answered bodies first (on
// an isolated preview of the post-entry board, the only state the zone
// filters match on), settles the remaining grants, then re-emits the move
// through the ordinary emit -- whose fold consumes the completed stage via
// entryStageDone.
type entryCounterStage struct {
	move      events.Event               // the staged entry as the pose received it
	grants    []entryGrant               // the origin-zone snapshot plus body-defined grants
	placed    []events.EntryCounterGrant // grants finalized so far, in grant order
	counter   events.Event               // the parked grant's counter event
	cands     []replMatch                // its competition
	applied   []replMatch                // bodies already applied, in answer order
	player    state.PlayerID             // the asked player
	inRes     bool                       // the pose's in-resolution provenance
	inBody    bool                       // the entry was emitted by a replacement body; do not rematch it on re-drive
	idx       int                        // index into grants of the parked grant
	complete  bool                       // every grant finalized; the fold may consume
	redriving bool                       // the completed stage's own re-emit is in flight (mint identity)
	// bodyIDs names every Updated PutCounter|ETB$ True replacement body whose
	// placement this stage's grant set folded (replIdentity). The completed
	// fold returns them so the Updated dispatch skips running those bodies;
	// their counters are already in the move's Pairs payload.
	bodyIDs []string
	// tokenPlan carries the mints a finalized token plan still owes after the
	// mint currently staged behind this entry's order ask. The stage's
	// completed re-drive continues the plan, so a plan whose FIRST mint parks
	// does not lose the mints after it (a second same-script mint would
	// otherwise be swallowed by the outstanding-stage re-drive guard, since
	// TokenCreate events carry no identity of their own). nil for a non-plan
	// entry. Clone-copied with the rest of the stage (engine/rules/clone.go).
	tokenPlan *tokenPlanResume
}

// tokenPlanResume is the tail of a finalized token plan that must wait for
// the staged mint's entry-counter order answer: the plan's original event
// (the mint context) and the mints still owed. It lives on the parked stage
// so the completion re-drive owns the continuation.
type tokenPlanResume struct {
	ev   events.Event
	plan []tokenPlanMint
}

// outstandingEntryStage returns the first entry-counter stage still awaiting
// its order answer, or nil. A parked mint leaves its stage on the
// replacement-choice queue until the answer lands; emitTokenPlanMints uses
// this to suspend the rest of a multi-mint plan instead of emitting into an
// outstanding stage (which sameEntryMove would treat as a re-drive).
func (e *Engine) outstandingEntryStage() *entryCounterStage {
	for i := range e.replChoices {
		rc := &e.replChoices[i]
		if rc.kind == replChoiceEntryOrder && rc.stage != nil && !rc.stage.complete {
			return rc.stage
		}
	}
	return nil
}

// entryGrant is one planned entry counter: the kind and amount a grant will
// place, plus the source id it came from when it is a BODY-defined grant
// (0 for an intrinsic grant). The body id is what the settlement uses to
// hand the AddCounter matcher the same replacement-body provenance the live
// body path carries (engine/rules/engine.go's replacementBodyCounterAdder), so
// EffectOnly$/ValidSource$ read it identically.
type entryGrant struct {
	kind   string
	amount int32
	body   state.ObjID
}

// restage is the continuation of st behind the next grant's newly posed
// competition (posed, the stage stageEntryCounterOrder just built): every
// field carries over from st except the pose's own competition -- the parked
// counter event, its candidates, the asked player, the grant index -- and the
// per-competition answer state (applied, complete), which restart.
func (st *entryCounterStage) restage(posed *entryCounterStage) entryCounterStage {
	next := *st
	next.counter, next.cands, next.player, next.idx = posed.counter, posed.cands, posed.player, posed.idx
	next.applied, next.complete = nil, false
	return next
}

// sameEntryMove reports whether ev is (a re-drive of) the staged move. For a
// MoveZone the identifying fields are the ones a re-driven entry keeps: a
// re-emit may recompute markers, never origin, destination or object.
//
// A TokenCreate/CardToken mint has no such identity -- two mints of one
// script, or two copies of one source (Encore's copy per opponent), are
// field-for-field equal -- so a mint is its stage's move only while that
// stage's own completed re-drive is being emitted (redriving, set by
// resumeEntryCounterOrder around exactly that emit). Any other mint, however
// equal, is a NEW mint and stages (or folds) on its own.
func (st *entryCounterStage) sameEntryMove(ev *events.Event) bool {
	if st.move.Kind != ev.Kind {
		return false
	}
	if ev.Kind == events.TokenCreate || ev.Kind == events.CardToken {
		return st.redriving
	}
	return st.move.Obj == ev.Obj && st.move.From == ev.From && st.move.To == ev.To
}

// entryCounterGrants snapshots the intrinsic counters of a battlefield
// entry, including tokens minted directly onto the battlefield. A MoveZone
// reads its object in the origin zone (elections and compleated payment);
// token mints read the card/face their Apply case will instantiate. A
// battlefield->battlefield stay is not a new object and grants nothing.
func (e *Engine) entryCounterGrants(ev *events.Event) []events.EntryCounterGrant {
	switch ev.Kind {
	case events.MoveZone:
		if ev.To != state.ZBattlefield {
			return nil
		}
		o := e.G.Obj(ev.Obj)
		if o == nil || o.Zone == state.ZBattlefield {
			return nil
		}
		grants := events.EntryCounterGrantsForFace(o, e.entryCopyBasisFace(o), events.IsFaceDownEntry(ev.Counter))
		// The counters the moving effect says the permanent enters WITH
		// (ChangeZone's WithCountersType$, effects.entryWithCounters): CR
		// 122.6 makes them part of the entry, so they settle and fold here
		// with the intrinsic grants, after them.
		_, requested := events.SplitEntryCounterPairs(ev.Pairs)
		// CR 702.176a: paid-Impending provenance is still on the stack object
		// immediately before the move folds. Include its time counters in the
		// common entry plan so AddCounter replacements are settled before entry.
		if !events.IsFaceDownEntry(ev.Counter) && o.CastFlags&state.FlagImpending != 0 {
			if n := cachedImpendingCount(o.Face(), e.walkFaceFactsOf(o.Face())); n > 0 {
				grants = append(grants, events.EntryCounterGrant{Kind: "TIME", Amount: n})
			}
		}
		return append(grants, requested...)
	case events.TokenCreate:
		// A token mint can carry the same requested entry counters a
		// ChangeZone does (effects.entryWithCounters on the TokenCreate):
		// WithCountersType$/WithCountersAmount$ are part of the mint, so they
		// settle with the token's intrinsic grants before it enters.
		_, requested := events.SplitEntryCounterPairs(ev.Pairs)
		return append(events.EntryCounterGrants(e.tokenSnapshot(*ev), false), requested...)
	case events.CardToken:
		_, requested := events.SplitEntryCounterPairs(ev.Pairs)
		src := e.G.Obj(ev.Obj)
		if src == nil {
			return nil
		}
		// CardToken copies the card and face, not the source object's
		// counters, elections or compleated payment.
		return append(events.EntryCounterGrants(&state.Object{Card: src.Card, FaceIdx: src.FaceIdx}, false), requested...)
	}
	return nil
}

// entryBodyCandidates reports whether an entry might carry a body-defined
// counter grant -- a PutCounter|ETB$ True Updated replacement body on the
// entering object, or a bloodthirst/sunburst grant. It is the cheap gate the
// emit pre-pass and foldEntryMove take BEFORE building the (costly) isolated
// preview, so an ordinary entry with no such body pays nothing. It reads only
// state: the entering face's own Replacement lines and the derived keyword
// list. A battlefield->battlefield stay grants nothing (the same guard
// entryCounterGrants keeps).
func (e *Engine) entryBodyCandidates(ev *events.Event) bool {
	var entrant *state.Object
	switch ev.Kind {
	case events.MoveZone:
		if ev.To != state.ZBattlefield || events.IsFaceDownEntry(ev.Counter) {
			return false
		}
		entrant = e.G.Obj(ev.Obj)
		if entrant == nil || entrant.Zone == state.ZBattlefield {
			return false
		}
	case events.TokenCreate:
		entrant = e.tokenSnapshot(*ev)
	case events.CardToken:
		if src := e.G.Obj(ev.Obj); src != nil {
			entrant = &state.Object{Card: src.Card, FaceIdx: src.FaceIdx}
		}
	default:
		return false
	}
	if entrant == nil || entrant.Face() == nil {
		return false
	}
	f := entrant.Face()
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.Event == "Moved" && r.With != nil && r.With.API == "PutCounter" &&
			effects.PutCounterOf(r.With).ETB &&
			entryBodyKindEncodable(r.With) {
			return true
		}
	}
	if len(e.foreignEntryBodies(entrant.ID, *ev)) > 0 {
		return true
	}
	// kw:Bloodthirst and kw:Sunburst are synthesised by the replacement
	// dispatch rather than expanded onto the face (engine/rules/replacement.go), so
	// a granted or printed keyword carries no face Repl to scan.
	if ev.Kind == events.MoveZone {
		if _, ok := e.derivedKeywordParamH(ev.Obj, kwhBloodthirst); ok {
			return true
		}
		if _, ok := e.derivedKeywordParamH(ev.Obj, kwhSunburst); ok {
			return true
		}
	}
	// An ETB clone whose conditional-keyword rider grants entry counters
	// (Flesh Duplicate's vanishing, CR 707.9b) stages this entry too; see
	// entryBodyCounterGrants's rider scan for the derivation.
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.Event != "Moved" || r.With == nil || r.With.API != "Clone" ||
			r.ParamStr(cards.PKKeyword) != "ETBReplacement" {
			continue
		}
		tpl := e.cloneRiderTemplate(ev.Obj)
		if tpl == nil {
			continue
		}
		for _, kw := range effects.CloneIfNew(r.With) {
			if tpl.Face().HasKeyword(cards.KeywordHead(kw)) {
				continue
			}
			if counters, complete := cards.KeywordEntryCounters(kw); complete && len(counters) > 0 {
				return true
			}
		}
	}
	return false
}

// cloneRiderTemplate is the chosen copy template of an entry whose as-enters
// clone election was answered for a template (the decline carries none), or
// nil. The template's printed face is what CR 707.9b's conditional compares.
func (e *Engine) cloneRiderTemplate(entrant state.ObjID) *state.Object {
	o := e.G.Obj(entrant)
	if o == nil || !o.ETBCloneChoiceValid {
		return nil
	}
	tpl := e.G.Obj(o.ETBCloneChoice)
	if tpl == nil || tpl.Face() == nil {
		return nil
	}
	return tpl
}

// entryCopyBasisFace is the face the entering object is about to have: the
// answered as-enters clone template's copiable face while ClonePermanent has
// not installed it (CR 707.2), else o's own face.
func (e *Engine) entryCopyBasisFace(o *state.Object) *cards.Face {
	if o == nil {
		return nil
	}
	if tpl := e.cloneRiderTemplate(o.ID); tpl != nil {
		return tpl.CopiableFace()
	}
	return o.Face()
}

// entryBodyKindEncodable reports whether a PutCounter|ETB$ True body's
// counter kind can be folded into the entry move. The MoveZone Pairs payload
// carries the four table kinds by fixed index and every other kind by a
// length-prefixed UTF-8 payload (events.EntryCounterPairs), so any non-empty
// kind the parser produces is encodable and may be absorbed. It stays a named
// gate so the absorption walk and the cheap pre-pass read one predicate, and
// so an empty kind (a body the parser could not name) still fails closed.
func entryBodyKindEncodable(sa *cards.SA) bool {
	if sa == nil {
		return false
	}
	return events.EntryCounterKindEncodable(entryBodyKind(effects.PutCounterOf(sa)))
}

// entryBodyAbsorbable reports whether a replacement body is the bare
// self-entry counter shape this engine may fold into the entry move: a single
// DB$ PutCounter | ETB$ True on the entering object itself with a resolvable
// CounterNum$ and one counter kind. Anything carrying a rider (SubAbility$),
// an asking modifier (Optional$/Choices$/Bolster$/Support$/Adapt$/
// Monstrosity$), a per-recipient count or a composite kind is left to the
// ordinary body path -- the conservative direction, so a body this build
// cannot fully fold never loses its own resolution.
func entryBodyAbsorbable(sa *cards.SA) bool {
	return entryBodyAbsorbableFor(sa, false)
}

// entryBodyAbsorbableFor is entryBodyAbsorbable with the referent made
// explicit: the entering object's own body names Defined$ Self, a body that
// another permanent's replacement lends it (Metallic Mimic's "enters with an
// additional +1/+1 counter") names Defined$ ReplacedCard.
func entryBodyAbsorbableFor(sa *cards.SA, foreign bool) bool {
	if sa == nil || sa.API != "PutCounter" {
		return false
	}
	// A condition-gated body stays on the ordinary body path: the live
	// Resolve evaluates the gate and SKIPS the body when it is not met, a
	// verdict the fold cannot reproduce by adding CounterNumResolved to the
	// grant (Vampire Socialite's "as long as an opponent lost life this
	// turn" rider folded unconditionally on a board where no opponent has).
	if entryBodyConditionGated(sa) {
		return false
	}
	pc := effects.PutCounterOf(sa)
	if !pc.ETB || sa.Sub != nil {
		return false
	}
	ref := effects.DefinedRefOf(sa)
	if foreign {
		if ref.Text != "ReplacedCard" {
			return false
		}
	} else if !ref.Is(effects.RefSelf) {
		return false
	}
	if !pc.CounterNum.Present || pc.EntryFoldBlocked {
		return false
	}
	kind := entryBodyKind(pc)
	if strings.Contains(kind, ",") || strings.EqualFold(kind, "EachFromSource") {
		return false
	}
	return true
}

// entryBodyConditionGated reports whether a PutCounter|ETB$ True body carries
// a resolution-condition gate: any Condition* key effects.Resolve's
// conditionMet evaluates, the DB-body PresentDefined$/IsPresent$/
// PresentCompare$ spellings conditionMetCore normalizes, or an unsupported
// Condition* key (which Resolve skips loudly, engine/effects/registry.go). The entry
// fold must leave such a body on the ordinary body path -- the conservative
// direction entryBodyAbsorbable promises -- because Resolve may skip it
// entirely, a verdict the fold cannot reproduce from CounterNumResolved alone.
// One reader, so every absorbability check shares it.
func entryBodyConditionGated(sa *cards.SA) bool {
	ap := effects.ActivationOf(sa)
	return ap.Cond.Any() || ap.PresentDefined != "" || ap.IsPresent.Text != "" ||
		ap.PresentCompare.Text != "" || ap.Has(effects.ActConditionOther)
}

// entryBodyAbsorbableMatch is the one selection predicate the entry fold's
// walks share: a matched Moved replacement the live dispatch would run as an
// Updated battlefield-entry counter (ReplacementResult$ Updated,
// Destination$ Battlefield) whose body is the bare self/lent PutCounter shape
// (entryBodyAbsorbableFor). foreignEntryBodies and absorbMatched both read it,
// so the fold's selection cannot drift from the live dispatch's.
func entryBodyAbsorbableMatch(r cards.Repl, foreign bool) bool {
	if r.ParamStr(cards.PKReplacementResult) != "Updated" || r.ParamStr(cards.PKDestination) != "Battlefield" {
		return false
	}
	return entryBodyAbsorbableFor(r.With, foreign)
}

// entryBodyKind is a PutCounter|ETB$ True body's counter kind as the entry
// fold names it: CounterType$ trimmed, "P1P1" when absent or empty.
func entryBodyKind(pc *effects.PutCounterParams) string {
	kind := strings.TrimSpace(pc.CounterType)
	if kind == "" {
		kind = "P1P1"
	}
	return kind
}

// entryBodyCounterGrants returns the body-defined entry-counter grants an
// entry's Updated PutCounter|ETB$ True replacement bodies would place on the
// entering object itself, plus each body's replIdentity so the Updated
// dispatch can skip running it (its placement is folded into the move
// instead). e is the ISOLATED post-entry preview: the body's count reads the
// board it would see after the move (X paid, a Count$ head over the settled
// permanent), and the replacement matcher gates it exactly as the live
// dispatch's collection would (a CheckSVar$/SVarCompare$ gate fails closed).
// A body whose count resolves to zero (Solemnity, an X of zero) is still
// absorbed -- its placement is nothing, and running it would only duplicate
// the zero -- but contributes no grant.
func (e *Engine) entryBodyCounterGrants(ev events.Event, entrant state.ObjID) ([]entryGrant, []string) {
	if ev.Kind == events.MoveZone && (ev.To != state.ZBattlefield || events.IsFaceDownEntry(ev.Counter)) {
		return nil, nil
	}
	o := e.G.Obj(entrant)
	if o == nil || o.Face() == nil {
		return nil, nil
	}
	f := o.Face()
	matchEv := ev
	if ev.Kind != events.MoveZone {
		matchEv = events.Event{Kind: events.MoveZone, Obj: entrant, From: state.ZLibrary, To: state.ZBattlefield}
	}
	var grants []entryGrant
	var ids []string
	var absorb func(m replMatch)
	absorbMatched := func(m replMatch, foreign bool) {
		if !entryBodyAbsorbableMatch(*m.repl, foreign) ||
			!entryBodyKindEncodable(m.repl.With) {
			return
		}
		// Recognised shape: absorb it whether or not the count resolves, so
		// its placement is never run twice. An unresolvable count degrades to
		// zero exactly as the ordinary body's Num would, so nothing is lost.
		ids = append(ids, replIdentity(m))
		pc := effects.PutCounterOf(m.repl.With)
		n, ok := pc.CounterNumResolved(e, e.replCtx(m, ev))
		if !ok || n <= 0 {
			return
		}
		kind := entryBodyKind(pc)
		body := entrant
		if foreign {
			body = m.id
		}
		// CR 614.1c / 122.6: every counter the object enters with is placed
		// by ONE event, so a same-kind body (its own etbCounter and a
		// Metallic Mimic rider) folds into one grant and a "that many plus
		// one" AddCounter replacement (Hardened Scales) applies once. Only a
		// lent (foreign) rider merges; an object's own separate bodies keep
		// their one-grant-each shape (the staged order asks pin it).
		if foreign {
			for gi := range grants {
				if grants[gi].kind == kind {
					grants[gi].amount += n
					return
				}
			}
		}
		grants = append(grants, entryGrant{kind: kind, amount: n, body: body})
	}
	absorb = func(m replMatch) {
		if e.replacementMatches(*m.repl, m.id, matchEv) {
			absorbMatched(m, false)
		}
	}
	absorbForeign := func(m replMatch) { absorbMatched(m, true) }
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.Event != "Moved" || r.With == nil || r.With.API != "PutCounter" ||
			!effects.PutCounterOf(r.With).ETB {
			continue
		}
		absorb(replMatch{id: entrant, face: f, repl: r})
	}
	// The ETB clone's conditional-keyword riders (effects CloneParams.IfNew;
	// Flesh Duplicate's `AddKeywords$ IfNew Vanishing:3` -- "vanishing 3 if
	// that creature doesn't have vanishing", CR 707.9b) grant their counters
	// here rather than through the basis face, because the clone body applies
	// the copy face only AFTER the move folds (composeUpdatedReplacements:
	// the Updated bodies run post-fold), so a repl on the swapped basis face
	// is never collected for this entry. The rider's own expansion names the
	// counters (cards.KeywordEntryCounters), the condition compares the
	// CHOSEN template's printed face -- the copiable characteristics the
	// fold will snapshot -- and the grant folds into the move's Pairs
	// payload exactly as a printed body's would. The granted keyword's
	// repl-based machinery (upkeep and last-counter triggers) rides the
	// basis face the fold applies; only the entry placement needed this
	// bridge.
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.Event != "Moved" || r.With == nil || r.With.API != "Clone" ||
			r.ParamStr(cards.PKKeyword) != "ETBReplacement" ||
			!e.replacementMatches(*r, entrant, matchEv) {
			continue
		}
		tpl := e.cloneRiderTemplate(entrant)
		if tpl == nil {
			continue
		}
		for _, kw := range effects.CloneIfNew(r.With) {
			if tpl.Face().HasKeyword(cards.KeywordHead(kw)) {
				// The copied template already carries the keyword: the
				// exception does not apply (CR 707.9b), and its own printed
				// machinery is the one grant.
				continue
			}
			counters, complete := cards.KeywordEntryCounters(kw)
			if !complete {
				continue
			}
			for _, c := range counters {
				if c.N <= 0 || !events.EntryCounterKindEncodable(c.Kind) {
					continue
				}
				grants = append(grants, entryGrant{kind: c.Kind, amount: c.N})
			}
		}
	}
	// A minted token's foreign entry bodies match the synthetic ZCeased-origin
	// move applyTokenEntryUpdates reads, so an Origin$-scoped line absorbs
	// exactly when its live run would have matched.
	foreignEv := matchEv
	if ev.Kind != events.MoveZone {
		foreignEv.From = state.ZCeased
	}
	for _, m := range e.foreignEntryBodies(entrant, foreignEv) {
		if !e.replacementMatches(*m.repl, m.id, foreignEv) {
			continue
		}
		absorbForeign(m)
	}
	for _, m := range e.effectEntryBodies(foreignEv) {
		absorbForeign(m)
	}
	if m := e.bloodthirstEntryMatch(matchEv); m != nil {
		absorb(*m)
	}
	if m := e.sunburstEntryMatch(matchEv); m != nil {
		absorb(*m)
	}
	return grants, ids
}

// foreignEntryBodies lists the mandatory Moved PutCounter|ETB$ True lines of
// OTHER permanents that target the entering object as Defined$ ReplacedCard
// (Metallic Mimic, Grumgully, Zameck-style "enters with an additional
// counter" riders): candidates by shape only, in the replacement scan's
// deterministic order. The caller matches them against the entry.
func (e *Engine) foreignEntryBodies(entrant state.ObjID, ev events.Event) []replMatch {
	var out []replMatch
	e.forEachReplacementSourceFor(replEventBits[cards.ReplMoved], func(id state.ObjID) {
		if id == entrant {
			return
		}
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			return
		}
		f := e.replacementFace(id, ev)
		if f == nil {
			return
		}
		for i := range f.Repls {
			r := &f.Repls[i]
			if r.Event != "Moved" || r.With == nil || r.OptionalValue() ||
				!entryBodyAbsorbableMatch(*r, true) {
				continue
			}
			out = append(out, replMatch{id: id, face: f, repl: r})
		}
	})
	return e.dropAppliedReplacements(out)
}

// effectEntryBodies lists the Moved PutCounter|ETB$ True replacements an
// Effect created (Recommission's "it enters with an additional +1/+1
// counter") that match the entry ev and are the bare lent-body shape, in
// active-effect order. They are the Effect-borne twin of foreignEntryBodies:
// the same absorbable-shape gate, matched the way the live dispatch matches an
// Effect-created line, and keyed "effect:<source>:<timestamp>" exactly as it
// keys them so replIdentity (bodyAbsorbed) keeps the Updated dispatch from
// running an absorbed body a second time.
func (e *Engine) effectEntryBodies(ev events.Event) []replMatch {
	if !e.effectReplacementsPossible() {
		return nil
	}
	var out []replMatch
	ceL := e.active()
	for i := range ceL {
		ce := &ceL[i]
		if ce.ReplacementEvent != "Moved" {
			continue
		}
		with := replacementBodySA(ce.ReplacementBody)
		if with == nil {
			continue
		}
		r := &cards.Repl{Event: ce.ReplacementEvent, Params: ce.ReplacementParams, With: with}
		if r.OptionalValue() || !entryBodyAbsorbableMatch(*r, true) {
			continue
		}
		cc := chosenCapture{ids: ce.Chosen, bound: ce.ChosenBound}
		if !e.replacementMatchesEffectCreatedByCap(*r, ce.Source, ev, ce.Remembered, ce.RememberedPlayers, ce.Controller, cc) {
			continue
		}
		out = append(out, replMatch{id: ce.Source, repl: r, remembered: ce.Remembered,
			rememberedPlayers: ce.RememberedPlayers,
			chosenCards:       ce.Chosen, chosenBound: ce.ChosenBound,
			chosen: ce.ChosenNumber, controller: ce.Controller, frozenController: true,
			key: effectReplKey(ce)})
	}
	return e.dropAppliedReplacements(out)
}

// entryRiderCandidates reports whether an entry might carry an AddsCounters$
// mana-spend rider grant: the entering object is a card moving onto the
// battlefield (never a token mint, which was never cast) whose cast consumed
// rider-bearing mana. It is the cheap gate the emit pre-pass and foldEntryMove
// take BEFORE building the costly isolated preview, and it reads only the
// persisted grant list.
func (e *Engine) entryRiderCandidates(ev *events.Event) bool {
	if ev.Kind != events.MoveZone || ev.To != state.ZBattlefield || events.IsFaceDownEntry(ev.Counter) {
		return false
	}
	o := e.G.Obj(ev.Obj)
	return o != nil && o.Zone != state.ZBattlefield && len(o.ManaAddsCounterGrants) > 0
}

// entryRiderGrants returns the AddsCounters$ mana-spend rider grants the
// entering object's cast earned: for each grant the cast recorded
// (Object.ManaAddsCounterGrants -- the producing ABILITY's rider snapshot and
// the number of its mana units the payment spent), match the stored filter
// against the entering permanent and evaluate the stored amount. Each of the
// grant's spent units yields its own entryGrant, so two units spent from one
// rider ability place the rider twice; the rider is never re-read from the
// source's current face, so a copied, modified or added/removed ability
// cannot change the grant. A filter miss or an unresolvable amount fails
// closed (no grant), never a guessed one.
func (e *Engine) entryRiderGrants(entrant state.ObjID) []entryGrant {
	o := e.G.Obj(entrant)
	if o == nil || len(o.ManaAddsCounterGrants) == 0 {
		return nil
	}
	you := o.Controller
	var grants []entryGrant
	for _, g := range o.ManaAddsCounterGrants {
		if g.Count <= 0 || !effects.MatchesSpec(e.G, g.Filter, entrant, you) {
			continue
		}
		n, ok := e.riderCounterAmount(g.Amount, entrant, you)
		if !ok || n <= 0 {
			continue
		}
		for i := int32(0); i < g.Count; i++ {
			grants = append(grants, entryGrant{kind: g.Kind, amount: n})
		}
	}
	return grants
}

// parseAddsCounters splits a mana ability's AddsCounters$ value into its
// filter, counter kind and count expression. Forge writes
// "<filter>_<kind>_<amount>" -- Opal Palace's
// "Card.YouOwn+IsCommander_P1P1_ManaAddsCounterNum", Biophagus's
// "Card.Creature_P1P1_1", Animal Attendant's "Creature.nonHuman_P1P1_1" --
// and the measured corpus carries exactly that three-part shape, so a value
// with fewer than three parts fails closed rather than inventing a counter.
// The split is from the RIGHT so a filter containing an underscore would still
// parse; no measured carrier has one.
func parseAddsCounters(v string) (filter, kind, amount string, ok bool) {
	i := strings.LastIndexByte(v, '_')
	if i < 0 {
		return "", "", "", false
	}
	amount = strings.TrimSpace(v[i+1:])
	rest := v[:i]
	j := strings.LastIndexByte(rest, '_')
	if j < 0 {
		return "", "", "", false
	}
	kind = strings.TrimSpace(rest[j+1:])
	filter = strings.TrimSpace(rest[:j])
	if filter == "" || kind == "" || amount == "" {
		return "", "", "", false
	}
	return filter, kind, amount, true
}

// riderCounterAmount resolves an AddsCounters$ amount already snapshotted at
// the cast's payment: a signed integer literal is the count, and anything else
// is an SVar BODY resolved against the producing source's table at capture
// time, evaluated at entry with the entering permanent as the source and its
// controller as "you" (Opal Palace's ManaAddsCounterNum ->
// Count$CommanderCastFromCommandZone, read off the log for that player). The
// verdict is false only when the body does not evaluate, so the caller can
// fail closed rather than place a guessed zero. No source face is read here:
// the cast-time snapshot is authoritative.
func (e *Engine) riderCounterAmount(amount string, entrant state.ObjID, you state.PlayerID) (int32, bool) {
	if n, err := strconv.Atoi(amount); err == nil {
		return int32(n), true
	}
	ctx := effects.NewCtxPtr(entrant, you, effects.CtxInit{})
	return effects.EvalCountOK(e, ctx, amount)
}

// entryGrantPlan is the entry's whole counter plan: its intrinsic grants
// (events.EntryCounterGrants, read on the ORIGIN board) plus the body-defined
// grants its Updated PutCounter|ETB$ True bodies would place (read on the
// post-entry preview). preview is the isolated post-entry board; entrant is
// the entry's battlefield object id. bodyIDs names each absorbed body by
// replIdentity for the Updated dispatch to skip.
func (e *Engine) entryGrantPlan(ev events.Event, preview *Engine, entrant state.ObjID) ([]entryGrant, []string) {
	var grants []entryGrant
	for _, g := range e.entryCounterGrants(&ev) {
		grants = append(grants, entryGrant{kind: g.Kind, amount: g.Amount})
	}
	// AddsCounters$ mana-spend rider (task opalp): the cast's consuming sources,
	// re-read at entry. Read on e (the origin-zone object carries the links),
	// before the body grants so the placement order is deterministic.
	grants = append(grants, e.entryRiderGrants(entrant)...)
	if preview == nil {
		return mergeEntryGrants(grants, nil), nil
	}
	bodyGrants, bodyIDs := preview.entryBodyCounterGrants(ev, entrant)
	return mergeEntryGrants(grants, bodyGrants), bodyIDs
}

// mergeEntryGrants is CR 614.1c / 122.6 for the plan's two lists: every
// counter an object enters with is put by ONE event, so a body grant folds
// into the first base grant (intrinsic, requested or rider) of its kind and a
// "that many plus one" AddCounter replacement (Hardened Scales, Kami of
// Whispered Hopes) applies once to the whole put. Two grants of one list do
// not fold into each other: an entrant's own separate keyword grants (Riot
// and Unleash) and its own separate bodies keep one grant each, the shape the
// staged order asks pin (a lent body already merged into the earlier body
// grant in entryBodyCounterGrants). A merged grant keeps a body source when
// either side had one, so the put still reads as effect-caused (EffectOnly$
// gates).
func mergeEntryGrants(base, bodies []entryGrant) []entryGrant {
	out := base
	for _, g := range bodies {
		merged := false
		for i := range base {
			if out[i].kind == g.kind {
				out[i].amount += g.amount
				if out[i].body == 0 {
					out[i].body = g.body
				}
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, g)
		}
	}
	return out
}

// bodyAbsorbed reports whether m's placement was folded into the entry move
// (its replIdentity is in the plan's absorbed set), so the Updated dispatch
// must not run its body a second time.
func bodyAbsorbed(absorbed []string, m replMatch) bool {
	if len(absorbed) == 0 {
		return false
	}
	return slices.Contains(absorbed, replIdentity(m))
}

// a cloned game with the entry move applied, a private log and a private
// replacement-choice queue, so no speculative event, ask or pose can leak
// into the real chain. The returned entrant is the object ID the entry will
// carry on the battlefield (a token mint's deterministic next ID).
func (e *Engine) entryPreview(ev events.Event) (*Engine, state.ObjID) {
	// The struct is e's reusable preview when it is free, and the object
	// arena comes from e's snapshot-arena pool; both go back when the caller
	// is done with the preview (releaseEntryPreview).
	pp := e.previewStruct()
	*pp = *e
	preview := pp
	isolatePreviewMemos(preview)
	isolatePreviewScratch(preview)
	preview.G = e.G.CloneInto(e.takeSnapshotObjs())
	// The livelock watcher's window and candidate index are slices: the
	// preview copies them on its first observed event rather than writing
	// into the live engine's.
	preview.loop.shared = true
	// The preview is a speculative engine: an ask it logs stays private
	// (below) and is never a tape ask of the live engine's resolution, so it
	// carries no kernel state (a struct copy would share the live run).
	preview.tape = resolve.Kernel{}
	// A competing AddCounter choice can log an ask during the preview. Keep
	// both its log and its queue private; no speculative event may leak into
	// the real chain. Preserve prior events for log-backed counter predicates.
	// The prior events are SHARED, capped at their length (the fork
	// events.Log.Clone makes): stored events are append-only history, so the
	// preview reads the live prefix in place, and a preview append -- rare,
	// only a competing AddCounter ask -- regrows into a private array and can
	// never write into the live log's. Copying the whole log here was a
	// per-entry O(log) copy (a measured top allocator of the search loop).
	shadow := *e.L
	shadow.Events = e.L.Events[:len(e.L.Events):len(e.L.Events)]
	// A preview taken while the live log re-executes a tape resolution must
	// not inherit its verify window: the preview's private appends are new
	// history of its own, never a check against the recorded events.
	shadow.VerifyClose()
	preview.L = &shadow
	preview.replChoices = append([]replChoice(nil), e.replChoices...)
	// TokenCreate/CardToken mint in their own Apply fold rather than
	// emitting a MoveZone. The preview reveals their deterministic new ID;
	// use that ID for the same replacement and prohibition path as a card.
	entrant := ev.Obj
	if ev.Kind == events.TokenCreate || ev.Kind == events.CardToken {
		entrant = e.G.NextID
	}
	// Pin the live action cause before the preview's Apply takes the entrant
	// off the cloned stack: the entry's grants are put by whatever resolving
	// spell or ability moved the object, and the EffectOnly class (Doubling
	// Season) reads that cause. Live engines never carry a pin.
	preview.causePin = e.actionCause()
	stripEntryRequests(&ev)
	events.Apply(preview.G, ev)
	preview.replArenaNoteApplied(ev)
	preview.refreshEntryPreviewStatics()
	return preview, entrant
}

// refreshEntryPreviewStatics re-reads the preview's static-continuous memo and
// layer-4 types table after the entry move was applied. Both are keyed on the
// log length, and the preview's bare Apply appends nothing, so without this
// they serve the PRE-entry board and a foreign entry rider's filter judges the
// entrant without its own static abilities (a second Metallic Mimic is not yet
// the chosen type). CR 614.12: the replacements that apply to a permanent
// entering are decided against its characteristics as it would exist on the
// battlefield, its own statics included. Only the preview's private memos move.
func (e *Engine) refreshEntryPreviewStatics() {
	e.staticEpoch = -1
	// The layer-4 probe cache behind staticsMayChangeTypes is log-keyed too;
	// the entrant just changed zone, so its cell is stale. A negative epoch
	// makes the next read re-probe the board (into the preview's own array).
	e.typesProbeEpoch = -1
	e.refreshStaticContinuous()
	e.refreshDerivedTypesFull(len(e.L.Events))
}

// isolatePreviewMemos gives an entry preview -- a shallow struct copy of the
// live engine -- private storage for the layer caches its reads rebuild in
// place, so a preview read can never write into the live engine's arrays.
// Without it the preview's static-memo rebuild (refreshStaticContinuous, over
// the post-entry board) overwrote the live memo's entries under the live
// engine's unchanged key: a lord destroyed earlier in the same resolution
// lost its entry, the live refresh then judged the lord's departure cold and
// re-stamped the memo, and the live active list kept the lord's buff (CR
// 611.3a; Stroke of Midnight on Empyrean Eagle, dirty-dirac K1l pairs 21961
// 37823 66869 74406). The served lists are copied, so every cache hit the
// preview inherits stays valid; the pure scratch buffers start empty; the
// per-object memo tables start empty exactly as a clone's do (their clone
// tags are reset).
func isolatePreviewMemos(p *Engine) {
	p.staticContinuous = slices.Clone(p.staticContinuous)
	p.staticGates = slices.Clone(p.staticGates)
	p.activeBuf = slices.Clone(p.activeBuf)
	p.activeKWHeads = slices.Clone(p.activeKWHeads)
	p.typesProbe = slices.Clone(p.typesProbe)
	p.typesSrcs = slices.Clone(p.typesSrcs)
	p.typesMayDiffer = slices.Clone(p.typesMayDiffer)
	p.renames = slices.Clone(p.renames)
	p.activeBufAlt, p.activeSrc = nil, nil
	p.typesTouch, p.typesAct, p.derivedTouched, p.boardScanBuf = nil, nil, nil, nil
	p.derivedMemo, p.derivedMemoStack = derivedMemoTable{}, derivedMemoTable{}
	p.typesMemo = typesMemoTable{}
	p.lossMemo, p.manaGate = abilityLossMemo{}, manaGateMemo{}
}

// isolatePreviewScratch gives an entry preview private storage for every
// other slice it inherits from the live engine by its shallow struct copy,
// so nothing the preview does -- an Effect ended through the in-place filter
// of the continuous registry, a rewritten pending-trigger queue, a refilled
// scratch buffer, an append into a ledger's spare capacity -- can write into
// a live array (CR 611.3a: the live game's effects are its own). Live state
// the preview may read is copied; pure scratch buffers and caches, which a
// clone also starts empty (their clone tags are reset), start empty.
// TestEntryPreviewSharesNoAppendableSlice holds the list complete. The
// engine's maps are still shared by reference: no preview path writes one
// (TestEntryPreviewNeverWritesALiveMap holds that), so a preview that starts
// to needs a private copy here.
func isolatePreviewScratch(p *Engine) {
	// Live state: copied.
	p.continuous = slices.Clone(p.continuous)
	p.controlGrants = slices.Clone(p.controlGrants)
	p.setupEntryDrops = slices.Clone(p.setupEntryDrops)
	p.pendingTriggers = slices.Clone(p.pendingTriggers)
	p.layer4Types = slices.Clone(p.layer4Types)
	p.activationsThisTurn = slices.Clone(p.activationsThisTurn)
	p.combatHitsThisTurn = slices.Clone(p.combatHitsThisTurn)
	p.counterAddsThisTurn = slices.Clone(p.counterAddsThisTurn)
	p.turnStartTurns = slices.Clone(p.turnStartTurns)
	p.turnsTaken = slices.Clone(p.turnsTaken)
	p.copyMintsPending = slices.Clone(p.copyMintsPending)
	p.cmdZone = slices.Clone(p.cmdZone)
	p.cmdZoneDue = slices.Clone(p.cmdZoneDue)
	p.defeatedCasts = slices.Clone(p.defeatedCasts)
	p.deferredAsks = slices.Clone(p.deferredAsks)
	p.suspendedCasts = slices.Clone(p.suspendedCasts)
	p.manaExpended = slices.Clone(p.manaExpended)
	p.damageBatchLog = slices.Clone(p.damageBatchLog)
	p.discardBatchLog = slices.Clone(p.discardBatchLog)
	p.millBatchLog = slices.Clone(p.millBatchLog)
	p.zoneBatchLog = slices.Clone(p.zoneBatchLog)
	p.entryHeld = slices.Clone(p.entryHeld)
	p.declaredAttackers = slices.Clone(p.declaredAttackers)
	p.declaredDefenders = slices.Clone(p.declaredDefenders)
	p.subOfferParent = slices.Clone(p.subOfferParent)
	p.effectMatchRemembered = slices.Clone(p.effectMatchRemembered)
	p.replDrawChain = slices.Clone(p.replDrawChain)
	p.replExclude = slices.Clone(p.replExclude)
	p.replRemembered = slices.Clone(p.replRemembered)
	p.replReplacedCards = slices.Clone(p.replReplacedCards)
	p.auraEntryCands = slices.Clone(p.auraEntryCands)
	p.specEnvs = slices.Clone(p.specEnvs)
	p.batchReplSources = slices.Clone(p.batchReplSources)
	p.secretVoteBallots = slices.Clone(p.secretVoteBallots)
	p.lifeLossBatch = slices.Clone(p.lifeLossBatch)
	p.ManaSpentAddsCounters = slices.Clone(p.ManaSpentAddsCounters)
	p.ManaSpentAddsKeywords = slices.Clone(p.ManaSpentAddsKeywords)
	p.ManaSpentSources = slices.Clone(p.ManaSpentSources)
	p.PaymentPlanCarriers = slices.Clone(p.PaymentPlanCarriers)
	p.PaymentPlanRelaxed = slices.Clone(p.PaymentPlanRelaxed)
	// The zone summaries' entries carry id lists rebuilt in place.
	if p.replZones != nil {
		p.replZones = copyReplZones(nil, p.replZones)
	}
	if p.trigZones != nil {
		p.trigZones = copyTrigZones(nil, p.trigZones)
	}
	// Scratch buffers and caches: empty, as in a clone. walkObjCls is
	// owner-checked (walk_objclass.go) and would reset on first use anyway.
	p.prioMemo = nil
	p.staticQueueBuf, p.activeStaticsCache, p.combatStatics = nil, nil, nil
	p.foreachBuf, p.graveCandBuf, p.intentBuf, p.sbaIDBuf = nil, nil, nil, nil
	p.legalOptBuf, p.manaAbBuf, p.mayPlaysCache, p.layer5Colors = nil, nil, nil, nil
	p.targetCensusBuf, p.trigVerifyBuf, p.trigVerifyBuf2 = nil, nil, nil
	p.staticZones, p.staticZoneVerified, p.triggerObjectMasks = nil, nil, nil
	p.walkObjCls, p.walkClsOwner = nil, nil
	p.ManaAbScratch = nil
}

// settleEntryGrants walks the entry's grants in order against the previewed
// entry, returning the finalized amounts. A second return of true reports
// the first grant whose AddCounter competition parked a CR 616.1 order
// choice on the PREVIEW: the pose is appended to preview.replChoices for the
// caller to extract (everything from the length the caller snapshotted).
func (e *Engine) settleEntryGrants(entrant state.ObjID, grants []entryGrant) ([]events.EntryCounterGrant, int) {
	var placed []events.EntryCounterGrant
	for i, g := range grants {
		if g.amount <= 0 || e.PutCounterBlocked(g.kind, entrant, 0, false) {
			continue
		}
		counter := events.Event{Kind: events.CounterChange, Obj: entrant, Counter: g.kind, Amount: g.amount}
		// A body-defined grant is settled with the replacement-body provenance
		// the body itself would carry (applyingReplacement + replacingSource),
		// so the AddCounter matcher's EffectOnly$/ValidSource$ gates read the
		// same cause they read on the live body path (Doubling Season,
		// Vorinclex). An intrinsic grant keeps its own provenance -- the
		// preview's pinned action cause.
		savedApplying, savedSource := e.applyingReplacement, e.replacingSource
		if g.body != 0 {
			e.applyingReplacement, e.replacingSource = true, g.body
		}
		rewritten, handled := e.applyReplacements(counter)
		e.applyingReplacement, e.replacingSource = savedApplying, savedSource
		if handled {
			// A noncommuting CR 616.1 choice must remain a real decision;
			// the pose sits at the tail of the preview's private queue.
			return placed, i
		}
		if rewritten.Amount > 0 {
			placed = append(placed, events.EntryCounterGrant{Kind: g.kind, Amount: rewritten.Amount})
		}
	}
	return placed, -1
}

// extractEntryPose returns the AddCounter order pose the preview appended
// during one grant's settlement. Poses append at the tail of the preview's
// private queue; only poses appended after n0 belong to this settlement.
func extractEntryPose(preview *Engine, n0 int) *replChoice {
	for i := len(preview.replChoices) - 1; i >= n0; i-- {
		if preview.replChoices[i].kind == replChoiceAddCounter {
			rc := preview.replChoices[i]
			return &rc
		}
	}
	return nil
}

// stageEntryCounterOrder parks ev's entry behind the CR 616.1 order choice
// the preview parked for grant idx, and asks the affected player. inRes is
// the pose's in-resolution provenance, captured by the caller at the moment
// the entry was emitted. Returns true (the caller must not fold the move).
//
// cont, when non-nil, is the stage this pose CONTINUES (resumeEntryCounterOrder
// settling the grants after an answered park): the new stage starts from a
// copy of the whole of cont, so every continuation field -- the accumulated
// placement, the absorbed-body set, the token-plan tail, and the in-body /
// in-resolution provenance the pose cannot re-derive here (applyingReplacement
// is false while an answer runs) -- carries over, taking only the new pose's
// own competition. The copy is made BEFORE the ask: a kernel-served order
// choice is answered in place, so the stage must be complete when posed.
func (e *Engine) stageEntryCounterOrder(ev events.Event, preview *Engine, n0 int, grants []entryGrant, placed []events.EntryCounterGrant, idx int, inRes bool, bodyIDs []string, cont *entryCounterStage) bool {
	posed := extractEntryPose(preview, n0)
	if posed == nil {
		return false
	}
	st := &entryCounterStage{
		move: ev, grants: grants, placed: placed,
		counter: posed.ev, cands: posed.cands, player: posed.player,
		inRes: inRes, inBody: e.applyingReplacement, idx: idx, bodyIDs: bodyIDs,
	}
	if cont != nil {
		*st = cont.restage(st)
		cont.tokenPlan = nil
	}
	e.replChoices = append(e.replChoices, replChoice{kind: replChoiceEntryOrder,
		ev: posed.ev, cands: posed.cands, player: posed.player,
		before: e.retainTriggerBefore(), inResolution: st.inRes,
		damaging: e.damaging, combatDamaging: e.combatDamaging, dmgSrcOverride: e.dmgSrcOverride,
		stage: st})
	if e.pending == nil {
		e.askReplacementChoice(st.player)
	}
	return true
}

// entryETBChoiceOutstanding reports whether ev's entry still owes an
// as-enters election (the same entryETBChoice walk the replacement dispatch
// poses from, at the ordinal that entry's pose sequence has reached). Pure:
// it only builds the next choice's options.
func (e *Engine) entryETBChoiceOutstanding(ev events.Event) bool {
	if ev.Kind != events.MoveZone || ev.To != state.ZBattlefield {
		return false
	}
	ordinal := 0
	if e.etbMove != nil {
		if e.etbMove.Obj != ev.Obj {
			return false
		}
		ordinal = e.etbNext
	}
	_, ok := e.entryETBChoice(ev, ordinal)
	return ok
}

// entryCounterOrderParks is Engine.emit's pre-pass: it reports whether ev's
// entry must stage behind an outstanding or newly posed CR 616.1 order
// choice instead of folding. A completed stage returns false -- the fold
// that re-drive reaches consumes it.
func (e *Engine) entryCounterOrderParks(ev events.Event) bool {
	for i := range e.replChoices {
		rc := &e.replChoices[i]
		if rc.kind == replChoiceEntryOrder && rc.stage != nil && rc.stage.sameEntryMove(&ev) {
			return !rc.stage.complete
		}
	}
	if e.entryStageDone != nil && e.entryStageDone.sameEntryMove(&ev) {
		return false
	}
	// An entry still owing an as-enters election stages nothing yet: the
	// election is part of defining the grant set (riot, then unleash, then a
	// name/type/colour election each append their grant on their own emit),
	// and the replacement dispatch poses it on this very emit. Staging before
	// the last election would snapshot an incomplete grant set and the
	// completed stage would fold the entry without the late grant.
	if e.entryETBChoiceOutstanding(ev) {
		return false
	}
	if len(e.entryCounterGrants(&ev)) == 0 && !e.entryBodyCandidates(&ev) && !e.entryRiderCandidates(&ev) {
		return false
	}
	preview, entrant := e.entryPreview(ev)
	defer e.releaseEntryPreview(preview)
	if o := preview.G.Obj(entrant); o == nil || o.Zone != state.ZBattlefield {
		return false
	}
	grants, bodyIDs := e.entryGrantPlan(ev, preview, entrant)
	if len(grants) == 0 {
		return false
	}
	n0 := len(preview.replChoices)
	placed, park := preview.settleEntryGrants(entrant, grants)
	if park < 0 {
		return false
	}
	return e.stageEntryCounterOrder(ev, preview, n0, grants, placed, park, e.resolvingObj != 0 || e.answerInResolution, bodyIDs, nil)
}

// resumeEntryCounterOrder answers one staged competition: the chosen body
// applies FIRST (CR 616.1's answer), the rest re-drive on an isolated
// preview of the post-entry board -- the only state the zone filters match
// on -- and a live non-commuting remainder re-poses for real. A fully
// settled stage completes: the staged move re-emits through the ordinary
// emit and its fold consumes the finalized amounts.
func (e *Engine) resumeEntryCounterOrder(rc replChoice, idx int) {
	st := rc.stage
	preview, entrant := e.entryPreview(st.move)
	defer e.releaseEntryPreview(preview)
	if o := preview.G.Obj(entrant); o == nil || o.Zone != state.ZBattlefield {
		// The entry cannot happen; the competition is moot. Re-emit the move
		// so the ordinary path records whatever the move actually does (the
		// same verdict the fold's preview takes).
		e.emit(st.move)
		return
	}
	// The chosen body applies first, on the preview.
	m := rc.cands[idx]
	if n, ok := preview.applyAddCounterBody(st.counter, m, st.counter.Amount); ok {
		st.counter.Amount = n
	}
	st.applied = append(st.applied, m)
	preview.driveAddCounterCompetition(
		replChoice{ev: st.counter, cands: st.cands, appliedRepls: st.applied},
		func(ev events.Event, applied []replMatch) {
			st.counter, st.applied = ev, applied
		},
		func(rc2 replChoice, p state.PlayerID) {
			// A live non-commuting remainder: re-pose for real against the
			// same stage.
			st.counter, st.applied = rc2.ev, rc2.appliedRepls
			e.replChoices = append(e.replChoices, replChoice{kind: replChoiceEntryOrder,
				ev: rc2.ev, cands: rc2.cands, player: p,
				before: e.retainTriggerBefore(), inResolution: st.inRes,
				damaging: e.damaging, combatDamaging: e.combatDamaging, dmgSrcOverride: e.dmgSrcOverride,
				stage: st})
			if e.pending == nil {
				e.askReplacementChoice(p)
			}
		})
	if len(e.replChoices) > 0 {
		last := &e.replChoices[len(e.replChoices)-1]
		if last.kind == replChoiceEntryOrder && last.stage == st && !st.complete {
			// The drive re-posed: the stage stays outstanding for the next
			// answer; nothing is finalized yet.
			return
		}
	}
	// The parked grant finalized.
	if st.counter.Amount > 0 {
		st.placed = append(st.placed, events.EntryCounterGrant{Kind: st.counter.Counter, Amount: st.counter.Amount})
	}
	// The grants after the parked one settle against the same preview.
	for j := st.idx + 1; j < len(st.grants); j++ {
		n0 := len(preview.replChoices)
		more, park := preview.settleEntryGrants(entrant, st.grants[j:])
		st.placed = append(st.placed, more...)
		if park < 0 {
			break
		}
		if !e.stageEntryCounterOrder(st.move, preview, n0, st.grants, st.placed, j+park, st.inRes, st.bodyIDs, st) {
			continue
		}
		// stageEntryCounterOrder built a fresh stage for the new park that
		// CONTINUES this entry (it copied st before posing).
		return
	}
	st.complete = true
	e.entryStageDone = st
	priorApplying := e.applyingReplacement
	if st.inBody {
		e.applyingReplacement = true
	}
	st.redriving = true
	e.emit(st.move)
	st.redriving = false
	if e.entryStageDone == st {
		// The re-drive never reached its fold (replaced away): the stage
		// is spent either way and must not be consumed by a later entry.
		e.entryStageDone = nil
	}
	e.applyingReplacement = priorApplying
	// A staged TOKEN mint may be only the first of a finalized plan. The
	// plan's remaining mints were suspended by emitTokenPlanMints so they
	// would not be mistaken for this stage's re-drive; continue them now that
	// this mint has folded. Consume the tail first: the continuation may park
	// on its own later mint and set a fresh stage's tail.
	if cont := st.tokenPlan; cont != nil {
		st.tokenPlan = nil
		if len(cont.plan) > 0 {
			e.emitTokenPlanMints(cont.ev, cont.plan)
		}
	}
}

// foldEntryMove is shared by the ordinary emit tail and the Updated
// replacement paths. A preview fold on an isolated Game lets counter filters
// see the entering permanent in its destination zone without changing live
// state. The finalized counter amounts travel in the entry event's Pairs
// payload (MoveZone, TokenCreate or CardToken): events.Apply installs them IN
// the entry, before any observer runs. CounterChange records after the entry
// are notification-only: they do not place counters twice on replay.
//
// *ev is folded in place -- on return it IS the stored event -- so a caller
// passes a copy it owns, never an event it still needs. tally (the ordinary
// emit tail only) brackets a Damage fold with the excess-damage tally: the
// recipient's pre-fold threshold read before, the folded amount after. Both
// tally steps are no-ops for every other kind, and a Damage event is never an
// entry (no stage, grant or preview matches it), so it folds directly.
func (e *Engine) foldEntryMove(ev *events.Event, tally bool) []string {
	if tally && ev.Kind == events.Damage {
		noteExcessHit(&e.excessBatch, e, e.damageSourceLKI, ev, e.damageBatchOpen)
		events.EmitPtr(e.G, e.L, ev)
		e.excessBatch.add(ev)
		return nil
	}
	if st := e.entryStageDone; st != nil && st.complete && st.sameEntryMove(ev) {
		e.entryStageDone = nil
		e.foldEntryWithPlaced(ev, st.placed)
		return st.bodyIDs
	}
	intrinsic := e.entryCounterGrants(ev)
	if len(intrinsic) == 0 && !e.entryBodyCandidates(ev) && !e.entryRiderCandidates(ev) {
		stripEntryRequests(ev)
		events.EmitPtr(e.G, e.L, ev)
		return nil
	}
	preview, entrant := e.entryPreview(*ev)
	if o := preview.G.Obj(entrant); o == nil || o.Zone != state.ZBattlefield {
		e.releaseEntryPreview(preview)
		stripEntryRequests(ev)
		events.EmitPtr(e.G, e.L, ev)
		return nil
	}
	grants, bodyIDs := e.entryGrantPlan(*ev, preview, entrant)
	placed, park := preview.settleEntryGrants(entrant, grants)
	e.releaseEntryPreview(preview)
	if park >= 0 {
		// Residual: only reachable where the emit pre-pass does not run --
		// an entry emitted inside a replacement body, or a TokenCreate/
		// CardToken mint. There the ordinary counter path owns its park and
		// resume, and the placements follow the move (the pre-staging
		// behaviour this task's pre-pass replaces for ordinary entries).
		// No body is absorbed here: its own placement run owns the park.
		stripEntryRequests(ev)
		events.EmitPtr(e.G, e.L, ev)
		for _, grant := range intrinsic {
			e.emit(events.Event{Kind: events.CounterChange, Obj: entrant, Counter: grant.Kind, Amount: grant.Amount})
		}
		return nil
	}
	e.foldEntryWithPlaced(ev, placed)
	return bodyIDs
}

// foldEntryWithPlaced folds the entry move with the finalized grant amounts
// in its Pairs payload, then emits each grant's notification-only
// CounterChange (EntryCounterNotice) through the ordinary emit path so
// trig:CounterAdded and the per-turn ledger see the placement exactly once.
func (e *Engine) foldEntryWithPlaced(ev *events.Event, placed []events.EntryCounterGrant) {
	stripEntryRequests(ev)
	for _, g := range placed {
		ev.Pairs = append(ev.Pairs, events.EntryCounterPairs(g)...)
	}
	entrant := ev.Obj
	if ev.Kind == events.TokenCreate || ev.Kind == events.CardToken {
		entrant = e.G.NextID
	}
	events.EmitPtr(e.G, e.L, ev)
	for _, g := range placed {
		// Replacement has already settled; the marker only notifies observers.
		// The entrant's controller is the adder of its entry counters, even
		// though this notification is an already-settled echo.
		savedApplying, savedFold := e.applyingReplacement, e.counterReplacementFold
		savedAdder := e.SetCounterAdder(e.controllerOf(entrant))
		e.applyingReplacement, e.counterReplacementFold = true, true
		e.emit(events.Event{Kind: events.CounterChange, Obj: entrant,
			Counter: g.Kind, Amount: g.Amount, Text: events.EntryCounterNotice})
		e.applyingReplacement, e.counterReplacementFold = savedApplying, savedFold
		e.SetCounterAdder(savedAdder)
	}
}

// stripEntryRequests drops the entry counters an effect REQUESTED on a move
// (effects.entryWithCounters) from the event about to fold: entryCounterGrants
// already read them into the entry's grant plan, and the plan's settled
// amounts are what the fold installs, so the request must not fold too.
func stripEntryRequests(ev *events.Event) {
	switch ev.Kind {
	case events.MoveZone, events.TokenCreate, events.CardToken:
		ev.Pairs, _ = events.SplitEntryCounterPairs(ev.Pairs)
	}
}
