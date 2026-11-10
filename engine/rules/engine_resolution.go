package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// engineResolution groups the Engine's in-flight resolution context, its
// LKI/answer-cursor maps and the replacement windows. It is embedded by
// value in Engine (engine/rules/engine_struct.go), so every field keeps its
// documented contract comment and every existing e.<field> access keeps
// compiling unchanged through Go's field promotion. Clone's per-field copy
// classes (engine/rules/clone.go) are unchanged by the move.
type engineResolution struct {
	// resolutionCtx is the live Ctx of the Resolve chain whose effect is
	// CURRENTLY running, published by effects.Resolve through the optional
	// resolutionCtxHost interface around the whole chain and restored on
	// return. Read by the tape seams (unless payment, Ward, Play) and the
	// departing-target snapshots. Transient scratch: rebuilt identically by
	// replay, nil outside a chain (combat, mulligan and other non-resolution
	// asks).
	resolutionCtx *effects.Ctx `clone:"reset"`
	// exploitedLKI maps an EXPLOITED creature's object id to the LKI snapshot
	// of it at the instant it was sacrificed to pay an exploit (CR 702.58a),
	// published by engine/effects/exploit.go through Host.RememberExploitedLKI while
	// the resolving marker holds Ctx.Sacrificed. The events.Exploit marker
	// carries only the exploited id, and Move has already cleared the
	// creature's counters and battlefield layers by emit time, so this map is
	// what lets a trig:Exploited body read TriggeredExploited$CardPower/
	// CardToughness as last-known information (rules' attachExploitedLKI).
	// Engine-only and replay-derived like the other LKI maps: replay re-runs
	// the same effect resolution, so it repopulates identically, and the entry
	// is removed when the exploited object leaves a zone.
	exploitedLKI map[state.ObjID]state.SacrificedInfo `clone:"deep"`
	// sourceLifelinkLKI maps an independently resolving ability's stack object
	// to its source permanent's derived lifelink state at the last moment that
	// source existed on the battlefield. The map's presence is the validity
	// bit: false is authoritative LKI too. It is captured before a source is
	// sacrificed as its own activation cost, refreshed for already-stacked and
	// pending abilities when their source later departs, cloned at intent
	// boundaries, and removed with the stack object. Resolution copies it into
	// effects.Ctx; effects uses it only when the source is no longer live.
	sourceLifelinkLKI map[state.ObjID]bool `clone:"deep"`
	// sourceControllerLKI is the matching pre-departure controller snapshot.
	// It is separate from sourceLifelinkLKI because false lifelink is still a
	// valid snapshot, and a controller may be seat zero.
	sourceControllerLKI map[state.ObjID]state.PlayerID `clone:"deep"`
	// sourceCharLKI is the matching pre-departure snapshot of the source's
	// power, toughness and counters, read by a source-relative target
	// filter's CR 608.2b recheck (source_char_lki.go).
	sourceCharLKI map[state.ObjID]sourceCharSnapshot `clone:"deep"`
	// damageSourceLKI carries snapshots keyed first by the waiting stack
	// object and then by a departed named DamageSource$ object. Unlike the
	// own-source maps above, every waiting resolution receives departures: the
	// named source can be TriggeredCard, Targeted, or Remembered.
	damageSourceLKI map[state.ObjID]map[state.ObjID]effects.DamageSourceLKI `clone:"deep"`
	// reflexiveRememberedPT maps a reflexive ability's stack object to the
	// battlefield power/toughness its remembered objects had as it was put on
	// the stack (CR 608.2h; QueueReflexiveTrigger's capture). Resolution
	// seeds Ctx.Snap.TargetPT from it, so Thorin, Mountain-king's
	// TriggerRemembered$CardPower of a creature bounced in response reads
	// that power, not the printed power of the card in its new zone.
	reflexiveRememberedPT map[state.ObjID]map[state.ObjID]effects.TargetPT `clone:"deep"`
	// replReplaced is the ev.Obj of the replacement applyReplacements is
	// currently resolving — the object the replaced event was about. It is
	// seeded by applyReplacements (Ctx.Replaced = ev.Obj) and read by Ask to
	// thread that object across a mid-resolution suspension: a ReplaceWith$
	// body that poses an ask needs Replaced (and Remembered = [that object],
	// and the X value) restored on the resume, or its Defined$ ReplacedCard
	// resolution and SVar:X Remembered$Amount gating see nothing and the
	// completed move never happens (Mox Diamond). Zero whenever no
	// replacement is in flight.
	replReplaced state.ObjID `clone:"reset"`
	// replReplacedCards is the ordered plural batch (Ctx.ReplacedCards) of the
	// replacement currently resolving -- the cascade instruction's exiled
	// cards, the counterpart of replReplaced for Averna's Defined$
	// ReplacedCards selector. Ask captures it onto the resume point so a
	// ReplaceWith$ body that suspends at its hidden pick re-resolves
	// ReplacedCards.<qual> against the same batch. nil outside a Cascade
	// replacement.
	replReplacedCards []state.ObjID `clone:"reset"`
	// cascadeResidue is the synthetic SA effCascade wants run after a Cascade
	// replacement body (bottom the non-found exiled cards, then the free-cast
	// election). It is scoped to one ProposeCascadeReplacement call, the same
	// scratch pattern as scrySA/scryTarget; nil outside one.
	cascadeResidue *cards.SA `clone:"reset"`
	// replacingEvent is the in-flight Damage event a DB$ ReplaceEffect body's
	// ReplaceEvent call may rewrite (Amount/Affected). It exists only during
	// emit, before the event is logged, so it is never part of
	// cloned/replayed engine state.
	replacingEvent  *events.Event `clone:"reset"`
	replacingSource state.ObjID   `clone:"reset"`
	// replRemembered is the in-flight replacement body's remembered referents,
	// visible to that one body and restored right after it, the same scratch
	// pattern as replacingEvent. ReplaceEvent carries no Ctx, so the
	// VarValue$ Remembered rewrite reads its binding here. Never part of
	// cloned/replayed engine state: it lives only during the body run, before
	// the held event is logged.
	replRemembered []state.Target `clone:"reset"`
	// replAction is the action marker (events.ActionMarker) of the event the
	// in-flight destination-changing replacement discarded: "sacrificed",
	// "discarded" or "discarded as a cost". emit re-labels the replacement
	// body's move of replReplaced with it (events.CarryAction), so a
	// sacrifice or discard redirected by a replacement is still seen as that
	// action by Sacrificed/Discarded triggers. Empty whenever no such
	// replacement is in flight; threaded across a suspension by resumePoint.
	replAction string `clone:"reset"`
	// replReplacedPlayer is the player a replaced DRAW event was about (the
	// draw-er), threaded the same way replReplaced threads the replaced
	// object: a ReplaceWith$ body over R:Event$ Draw poses mid-resolution
	// asks (Breathstealer's Crypt's unless-pay discard) and the resume must
	// restore Ctx.ReplacedPlayer. Only a Draw replacement sets it.
	replReplacedPlayer state.Target `clone:"reset"`
	// replRedirect is the destination-changing ("Replaced") move replacement
	// whose ReplaceWith$ body is resolving, with every replacement already
	// applied to that event (CR 614.5). A body move of the same object to a
	// DIFFERENT zone is the modified event of CR 616.1f and gets one more
	// replacement pass that skips those (Engine.emit). Immutable once set;
	// threaded across a suspension by resumePoint.redirect; nil at every
	// intent boundary outside a suspended body.
	replRedirect *replRedirect `clone:"reset"`
	// replExclude is the applied set replRedirect carried into that one
	// recheck pass: applyReplacementsDispatch drops those matches and
	// applyReplacement extends it for a nested redirect. Nil otherwise.
	replExclude []string `clone:"reset"`
	// replDrawChain is the draw lineage's applied set (CR 614.5, 616.1e):
	// the identities (replIdentity) of every draw replacement currently
	// running on this draw's ancestry. applyReplacement pushes a draw
	// replacement's identity for the duration of its ReplaceWith$ body, so
	// every Draw the body emits -- a per-card draw or a DrawBatchProposal --
	// re-enters the replacement pass with those identities excluded while
	// the remaining replacements get their CR 616.1e opportunity (Thought
	// Reflection's two draws still meet Notion Thief, Blood Scrivener's
	// draws still meet a second Scrivener). A replacement effect that
	// already applied to an ancestor draw never applies again (CR 614.5);
	// a sibling draw of the same body starts from the same stack and may
	// meet a replacement whose own body has already finished. Empty
	// whenever no draw replacement body is in flight, so no unrelated draw
	// changes behavior. Transient scratch, the replRedirect class.
	replDrawChain []string `clone:"reset"`
}
