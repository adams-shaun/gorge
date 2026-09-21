// ContinuousEffect and its Layer/Sublayer vocabulary live in state, below
// effects and rules, purely so effects primitives (Pump, PumpAll, Animate,
// Protection -- see mtgcore/effects/combatfx.go) can build one and hand it to
// rules.Engine through the effects.Host interface without effects ever
// importing rules (the dependency order is cards -> state -> decision ->
// events -> effects -> rules -> view -> seat -> replay -> cmd/*, and effects
// must never import rules). The layer computation itself -- ordering,
// expiry, Derived -- stays in rules/layers.go, which re-exports these same
// names as aliases so its own existing API and tests are unaffected by the
// move. See Task 19c.
package state

import "github.com/adams-shaun/gorge/cards"

// Layer is CR 613's application order.
type Layer uint8

const (
	LCopy      Layer = 1
	LControl   Layer = 2
	LText      Layer = 3
	LType      Layer = 4
	LColor     Layer = 5
	LAbilities Layer = 6
	LPT        Layer = 7
)

// Sublayer is CR 613.4's breakdown of layer 7.
type Sublayer uint8

const (
	SubNone     Sublayer = 0
	SubCDA      Sublayer = 1 // 7a characteristic-defining
	SubSet      Sublayer = 2 // 7b setting
	SubModify   Sublayer = 3 // 7c modifying
	SubCounters Sublayer = 4 // 7d counters
	SubSwitch   Sublayer = 5 // 7e switching
)

