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
	// Remembered is the objects the Effect captured for its restriction
	// (Vines of Vastwood's targeted creature, Incinerate's damaged creature),
	// so a restriction whose ValidCard$/ValidTarget$ says "Card.IsRemembered"
	// can be resolved against the set the effect actually held, not against
	// nothing. Objects only; a player-only remembered target yields an empty
	// slice.
	Remembered []ObjID
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
	// mayPlayLandIds), never a CR 613 layer change -- no layer fields are
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
