package state

type (
	ObjID    uint32
	PlayerID uint8
	Zone     uint8
	Step     uint8
)

const (
	ZLibrary Zone = iota
	ZHand
	ZBattlefield
	ZGraveyard
	ZExile
	ZStack
	numZones = int(ZStack) + 1
)

var zoneNames = [numZones]string{"library", "hand", "battlefield", "graveyard", "exile", "stack"}

func (z Zone) String() string { return zoneNames[z] }

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

func (s Step) String() string { return stepNames[s] }

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
