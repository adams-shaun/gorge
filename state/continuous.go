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