// ContinuousEffect is one active modification. Affects is a Forge filter spec
// evaluated with effects.MatchesSpecFrom against each object on the
// battlefield, so continuous effects reuse the same filter language as
// everything else rather than reimplementing predicate matching. (That
// evaluation itself lives in rules.Engine.Derived, which is free to import
// effects; this type only needs to be nameable from both packages.)
type ContinuousEffect struct {
	Source     ObjID
	Timestamp  uint32
	Layer      Layer
	Sub        Sublayer
	Affects    string
	Controller PlayerID
	UntilEOT   bool

	AddPower, AddToughness int32
	SetPower, SetToughness int32
	HasSet                 bool
	AddKeywords            []string
	AddTypes               []string

	// AddPowerExpr preserves a static P/T parameter that must be evaluated
	// against its source each time characteristics are derived (for example
	// +X or -X). An empty expression retains the already-resolved numeric
	// field. Written only by rules' static scanner; the numeric fields above
	// stay the API for resolution-created effects.
	AddPowerExpr, AddToughnessExpr string
	SetPowerExpr, SetToughnessExpr string
	// SetPowerPresent and SetToughnessPresent distinguish an omitted setter
	// from an explicit zero on a static that sets only one characteristic.
	SetPowerPresent, SetToughnessPresent bool
	// StaticSet marks a setter read from a card's S:Mode$ Continuous line.
	// Such a static may set exactly one characteristic, so Derived must gate
	// each assignment on its corresponding *Present bit. Effects created by
	// the older numeric ContinuousEffect API leave this false and retain
	// their historical paired-setter behaviour for compatibility.
	StaticSet bool

	// AddColors is a layer-5 colour change (CR 613.1e): the WUBRG letters of
	// the colours the affected object GAINS. Written by the continuous-effect
	// primitives (effects' Animate Colors$ without OverwriteColors$), composed
	// by rules' Derived in timestamp order. Empty on every effect that grants
	// no colour. "All"/"Colorless" never reach this field: the registering
	// primitive normalises them to "WUBRG" and the empty set respectively.
	AddColors []string
	// OverwriteColors marks a layer-5 colour SET (Forge's Animate
	// OverwriteColors$ True with Colors$): while this effect applies, the
	// affected object's colours are exactly AddColors -- replacing, never
	// extending, the printed colours and every earlier layer-5 grant
	// (CR 613.1e sets by timestamp order). With it false AddColors extends.
	OverwriteColors bool
	// RemoveCreatureTypes is Forge's Animate RemoveCreatureTypes$ True: while
	// this effect applies, the affected object loses every creature-type
	// SUBTYPE (its face's types that are neither card types nor supertypes)
	// BEFORE this same effect's AddTypes apply -- so an animated creature's
	// own new creature type lands on a stripped base, and the printed types
	// return the moment the animation expires because Derived recomputes from
	// the face. A printed planeswalker's name-subtype ("Sarkhan") is stripped
	// with the rest while the walker is animated as a creature.
	RemoveCreatureTypes bool
	// RemoveCardTypes is the S:Mode$ Continuous RemoveCardTypes$ True strip
	// (Darksteel Mutation, Kenrith's Transformation, Witness Protection):
	// while this effect applies, the affected object loses every card type
	// AND every subtype -- subtypes are tied to their card types (CR
	// 205.2-family), so the object keeps only its supertypes -- BEFORE this
	// same effect's AddTypes apply (strip-before-add, like
	// RemoveCreatureTypes). Set only by the static scanner today; the Animate
	// primitive does not read RemoveCardTypes$ yet.
	RemoveCardTypes bool
	// AddAbilities is a layer-6 ability GRANT (CR 613.1f): the SVar names --
	// on the SOURCE object's own face -- of the AB$ activated abilities the
	// affected object gains for the effect's lifetime. Written only by the
	// continuous-effect primitives (effects' Animate Abilities$), read only
	// by rules' offer/activation paths (legal.go's grantedAbilities), never
	// by the CR 613 layer sorter itself: the grant contributes no
	// characteristic, it contributes an activation surface. Empty on every
	// effect that grants none.
	AddAbilities []string

	// Restriction carries an Effect-created S: mode (CantTarget,
	// CantRegenerate) rather than a layer change. When non-empty the effect is
	// a rules-mod, consulted by the decision point the mode names (rules'
	// askTarget for CantTarget, effects.ReplaceDestruction via the Host for
	// CantRegenerate); the layer fields above are ignored for it, and it is
	// never handed to the CR 613 layer sorter as a characteristic change.
	// A zero value means the effect is an ordinary layer effect.
	Restriction string
	// RestrictParams carries the restriction's S: parameters (ValidCard$,
	// ValidTarget$, Activator$...) so the consultation point can resolve the
	// same logic the registered static carries. Map-only, never read by the
	// layer sorter.
	RestrictParams map[string]string
	// Name is an Effect's Name$ (Wrenn and Six's "Emblem — Wrenn and Six",
	// Sephiroth's emblem): the effect's own display name, carried so an Effect
	// whose Stackable$ is False can ask the registry (rules'
	// ContinuousNamed) whether a same-name effect from the same controller is
	// already active and decline to stack a second one. Empty on every effect
	// that carries no Name$.
	Name string
	// Remembered is the objects the Effect captured for its restriction
	// (Vines of Vastwood's targeted creature, Incinerate's damaged creature),
	// so a restriction whose ValidCard$/ValidTarget$ says "Card.IsRemembered"
	// can be resolved against the set the effect actually held, not against
	// nothing. Objects only; a player-only remembered target yields an empty
	// slice.
	Remembered []ObjID
	// RememberedPlayers is the PLAYERS an Effect captured for its restriction
	// (Call for Aid's RememberObjects$ TargetedPlayer: the targeted opponent
	// whose creatures were stolen, so the registered CantAttack's Target$
	// Player.IsRemembered — "you can't attack that player" — has a remembered
	// player to resolve). effectRemembered deliberately records objects only;
	// a player-only remember yields an empty slice there, so this field is the
	// explicit player half of the same capture. Empty on every effect that
	// captured no players. Engine-runtime only, rebuilt by re-execution on
	// replay like every other continuous-effect field.
	RememberedPlayers []PlayerID
	// Duration is the original Duration$ value ("" means Permanent, the
	// effEffect default) preserved for reporting and for the expiry decision
	// in rules/layers.go. Cosmetic for a layer effect.
	Duration string
	// Permanent marks a resolution-created effect with no stated duration
	// (CR 611.2a): it lasts until the end of the game, outliving its source
	// (a one-shot spell is already in the graveyard by the time it registers)
	// and surviving every EndOfTurnCleanup. Distinct from Duration "Permanent"
	// on a permanent's OWN static, which CR 611.3b scopes to the source's
	// battlefield presence -- that case leaves Permanent false and keeps the
	// source-leaves rule. Set only by the continuous-effect primitives that
	// build a lasting one-shot (effects/combatfx.go).
	Permanent bool
	// ReplacementEvent/ReplacementParams/ReplacementBody describe an Effect-created
	// replacement (for example Blood of the Martyr). They are deliberately plain
	// data rather than cards types: state sits below cards' parsed SA graph.
	// Rules reconstructs the body at application time under the original source's
	// SVar context. An empty ReplacementEvent is not a replacement effect.
	ReplacementEvent  string
	ReplacementParams map[string]string
	ReplacementBody   string
	// ChosenNumber is the Effect's SetChosenNumber$ binding: the number the
	// Effect resolved when it was created (Torgal's Dog/Wolf count at trigger
	// time, Wildgrowth Archaic's TriggeredCard$Converge snapshot, Communal
	// Brewing's ingredient-counter count), read later by the registered
	// replacement's body through the Count$ChosenNumber head (rules' replCtx
	// threads it into the body Ctx). Binding ONCE at creation against the
	// trigger's own context is the point: a live re-read after the entry would
	// answer a different question. Engine-runtime only, rebuilt by
	// re-execution on replay like every other continuous-effect field. Zero
	// means nothing bound (and reads as zero).
	ChosenNumber int32
	// RemoveAbilities is a layer-6 ability-removing effect (CR 613.1f/613.4b,
	// e.g. Humility's RemoveAllAbilities$ True): when an applicable effect
	// carries it, Derived clears the object's printed (and any earlier-granted)
	// keywords/abilities before later layer-6 grants re-add any.
	RemoveAbilities bool
	// MayPlay marks a may-play-from-zone grant (CR 401.5: "you may play
	// cards of a certain kind from a zone other than the one they would
	// normally be played from", e.g. Conduit of Worlds' "You may play lands
	// from your graveyard."). Registered from an S:Mode$ Continuous static
	// that carries MayPlay$ True, alongside the Affects filter (the Affected$
	// spec, the effect's ordinary filter field) and AffectedZone. When set
	// the effect is a rules-mod consulted by the land-play offer (rules'
	// mayplay.go), never a CR 613 layer change -- no layer fields are
	// read for it -- and it expires with its source permanent (CR 611.3b)
	// through the ordinary source-on-battlefield check in active().
	MayPlay bool
	// AffectedZone is the may-play grant's AffectedZone$ value (a single
	// zone, a comma-separated list, or "All"), interpreted with
	// effects.ParseZones. Meaningful only when MayPlay is set.
	AffectedZone string

	// MayPlayIgnoreColor marks the grant's MayPlayIgnoreColor$ True rider:
	// "you may spend mana as though it were mana of any color to cast it"
	// (Opposition Agent, Kotose, ...). While it holds, every coloured pip of
	// the may-play cast's cost is payable by any colour of mana in the pool;
	// the {C} pip stays colourless-only (CR 107.4c: "any color" never
	// includes colourless). Set only together with MayPlay.
	MayPlayIgnoreColor bool

	// MayPlayLimit is the grant's MayPlayLimit$ once-per-turn cap (Kotose's
	// and Evelyn's "once each turn"): the number of cards p may play through
	// THIS KIND of grant in one turn. Zero means unlimited. The count is a
	// log scan (rules' mayPlaysThisTurn), never a mutable field.
	MayPlayLimit int32

	// MayPlayPlayerTurn marks the grant's Condition$ PlayerTurn rider (the
	// Kess/Karador "during each of your turns" family): the grant is live
	// only while its controller is the ACTIVE player. The walks check the
	// live turn, never a mutable field.
	MayPlayPlayerTurn bool

	// AddTrigger is a static-grant's triggered ability (AddTrigger$ on a
	// Mode$ Continuous static, e.g. Hearthhull's "STATION 8+ Whenever you
	// sacrifice a land"): the SVar-parsed trigger (cards.ParseTriggerLine
	// off the granting face's own SVar table, its Execute$ linked there) the
	// objects the static's Affected$ matches gain while the static is live.
	// Written only by rules/layers.go's static scan, matched and queued by
	// rules/trigger_match.go's granted-trigger walk (the granted-Ward/
	// granted-Dethrone precedent); it contributes no CR 613 characteristic
	// and the layer sorter ignores it. Nil on every effect that grants none.
	AddTrigger *cards.Trigger
	// AddSVars is a static-grant's named variables (AddSVar$): the SVar the
	// affected object GAINS, parsed from Forge's "SVar:<Name>:<Value>" value
	// shape. The corpus's granted SVars are AI-evaluation hints (AE, AITap,
	// MustBeBlocked) no rules consumer reads yet; the engine records the
	// grant and resolves it through Engine.GrantedSVar, the same lookup a
	// later CheckSVar$-style consumer of the affected object's variables
	// reads. Nil on every effect that grants none.
	AddSVars map[string]string
	// GainControl is a control-change static (Mind Control's "You control
	// enchanted creature", Fealty to the Realm's "The monarch controls
	// enchanted creature"): the value is the GainControl$ parameter of an
	// S:Mode$ Continuous static, and while the static is live every object
	// its Affects spec matches is controlled by the player the value names.
	// Like MayPlay it changes no characteristic and is not a CR 613 layer
	// change -- it is a rules-mod realized by rules' static-control reconcile
	// (rules/control_static.go), which registers a real tracked control grant
	// (rules/control.go) and emits events.ControlChange, so triggers, the
	// controller-reset semantics and the view all see the transfer through
	// the ordinary path. The value resolves through the shared player-spec
	// grammar: "You" is the static's controller, any other qualified player
	// spec ("Player.isMonarch") resolves to the single seat the spec matches,
	// and anything that names nobody (or several) fails closed -- no grant.
	// The static's own "as long as" gate (IsPresent$/CheckSVar$) is the
	// ordinary continuousGateHolds the scan runs for every static; the grant
	// ends when the static stops being live (source left the battlefield,
	// gate flipped, Aura moved bearers) or the resolved controller changes.
	// Empty on every effect that grants no control.
	GainControl string

	// MayLookAt is a look-permission grant (MayLookAt$ on a Mode$
	// Continuous static, e.g. Oracle of Mul Daya): while the static is live,
	// the affected player may look at the object its Affected$ spec matches
	// -- in the corpus always the top card of the controller's own library
	// (Affected$ Card.TopLibrary+YouCtrl, AffectedZone$ Library). Consumed
	// by Engine.MayLookAtLibraryTop (the view's reveal of that top card);
	// it contributes no CR 613 characteristic. False on every effect that
	// grants none.
	MayLookAt bool

	// MayPlayIgnoreType marks the grant's MayPlayIgnoreType$ True rider
	// (Rakdos, the Muscle's "mana of any type can be spent to cast those
	// spells"): like MayPlayIgnoreColor it lets any colour pay a coloured
	// pip, and it additionally lets any colour pay the {C} pips, which
	// CR 107.4c's colour-only "any color" never reaches. Set only together
	// with MayPlay.
	MayPlayIgnoreType bool

	// ForgetOnMoved carries the Effect's ForgetOnMoved$ zone: a remembered
	// card that moves FROM that zone leaves the effect's Remembered set (the
	// may-play grant's Affected$ Card.IsRemembered list; Abbot of Keral
	// Keep's "for as long as it remains exiled"). Empty means the
	// effect never forgets. Engine-runtime only, rebuilt by re-execution on
	// replay like every other continuous-effect field.
	ForgetOnMoved string
	// ExileOnMoved carries the Effect's ExileOnMoved$ zone: a remembered
	// card that moves FROM that zone ENDS the whole effect (Vines of
	// Vastwood's blinked target, Abbot's exiled card being cast). Empty
	// means the effect never ends on a move. Engine-runtime only, like
	// ForgetOnMoved.
	ExileOnMoved string
	// ForgetCounter carries the Effect's ForgetCounter$ counter kind (task
	// vow1; Promise of Loyalty's VOW, Quicksilver Fountain's FLOOD,
	// Obsidian Fireheart's BLAZE -- 18 corpus carriers): a remembered card
	// whose count of that kind reaches zero after a counter-removal leaves
	// the effect's Remembered set -- "for as long as it has a vow counter
	// on it". The count DROPPING without reaching zero keeps the card (the
	// measured semantics this build pins: a multi-countered card loses the
	// restriction only when its LAST such counter goes). Engine-runtime
	// only, rebuilt by re-execution on replay like every other
	// continuous-effect field.
	ForgetCounter string

	// AdjustLandPlays marks an additional-land-drops grant (Azusa, Lost but
	// Seeking's "You may play two additional lands on each of your turns",
	// Oracle of Mul Daya, Exploration): the number of EXTRA land drops the
	// affected player gets each turn, ON TOP of the one ordinary drop
	// (CR 305.2a reads the printed sentence as a modifier on the one-drop
	// normal, so grants from multiple permanents SUM -- Azusa plus
	// Exploration is three drops, never the max). Registered from an
	// S:Mode$ Continuous static carrying a plain integer AdjustLandPlays$,
	// alongside the Affects player spec (Affected$ You/Player); the effect
	// is a rules-mod consulted by rules' land-play offer gates, never a
	// CR 613 layer change -- no layer fields are read for it -- and it
	// expires with its source permanent (CR 611.3b) through the ordinary
	// source-on-battlefield check in active(). Zero means the effect grants
	// no additional drop.
	AdjustLandPlays int32

	// CloneTarget marks a continuous effect created by an api:Clone copy
	// (DB$ Clone): the permanent that BECAME the copy. Every effect a single
	// clone registered carries the same value -- the layer-1 LCopy marker
	// (which owns the lifetime) and its layer-4/5/6/7 modifier effects (which
	// share it), so rules' clone sweep can expire a copy as one unit: it
	// drops the marker on the marker's own duration and removes every
	// effect whose CloneTarget is the same object, emitting the ClonePermanent
	// clear that drops the object's CopyFace basis. Zero on every other
	// effect. Engine-runtime, rebuilt by re-execution on replay like the
	// other resolution-created fields.
	CloneTarget ObjID

	// CloneSource, CloneName and CloneGainThisAbility describe the copy the
	// layer-1 LCopy MARKER of a clone unit owns: the object whose printed
	// face the copy is taken from, the NewName$ rider (empty means the
	// source's own name) and the GainThisAbility$ True rider. They exist so a
	// unit's expiry can RE-BASE the become object onto whatever OTHER clone
	// unit is still live on it (CR 613.1a applies copy effects in timestamp
	// order, so the highest-timestamp survivor wins) instead of clearing the
	// shared CopyFace basis outright. Set only on the marker (Layer LCopy)
	// by effects' api:Clone; zero on its sibling modifier effects and on
	// every non-clone effect. Engine-runtime, rebuilt by re-execution on
	// replay like CloneTarget itself.
	//
	// The re-base re-snapshots the surviving source's face at expiry time
	// rather than replaying the original snapshot, so a survivor whose own
	// source has since changed re-bases onto the source's CURRENT printed
	// face. Measured corpus-unreachable (no carrier stacks two clone units
	// on one permanent and then mutates the older one's source), recorded in
	// AGENTS.md's clone row.
	CloneSource          ObjID
	CloneName            string
	CloneGainThisAbility bool

	// UntilTurn is the turn number at whose END (its cleanup step) this
	// effect expires, for a Duration$ that spans the controller's NEXT turn
	// (UntilYourNextTurn, UntilTheEndOfYourNextTurn). Computed at
	// registration from the live turn/active-player rotation
	// (rules.Engine.AddContinuous), so an effect on a one-shot spell outlives
	// its source and is dropped only by EndOfTurnCleanup when e.G.Turn
	// reaches UntilTurn -- a real turn-boundary lifetime, not the
	// source-leaves branch nor a premature end-of-this-turn expiry. Zero
	// means "no turn boundary": the effect is UntilEOT or expires with its
	// source, per the fields above. Reproduced byte-identically by a replay,
	// because AddContinuous recomputes it from the same deterministic
	// rotation when the intents re-execute.
	UntilTurn int32
}
