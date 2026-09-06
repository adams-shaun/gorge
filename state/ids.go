package state

type (
	ObjID    uint32
	PlayerID uint8
	Zone     uint8
	Step     uint8
)

// playerRefBit marks an ObjID that does not name a real object at all, but
// instead carries a PlayerID -- for a caller (rules.pushTrigger, for a
// DeclareAttackers-driven Attacks trigger's defending-player entry) that
// needs to smuggle a player reference through a slot that can only ever
// hold object ids (events.Event.IDs, and therefore the TriggerPush event
// that persists a pending trigger's Remembered onto the stack). See
// Game.NextID/AddObject for why no real object id can ever collide with it.
const playerRefBit ObjID = 1 << 31

// PlayerRef encodes p as an ObjID carrying playerRefBit, so it can travel
// through an []ObjID (an events.Event's IDs field) alongside real object
// ids without a schema change. Decode it back with ObjID.PlayerRef.
func PlayerRef(p PlayerID) ObjID { return playerRefBit | ObjID(p) }

// PlayerRef reports whether id was produced by the package-level PlayerRef
// and, if so, the PlayerID it carries. A real object id -- everything
// AddObject ever hands out -- always reports false; see NextID's own
// comment for why the two ranges can never overlap.
func (id ObjID) PlayerRef() (PlayerID, bool) {
	if id&playerRefBit == 0 {
		return 0, false
	}
	return PlayerID(id &^ playerRefBit), true
}

const (
	ZLibrary Zone = iota
	ZHand
	ZBattlefield
	ZGraveyard
	ZExile
	ZStack
	// ZCommand is the command zone (CR 903.6). It is appended AFTER ZStack, not
	// in Magic's conventional order, so every earlier zone constant keeps its
	// numeric value -- no serialized event log and no chain head shifts from
	// renumbering. It is public information: every seat sees it, so Hidden()
	// stays false for it.
	ZCommand
	numZones = int(ZCommand) + 1
)

var zoneNames = [numZones]string{"library", "hand", "battlefield", "graveyard", "exile", "stack", "command"}

func (z Zone) String() string { return zoneNames[z] }

// Valid reports whether z is one of the defined zone constants. Zone is
// unsigned, so only the upper bound needs checking. Callers that index
// game state by a Zone coming from outside this package (an event, a
// network message) must check Valid first: an out-of-range Zone reaches
// Game's zone-index arithmetic and panics with index-out-of-range.
func (z Zone) Valid() bool { return int(z) < numZones }

// Hidden reports whether a zone's contents are private to its owner. View
// projection and event redaction both key off this.
func (z Zone) Hidden() bool { return z == ZLibrary || z == ZHand }

const (
	StepUntap Step = iota
	StepUpkeep
	StepDraw
	StepMain1
	StepBeginCombat
	StepDeclareAttackers
	StepDeclareBlockers
	StepCombatDamage
	StepEndCombat
	StepMain2
	StepEnd
	StepCleanup
	numSteps = int(StepCleanup) + 1
)

var stepNames = [numSteps]string{"untap", "upkeep", "draw", "main1",
	"begin-combat", "declare-attackers", "declare-blockers", "combat-damage",
	"end-combat", "main2", "end", "cleanup"}

// Valid reports whether s is one of the defined step constants. Mirrors
// Zone.Valid: a Step read back from an untrusted source (a tampered log,
// events/apply.go's StepChange case stores e.Step with no check of its own)
// must be validated before it indexes stepNames, or Project (view.go) and
// this package's own String below would panic on it.
func (s Step) Valid() bool { return int(s) < numSteps }

// String returns "unknown" for an out-of-range Step rather than panicking,
// the same total shape as events.Kind.String().
func (s Step) String() string {
	if !s.Valid() {
		return "unknown"
	}
	return stepNames[s]
}

// IsMain reports whether sorcery-speed actions are legal in this step.
func (s Step) IsMain() bool { return s == StepMain1 || s == StepMain2 }

// Mana indices. A fixed array rather than a map keeps the pool copyable and
// iteration order deterministic.
const (
	MW = iota
	MU
	MB
	MR
	MG
	MC
	numMana
)

type Mana [numMana]int32

// ManaIndex maps a WUBRGC symbol to its pool slot. Returns MC for anything
// unrecognised, which is the safe default for colourless-producing lands.
func ManaIndex(sym byte) int {
	switch sym {
	case 'W':
		return MW
	case 'U':
		return MU
	case 'B':
		return MB
	case 'R':
		return MR
	case 'G':
		return MG
	}
	return MC
}

func (m Mana) Total() int32 {
	var n int32
	for _, v := range m {
		n += v
	}
	return n
}
