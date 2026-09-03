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
}
