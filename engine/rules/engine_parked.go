package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// engineParked groups the Engine's parked continuation windows (suspended
// casts, mana activations, commander zone, replacement choices). It is
// embedded by value in Engine (engine/rules/engine_struct.go), so every field
// keeps its documented contract comment and every existing e.<field>
// access keeps compiling unchanged through Go's field promotion. Clone's
// per-field copy classes (engine/rules/clone.go) are unchanged by the move.
type engineParked struct {
	// suspendedCasts is the mandatory "cast it if able" trigger created when
	// a real suspended card loses its final TIME counter. IDs are appended in
	// exile order and consumed before priority; it is plain replayable engine
	// continuation state, not an inference from arbitrary exile cards.
	suspendedCasts []state.ObjID `clone:"deep"`
	// defeatedCasts is the CR 310.11 "may cast it transformed without paying
	// its mana cost" offer for every battle the zero-defense SBA exiled
	// (engine/rules/sba.go's battleZeroDefense). Fed by Engine.emit's battlefield→
	// exile feed (the ONE home every defeat route shares), drained before
	// priority by startDefeatedCast; plain replayable continuation state like
	// suspendedCasts above.
	defeatedCasts []state.ObjID `clone:"deep"`
	// queuedPlays is the rest of an Amount$ Play answer (Etali, Primal
	// Storm's "you may cast any number of spells from among them") whose
	// earlier cast parked on its own question -- a target, an as-enters
	// choice, a mana window. CR 608.2g casts each chosen card in turn, each
	// cast fully completed before the next begins, so the remaining cards
	// wait here and startQueuedPlay begins the next one once nothing is
	// pending. Plain replayable continuation state like suspendedCasts.
	queuedPlays *queuedPlays `clone:"deep"`
	// manaActivation is non-nil while a source with several available mana
	// abilities waits for its controller to select one. manaColorActivation
	// similarly holds an already-paid Produced$ Any ability (the session's
	// ManaCost holds an ability whose discard cost is being chosen). All are
	// plain data so Clone preserves an offered activation.
	manaActivation       *manaActivation       `clone:"deep"`
	manaColorActivation  *manaColorActivation  `clone:"deep"`
	manaUnlessActivation *manaUnlessActivation `clone:"deep"`
	// offStackMana is the transient frame of the off-stack mana resolution
	// currently running synchronously (engine/rules/mana_activation.go's
	// offStackManaFrame). It is nil between Submits, so Clone never sees it.
	offStackMana *offStackManaFrame `clone:"reset"`
	// offStackSlots back the innermost offStackManaFrames (withOffStackMana)
	// so a mana resolution allocates no frame; offStackDepth is the nesting.
	offStackSlots [2]offStackManaFrame `clone:"reset"`
	offStackDepth int                  `clone:"reset"`
	// manaAfterCost is a mana ability whose cost is fully paid but whose
	// payment posed a decision -- a sacrificed or discarded commander's
	// CR 903.9 command-zone choice parks the move and asks its owner. The
	// mana effect (and its own colour choice) waits here until that answer
	// lands; Submit resumes it once nothing is pending (resumeManaAfterCost).
	// Plain data, deep-copied by Clone like its siblings.
	manaAfterCost *manaAfterCost `clone:"deep"`
	// deferredAsks holds decisions posed while a CR 903.9 commander-zone
	// choice was pending (see ask): they are posed in order, one at a time,
	// once nothing is pending (drainDeferredAsks, from Submit). Deep-copied
	// by Clone like pending.
	deferredAsks []*decision.Decision `clone:"deep"`
	// Resolution-time payment windows. cumulative belongs to the replayable
	// keyword trigger; triggerCost belongs to an ordinary triggered effect
	// carrying Cost$ (Mana Vault). Both are plain data and Clone-copied.
	cumulative  *cumulativeUpkeep    `clone:"deep"`
	triggerCost *triggeredEffectCost `clone:"deep"`
	// echo (engine/rules/echo.go, kw:Echo): the pay-or-sacrifice election of a
	// resolving echo keyword trigger. Same plain-data class as the two
	// above; Clone-copied.
	echo *echoFlow `clone:"deep"`

	// attackPay holds the declare-attackers attack-cost payment window
	// (engine/rules/attack_cost.go): the answered KAttackers declaration, its payer
	// and the outstanding charge, while the payer taps mana sources to cover
	// a CantAttackUnless prop. Plain data; Clone
	// copies the pointer.
	attackPay *attackPayWindow `clone:"deep"`
	// blockPay holds the declare-blockers CantBlockUnless payment window.
	blockPay *blockPayWindow `clone:"deep"`

	// cmdZone is the queue of commander command-zone asks (CR 903.9, Task m32,
	// engine/rules/replacement.go): the CR 903.9b parked hand/library moves a
	// commander is about to undergo, deferred until its owner answers the
	// KCommanderZone decision for the FRONT entry, plus at most one CR 903.9a
	// sba entry, the one being asked (commanderZoneSBA). Multiple commanders
	// can be parked by one burst (a bounce sweep, one state-based-action
	// pass), but only one decision can be pending at a time, so the queue
	// hands from one answer to the next (handleCmdZone asks the new front
	// after emitting the old one). Entries are deduplicated by object id, so
	// a re-offered SBA pass can never enqueue the same commander twice (the
	// no-progress failure shape a replacement must not spin on). Never
	// mutated while e.pending is nil except by a park that also asks; Clone
	// deep-copies it (clone.go) so a clone taken with a decision outstanding
	// carries the same queue. It is always empty in a non-Commander game:
	// nothing ever parks there.
	cmdZone []cmdZoneMove `clone:"deep"`
	// cmdZoneDue is the CR 903.9a arrival list: commanders a logged MoveZone
	// put into a graveyard or exile since the last state-based check, in log
	// order, each once (collectCommanderArrivals). commanderZoneSBA asks the
	// owner of the first still-eligible entry and drops the rest of the
	// ineligible prefix; a commander not taken stays wherever it is and is
	// not re-offered until it arrives anew. cmdZoneScan is the log index the
	// scan has reached. Both are a pure function of the log; Clone copies
	// them. Always empty/zero in a non-Commander game.
	cmdZoneDue  []state.ObjID `clone:"deep"`
	cmdZoneScan int           `clone:"deep"`
	// cmdZoneDep is the DepSeq watermark the CR 903.9a split scan has checked
	// through (collectCommanderArrivals): a split stamp at or below it is a
	// departure an earlier state-based check already consumed, so that
	// commander is not offered again; a stamp above it is a fresh arrival.
	// Set to Game.DepSeq at every scan's end. A pure function of the log like
	// cmdZoneScan; Clone copies it.
	cmdZoneDep uint32 `clone:"deep"`
	// cmdZoneDeclined is the commander whose CR 903.9b ask was just declined
	// (or whose owner has left), set only for the one synchronous re-emit of
	// that move so commanderZoneReplacementApplies skips it while every other
	// replacement still applies. Zero otherwise; Clone resets it.
	cmdZoneDeclined state.ObjID `clone:"reset"`
	// legendBatch is the parked CR 704.5j legend-rule application (engine/rules/
	// sba.go): the duplicate legendary set whose controller is choosing which
	// member to keep, everything else found in the same SBA check, and the
	// pre-batch look-back board. Parked and asked atomically by
	// parkLegendChoice; cleared and applied by legendAnswer. Parked ONLY
	// together with its ask (the pose is one step), so a clone taken at an
	// intent boundary either sees the zero value or a batch whose decision is
	// outstanding -- and must carry the batch, or answering the copied
	// decision would find nothing parked. Clone deep-copies it (clone.go),
	// the cmdZone class. Always nil outside an outstanding legend choice.
	legendBatch *legendBatch `clone:"deep"`
	// replChoices is the queue of parked replacement choices (see replChoice /
	// handleReplacement in replacement.go): CR 616.1 ordering for MoveZone,
	// Untap, ProduceMana and BeginPhase, replacement-time mana-colour choices,
	// an Optional$ BeginPhase yes/no, and the AddCounter/CreateToken/Updated
	// competitions. Plain value entries are deep-copied by
	// Clone, so every in-flight event survives an intent boundary.
	replChoices []replChoice `clone:"deep"`
}
