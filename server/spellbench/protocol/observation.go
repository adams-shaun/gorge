package protocol

type ManaPool struct {
	W uint32 `json:"W"`
	U uint32 `json:"U"`
	B uint32 `json:"B"`
	R uint32 `json:"R"`
	G uint32 `json:"G"`
	C uint32 `json:"C"`
}

type Characteristics struct {
	Supertypes []string `json:"supertypes"`
	Types      []string `json:"types"`
	Subtypes   []string `json:"subtypes"`
	Colors     []string `json:"colors"`
	ManaValue  uint32   `json:"mana_value"`
	Power      *int32   `json:"power"`
	Toughness  *int32   `json:"toughness"`
	Keywords   []string `json:"keywords"`
}

type Permanent struct {
	Tapped           bool                `json:"tapped"`
	SummoningSick    bool                `json:"summoning_sick"`
	Damage           uint32              `json:"damage"`
	Counters         map[string]uint32   `json:"counters"`
	AttachedTo       *TargetRef          `json:"attached_to"`
	Attacking        bool                `json:"attacking"`
	AttackTarget     *TargetRef          `json:"attack_target"`
	Blocking         bool                `json:"blocking"`
	BlockedAttackers []ObjectRef         `json:"blocked_attackers"`
	PhasedOut        bool                `json:"phased_out"`
	Statuses         []string            `json:"statuses"`
	ClassLevel       *uint32             `json:"class_level"`
	Chosen           []map[string]string `json:"chosen"`
}

type ObjectRecord struct {
	ObjectRef
	FullName        *string          `json:"full_name"`
	FaceDown        bool             `json:"face_down"`
	Token           bool             `json:"token"`
	Copy            bool             `json:"copy"`
	Characteristics *Characteristics `json:"characteristics"`
	Permanent       *Permanent       `json:"permanent"`
	ExiledBy        *ObjectRef       `json:"exiled_by"`
	Abilities       []AbilityRecord  `json:"abilities"`
}

// AbilityRecord is one non-mana activated ability of an ObjectRecord, in the
// order activate_ability.ability_index names it: the printed face's abilities
// first (face order), then the granted abilities in the engine's order. An
// anchor (svar, gained_from/gained_index, keyword) is set exactly for an
// ability the printed face does not carry; a printed ability has every anchor
// nil.
type AbilityRecord struct {
	Label       string     `json:"label"`
	SVar        *string    `json:"svar"`
	GrantSource *ObjectRef `json:"grant_source"`
	GainedFrom  *ObjectRef `json:"gained_from"`
	GainedIndex *uint32    `json:"gained_index"`
	Keyword     *string    `json:"keyword"`
}

type PlayerObs struct {
	Seat                string            `json:"seat"`
	Life                int32             `json:"life"`
	Poison              *uint32           `json:"poison"`
	Counters            map[string]uint32 `json:"counters"`
	ManaPool            ManaPool          `json:"mana_pool"`
	LandsPlayedThisTurn uint32            `json:"lands_played_this_turn"`
	MulligansTaken      uint32            `json:"mulligans_taken"`
	Designations        []string          `json:"designations"`
	Progress            *Progress         `json:"progress"`
	HandCount           uint32            `json:"hand_count"`
	LibraryCount        uint32            `json:"library_count"`
	Hand                []ObjectRecord    `json:"hand"`
	Battlefield         []ObjectRecord    `json:"battlefield"`
	Graveyard           []ObjectRecord    `json:"graveyard"`
	Exile               []ObjectRecord    `json:"exile"`
	Command             []ObjectRecord    `json:"command"`
}

// Progress is Section 6.3's player progress. This engine leaves it null
// (player_progress is false).
type Progress struct {
	Dungeon     *string `json:"dungeon"`
	DungeonRoom *string `json:"dungeon_room"`
	RingTempted uint32  `json:"ring_tempted"`
	Speed       *uint32 `json:"speed"`
}

type StackEntry struct {
	ObjectRef
	StackKind       string           `json:"stack_kind"`
	Source          *ObjectRef       `json:"source"`
	FaceDown        bool             `json:"face_down"`
	Copy            bool             `json:"copy"`
	Characteristics *Characteristics `json:"characteristics"`
	Targets         []*TargetRef     `json:"targets"`
	Divided         []uint32         `json:"divided"`
	Modes           []uint32         `json:"modes"`
	XValue          *uint32          `json:"x_value"`
	Text            *string          `json:"text"`
}

type PendingTrigger struct {
	Source         *ObjectRef `json:"source"`
	SourceName     *string    `json:"source_name"`
	ControllerSeat string     `json:"controller_seat"`
	Label          *string    `json:"label"`
	Optional       bool       `json:"optional"`
}

type Known struct {
	OwnerSeat          string  `json:"owner_seat"`
	Zone               string  `json:"zone"`
	CardName           string  `json:"card_name"`
	ObjectID           *string `json:"object_id"`
	PositionFromTop    *uint32 `json:"position_from_top"`
	PositionFromBottom *uint32 `json:"position_from_bottom"`
	How                string  `json:"how"`
}

type Observation struct {
	Viewer          string           `json:"viewer"`
	Turn            uint32           `json:"turn"`
	PhaseStep       string           `json:"phase_step"`
	ActiveSeat      *string          `json:"active_seat"`
	PrioritySeat    *string          `json:"priority_seat"`
	PassedSeats     []string         `json:"passed_seats"`
	DayNight        *string          `json:"day_night"`
	Players         [2]PlayerObs     `json:"players"`
	Stack           []StackEntry     `json:"stack"`
	PendingTriggers []PendingTrigger `json:"pending_triggers"`
	Known           []Known          `json:"known"`
}
