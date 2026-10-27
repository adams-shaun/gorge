package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// engineContinuation groups the Engine's continuation, ask-parking and
// round-ask state. It is embedded by value in Engine
// (engine/rules/engine_struct.go), so every field keeps its documented contract
// comment and every existing e.<field> access keeps compiling unchanged
// through Go's field promotion. Clone's per-field copy classes
// (engine/rules/clone.go) are unchanged by the move.
type engineContinuation struct {
	// choosing says which flow is waiting on the current KChoose decision
	// (Task 8). It is plain data, not a closure, so Engine.Clone (a sibling
	// branch, not yet in this worktree) can copy it like any other field --
	// a closure captured over this Engine's own pointers would not survive a
	// clone at all. handleChoose (turn.go) switches on it; Task 9 adds
	// chooseCast in cast.go, and Tasks 12 and 18 add the "as this enters" and
	// miracle cases in their own files.
	choosing chooseFor `clone:"deep"`

	// cleanupWindow is true while a cleanup step's CR 514.3a priority
	// window is open: grantPriority sets it whenever it hands out priority
	// during the cleanup step, and repeatCleanup and setStep clear it (a
	// new cleanup step begins, CR 514.3b, or the step is left). While it is
	// set, priorityRound runs the ordinary round instead of the cleanup
	// procedure, so the active player receives priority after a resolution
	// (CR 117.3b) and the step repeats only once all players pass in
	// succession with the stack empty (handlePriority's empty-stack pass).
	cleanupWindow bool `clone:"deep"`

	// oppSel (engine/rules/stack.go) is the TargetingPlayer$ Opponent
	// controller-selection ask's flow record: set when poseOpponentPick
	// posts the which-opponent ask at a rules-tier ask site, flipped to done
	// by answerOppPick, consumed when the re-posed target ask reads it.
	// Plain scalars, so Clone carries it like the blockerRound class.
	oppSel oppSelectState `clone:"deep"`

	// oppPicksMid is the effects-tier answered-selection store, keyed by the
	// asking SA's line: OpponentPickAsk records the controller's chosen
	// opponent there (the "opp_pick" answer) and the walk's ChooserFor
	// consumes it. The entry only lives between that answer and the read, so no
	// entry can outlive the ask it belongs to. Clone copies it.
	oppPicksMid map[string]state.PlayerID `clone:"deep"`

	// tpCtlChooser (engine/rules/stack.go) is the TargetingPlayerControls$ answered
	// record (tpc1): the seat that answered a target ask whose SA carries
	// `TargetingPlayerControls$ True`, keyed by the RESOLVING stack object
	// (pc.stackObj for a cast/activation, the TriggerPush object for a
	// placement ask) and carrying the asking SA's line. The entry lives from
	// the ask's answer until the object leaves the stack, so the CR 608.2b
	// recheck (legalTargets, which reads it via its self parameter) judges
	// the restriction against exactly the seat that answered. Clone copies it.
	tpCtlChooser map[state.ObjID]tpCtlAnswer `clone:"deep"`

	// askCount counts the mid-resolution asks Engine.Ask took, posed or
	// deferred (effects' askSeam). Transient scratch, never logged.
	askCount uint64 `clone:"reset"`
	// AbortTally and LegacyAskTally attribute the notes whose event carries
	// no source object (CV-6): note text <US> attribution label -> count.
	// abortCast knows the pending cast's card (the Note event does not), and
	// Engine.Ask knows the refused decision's source; the cardval note
	// census reads both tallies at game end. Deep-cloned, not reset: the
	// tape kernel's checkpoint Restore (a Clone) must not drop the counts of
	// the log prefix the restore keeps (engine/rules/note_tally.go).
	AbortTally     map[string]int `clone:"deep"`
	LegacyAskTally map[string]int `clone:"deep"`
	// resolvingObj is the stack object whose resolution is running (resolveTop
	// or a resumed resolution), kept through its final move off the stack so
	// an entry replacement that asks can tell whether it interrupted that
	// resolution's own move. Zero outside a resolution.
	resolvingObj state.ObjID `clone:"reset"`
	// cast holds the in-progress cast-flow state while choosing ==
	// chooseCast (Task 9, engine/rules/cast.go). Nil whenever no cast is mid-flow.
	cast *pendingCast `clone:"deep"`
	// castIssued / castFree are the pendingCast recycling pair
	// (cast_pool.go): the last cast storage newCast handed out, and a
	// zeroed one ready for the next cast. Never copied by Clone (a clone's
	// cast is its own copy); castFree rides a Spare.
	castIssued *pendingCast `clone:"reset"`
	castFree   *pendingCast `clone:"reset,pool=cast,release=releaseCastFree(e)"`
	// costCompositionEvent is the one PutOnStack event excluded from
	// cast-count statics while the current cast's cost modifiers are composed
	// after PutOnStack. Stored as event index + 1 (zero means none), so an
	// earlier cast of the same object remains visible.
	costCompositionEvent int `clone:"reset"`
	// turnUp holds the CR 708.6 morph-family turn-face-up special action's
	// payment flow while choosing == chooseTurnUp (engine/rules/morph_turnup.go).
	// Nil whenever no turn-up is mid-payment. A plain-value struct with no
	// closures, so Clone copies it like cast/choosing and a replay
	// re-derives it from the recorded intents.
	turnUp *turnUpPay `clone:"deep"`
	// etbMove parks a battlefield entry while its as-enters choice is answered
	// through the mid-resolution decision path. etbNext is the ordinal of the
	// next choice on that entry; both are plain data so a clone at the decision
	// boundary preserves the entry exactly.
	etbMove *events.Event `clone:"deep"`
	etbNext int           `clone:"deep"`
	// auraEntry is the CR 303.4f non-cast Aura entry's transient record
	// (engine/rules/aura_entry.go): the bearer the entry's fold attaches (default,
	// answered or effect-named). Plain data, set and consumed around one
	// entry, carried across its "enchant" decision boundary. auraEntryCands
	// is the candidate walk's scratch buffer.
	auraEntry      auraEntryState  `clone:"deep"`
	auraEntryCands []auraEntryCand `clone:"reset"`
	// turnUpMove parks the events.TurnFaceUp marker of a morph-family
	// turn-up while an "as this is turned face up" replacement body's own
	// answer is outstanding (task cli-20260924T031747Z-6d0658fc): the
	// transition must not fold until the body finishes, or the body and the
	// turn-up's triggers would observe a face the transition already
	// revealed. Unlike etbMove the park is consumed synchronously inside
	// resolveReplacementBody -- before any decision can be answered -- so it
	// need not survive a clone; the re-emit frame it installs on the body's
	// suspension chain is what crosses the decision boundary.
	turnUpMove *events.Event `clone:"reset"`
	// causePin is the action cause an entry-settle preview pins while its
	// cloned stack no longer holds the entrant (engine/rules/entry_counters.go).
	// Zero on every live engine.
	causePin state.ObjID `clone:"reset"`
	// costCounters is non-zero while a cost's own counter placement is being
	// emitted (an activation's AddCounter<N/KIND> part -- a planeswalker's
	// [+N] -- and a mana ability's AddCounter part). Such a placement is a
	// COST, never an effect, whatever is on the stack: since CR 602.2a's
	// push-first the ability being activated is itself on the stack while it
	// is paid for, and an instant-speed activation always paid above other
	// objects. The AddCounter replacement's EffectOnly$ gate reads it
	// (replacement_match.go). Set and cleared synchronously around the emit.
	costCounters int `clone:"reset"`
	// counterEffectOnlyAdmitted opens the EffectOnly$ provenance gate for one
	// AddCounter re-scan after another replacement effect has already applied
	// to the same counter event (CR 614.16). Set and cleared synchronously;
	// zero on every live engine.
	counterEffectOnlyAdmitted bool `clone:"reset"`
	// etbLandPlay identifies the land whose LandPlayed event must wait for its
	// final battlefield entry. A replacement can suspend and later re-emit the
	// move, so the object id is needed to avoid consuming this continuation on
	// a different move the replacement body emits first.
	etbLandPlay   bool           `clone:"deep"`
	etbLandObj    state.ObjID    `clone:"deep"`
	etbLandPlayer state.PlayerID `clone:"deep"`
	// etbLandPerm is the limited untyped may-play permission the land play
	// consumes (mayPlayLandPermKey), stamped on its LandPlayed event.
	etbLandPerm string `clone:"deep"`
	// riotMove parks a non-cast battlefield entry while its controller makes
	// Riot's as-enters choice. The event is emitted only after Choose records
	// the answer, so every entry path reaches events.Move with RiotChoice set.
	riotMove *events.Event `clone:"deep"`
	// siegeMove parks a non-cast Battle entry while its controller makes the
	// unleashMove parks a non-cast battlefield entry while its controller
	// makes Unleash's as-enters choice (CR 702.86, engine/rules/unleash.go). Same
	// discipline as riotMove: the MoveZone is emitted only after the Choose
	// "unleash" event records the answer, so every entry path reaches
	// events.Move with UnleashChoice set. Clone-copied (clone.go).
	unleashMove *events.Event `clone:"deep"`
	// CR 310.10 Siege protector choice. Same discipline as riotMove: the
	// MoveZone is emitted only after the Choose "protector" event records the
	// answer, so every entry path records the protector beside the entry and a
	// log-only replay re-derives it. Clone-copied (clone.go).
	siegeMove *events.Event `clone:"deep"`
	// entryStageDone holds an entry stage (engine/rules/entry_counters.go) whose
	// characteristic-counter order competition has fully resolved and whose
	// move is being re-emitted for its fold: the fold consumes the finalized
	// amounts from it. Set immediately before the completion re-emit,
	// consumed by the very fold that emit reaches -- the same set-then-
	// consume window the parked-move fields use. Clone-copied (clone.go).
	entryStageDone *entryCounterStage `clone:"deep"`
	// attachedChoice parks an Attach event while an Attached replacement asks
	// for its name and creature type.
	attachedChoice   *attachedChoice `clone:"deep"`
	attachedApplying bool            `clone:"deep"`
	// tokenChoice parks a CreateToken replacement plan while the chosen-copy
	// body (Type$ ReplaceToken | TokenScript$ Chosen -- Esix, Moonlit
	// Meditation, Mirrormind Crown) asks its controller which creature to
	// copy. Same discipline as siegeMove/attachedChoice: the plan's mints are
	// emitted only after the answered election is applied, so the log's
	// CopyToken events carry the choice and a log-only replay re-derives the
	// mints. Clone-copied (clone.go).
	tokenChoice *tokenChoiceState `clone:"deep"`
	// specEnvs/specEnvDepth: targetSpecContext's reusable Resolve records,
	// used as a stack (trigger_referents.go, acquireSpecEnv). Scratch that is
	// free at every intent boundary, so Clone starts a fresh one.
	specEnvs     []*specResolveEnv `clone:"reset"`
	specEnvDepth int               `clone:"reset"`
}
