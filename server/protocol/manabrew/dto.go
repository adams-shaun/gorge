package manabrew

import "encoding/json"

// Wire enums and shared DTOs from the ManaBrew protocol
// (https://docs.manabrew.app/protocol/game-view/ and /protocol/shared-types/).
//
// Types the published spec references but never defines (ZoneKind, DayTime,
// PlayerStatus, PlayerCounterKind, Mana inside ActivatableAbilityInfo) are
// modelled as opaque string/number types; constants marked INFERRED are
// conveniences derived from the spec's examples, not from a published enum.

// StepKind is the defined step enum (game-view page).
type StepKind string

const (
	StepUntap                   StepKind = "untap"
	StepUpkeep                  StepKind = "upkeep"
	StepDraw                    StepKind = "draw"
	StepMain1                   StepKind = "main1"
	StepCombatBegin             StepKind = "combatBegin"
	StepCombatDeclareAttackers  StepKind = "combatDeclareAttackers"
	StepCombatDeclareBlockers   StepKind = "combatDeclareBlockers"
	StepCombatFirstStrikeDamage StepKind = "combatFirstStrikeDamage"
	StepCombatDamage            StepKind = "combatDamage"
	StepCombatEnd               StepKind = "combatEnd"
	StepMain2                   StepKind = "main2"
	StepEndOfTurn               StepKind = "endOfTurn"
	StepCleanup                 StepKind = "cleanup"
)

// ZoneKind is UNDEFINED in the published spec; the values below are INFERRED
// from the game-view examples.
type ZoneKind string

const (
	ZoneLibrary     ZoneKind = "library"
	ZoneHand        ZoneKind = "hand"
	ZoneBattlefield ZoneKind = "battlefield"
	ZoneGraveyard   ZoneKind = "graveyard"
	ZoneExile       ZoneKind = "exile"
	ZoneCommand     ZoneKind = "command"
	// ZoneStack is gorge-internal: ManaBrew's ZoneKind has no stack member,
	// so it never reaches the wire.
	ZoneStack ZoneKind = "stack"
)

// DayTime is the CR 731 designation. ManaBrew's enum (manabrew-protocol
// src/game/mod.rs DayTime) has exactly these three values; an empty string
// is not one of them and fails every ManaBrew client's decode.
type DayTime string

const (
	DayTimeNeither DayTime = "neither"
	DayTimeDay     DayTime = "day"
	DayTimeNight   DayTime = "night"
)

// PlayerStatus is ManaBrew's PlayerStatus enum (manabrew-protocol
// src/game/mod.rs): playing, lost or conceded.
type PlayerStatus string

const (
	PlayerPlaying  PlayerStatus = "playing"
	PlayerLost     PlayerStatus = "lost"
	PlayerConceded PlayerStatus = "conceded"
)

// PlayerCounterKind is ManaBrew's PlayerCounterKind enum (manabrew-protocol
// src/game/mod.rs): the counters a PLAYER holds. A card's counters are keyed
// by free-form strings instead (CardDto.Counters).
type PlayerCounterKind string

const (
	CounterPoison     PlayerCounterKind = "poison"
	CounterEnergy     PlayerCounterKind = "energy"
	CounterExperience PlayerCounterKind = "experience"
	CounterRadiation  PlayerCounterKind = "radiation"
	CounterTicket     PlayerCounterKind = "ticket"
)

// ManaColor is the colour palette used by manaPool maps and Mana values.
type ManaColor string

const (
	ColorWhite     ManaColor = "W"
	ColorBlue      ManaColor = "U"
	ColorBlack     ManaColor = "B"
	ColorRed       ManaColor = "R"
	ColorGreen     ManaColor = "G"
	ColorColorless ManaColor = "C"
)

// TargetRefKind is the defined TargetRef kind enum (shared-types page).
type TargetRefKind string

const (
	RefPlayer TargetRefKind = "player"
	RefCard   TargetRefKind = "card"
	RefSpell  TargetRefKind = "spell"
)

// ScryDestination is the defined scry destination enum.
type ScryDestination string

const (
	DestinationLibraryTop    ScryDestination = "libraryTop"
	DestinationLibraryBottom ScryDestination = "libraryBottom"
	DestinationGraveyard     ScryDestination = "graveyard"
	DestinationExile         ScryDestination = "exile"
	DestinationHand          ScryDestination = "hand"
)

// TargetingIntent is the defined intent enum (shared-types page).
type TargetingIntent string

const (
	IntentDamage      TargetingIntent = "damage"
	IntentDestroy     TargetingIntent = "destroy"
	IntentSacrifice   TargetingIntent = "sacrifice"
	IntentExile       TargetingIntent = "exile"
	IntentBounce      TargetingIntent = "bounce"
	IntentMill        TargetingIntent = "mill"
	IntentDiscard     TargetingIntent = "discard"
	IntentCounter     TargetingIntent = "counter"
	IntentTap         TargetingIntent = "tap"
	IntentUntap       TargetingIntent = "untap"
	IntentCopy        TargetingIntent = "copy"
	IntentBuff        TargetingIntent = "buff"
	IntentDebuff      TargetingIntent = "debuff"
	IntentHeal        TargetingIntent = "heal"
	IntentLoseLife    TargetingIntent = "loseLife"
	IntentReveal      TargetingIntent = "reveal"
	IntentDraw        TargetingIntent = "draw"
	IntentFetch       TargetingIntent = "fetch"
	IntentGainControl TargetingIntent = "gainControl"
	IntentFight       TargetingIntent = "fight"
	IntentAttach      TargetingIntent = "attach"
	IntentAttack      TargetingIntent = "attack"
	IntentBlock       TargetingIntent = "block"
	IntentHostile     TargetingIntent = "hostile"
	IntentFriendly    TargetingIntent = "friendly"
)

// Mana is UNDEFINED in the published spec; {color, amount} is INFERRED from
// the examples.
type Mana struct {
	Color  ManaColor `json:"color"`
	Amount int       `json:"amount"`
}

// TargetRef is a reference to a targetable player, card or spell.
type TargetRef struct {
	Kind   TargetRefKind   `json:"kind"`
	ID     string          `json:"id"`
	Intent TargetingIntent `json:"intent,omitempty"`
	Oracle string          `json:"oracle,omitempty"`
}

// CardIdentity names a card independent of any game instance.
type CardIdentity struct {
	Name       string `json:"name"`
	SetCode    string `json:"setCode"`
	CardNumber string `json:"cardNumber"`
	IsToken    bool   `json:"isToken"`
	// TokenScript is a Forge tokenscripts stem ("taken directly from the Forge
	// spec" per the protocol index).
	TokenScript string `json:"tokenScript,omitempty"`
}

// ClassLevelDto is one Class level (manabrew-protocol src/game/mod.rs
// ClassLevelDto).
type ClassLevelDto struct {
	Level  int    `json:"level"`
	Oracle string `json:"oracle"`
	Cost   string `json:"cost,omitempty"`
}

// SagaChapterDto is one Saga chapter ability and the chapter numbers it
// fires on (manabrew-protocol src/game/mod.rs SagaChapterDto).
type SagaChapterDto struct {
	Chapters []int  `json:"chapters"`
	Oracle   string `json:"oracle"`
}

// CardChoiceDto is one choice made for a card, a union on kind
// (manabrew-protocol src/game/mod.rs CardChoiceDto: color{colors},
// type{values}, namedCard{names}, chosenCard{count}, number{value},
// mode{value}, player{playerId,name}). It is one flat struct: Value is a
// number for kind number and a string for kind mode, so it stays raw.
type CardChoiceDto struct {
	Kind     string          `json:"kind"`
	Colors   []ManaColor     `json:"colors,omitempty"`
	Values   []string        `json:"values,omitempty"`
	Names    []string        `json:"names,omitempty"`
	Count    *int            `json:"count,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
	PlayerID string          `json:"playerId,omitempty"`
	Name     string          `json:"name,omitempty"`
}

// CardDto is the full card shape carried by visible CardViews and prompts
// (manabrew-protocol src/game/mod.rs CardDto). Counter keys follow the
// engine's canonical form: "P1P1", "Loyalty", uppercase one-offs.
//
// Color is ManaBrew's String: the card's colours as WUBRG letters in that
// order ("WU", "" for a colourless card). CMC is its mana value, an i32
// upstream. Types holds the card types only; subtypes and supertypes have
// their own lists, as upstream splits them.
//
// Every list and map field is a JSON array/object, never null: ManaBrew's
// Vec and BTreeMap fields reject null, so MarshalJSON writes a nil one as
// empty (a Go nil slice would otherwise encode as null).
type CardDto struct {
	ID         string       `json:"id"`
	Identity   CardIdentity `json:"identity"`
	Color      ColorText    `json:"color"`
	ManaCost   string       `json:"manaCost"`
	CMC        int          `json:"cmc"`
	Types      []string     `json:"types"`
	Subtypes   []string     `json:"subtypes"`
	Supertypes []string     `json:"supertypes"`
	Power      *string      `json:"power"`
	Toughness  *string      `json:"toughness"`
	// BasePower/BaseToughness are Option<i32> upstream.
	BasePower         *int             `json:"basePower,omitempty"`
	BaseToughness     *int             `json:"baseToughness,omitempty"`
	FinalChapter      *int             `json:"finalChapter,omitempty"`
	ClassLevel        *int             `json:"classLevel,omitempty"`
	ClassLevels       []ClassLevelDto  `json:"classLevels"`
	SagaChapters      []SagaChapterDto `json:"sagaChapters"`
	Text              string           `json:"text"`
	Choices           []CardChoiceDto  `json:"choices"`
	ControllerID      string           `json:"controllerId"`
	OwnerID           string           `json:"ownerId"`
	Tapped            bool             `json:"tapped"`
	IsCrewed          bool             `json:"isCrewed"`
	IsAttacking       bool             `json:"isAttacking"`
	AttackingPlayerID string           `json:"attackingPlayerId,omitempty"`
	AttackTargetID    string           `json:"attackTargetId,omitempty"`
	Keywords          []string         `json:"keywords"`
	Counters          map[string]int   `json:"counters"`
	Damage            int              `json:"damage"`
	SummoningSick     bool             `json:"summoningSick"`
	IsCopy            bool             `json:"isCopy"`
	IsDoubleFaced     bool             `json:"isDoubleFaced"`
	IsTransformed     bool             `json:"isTransformed"`
	IsFaceDown        bool             `json:"isFaceDown"`
	IsBestowed        bool             `json:"isBestowed"`
	PhasedOut         bool             `json:"phasedOut"`
	Exerted           bool             `json:"exerted"`
	IsRingBearer      bool             `json:"isRingBearer"`
	AttachedTo        string           `json:"attachedTo,omitempty"`
	AttachmentIDs     []string         `json:"attachmentIds"`
	MergedCardIDs     []string         `json:"mergedCardIds"`
	FlashbackCost     string           `json:"flashbackCost,omitempty"`
	KickerCost        string           `json:"kickerCost,omitempty"`
	EffectiveManaCost string           `json:"effectiveManaCost,omitempty"`
	CommanderTax      *int             `json:"commanderTax,omitempty"`
	MadnessCost       string           `json:"madnessCost,omitempty"`
	IsMadnessExiled   bool             `json:"isMadnessExiled"`
	IsPlotted         bool             `json:"isPlotted"`
	IsWarpExiled      bool             `json:"isWarpExiled"`
	Foil              bool             `json:"foil"`
	WouldDieInCombat  bool             `json:"wouldDieInCombat"`
}

// CardViewValue is the closed set of card view payloads; see PromptInputData
// (package prompts) for why the union is a struct carrier.
type CardViewValue interface{ isCardViewValue() }

// CardView is the visibility-tagged union a ZoneDto carries:
//
//	({"visibility":"visible"} & CardDto) | {"visibility":"hidden","id"}
type CardView struct {
	Value CardViewValue `json:"-"`
}

// VisibleCard is a fully readable card.
type VisibleCard struct {
	Visibility string `json:"visibility"`
	CardDto           // flattened into the same object
}

// HiddenCard carries only the instance id.
type HiddenCard struct {
	Visibility string `json:"visibility"`
	ID         string `json:"id"`
}

func (VisibleCard) isCardViewValue() {}

// cardDtoFields is CardDto without its methods, so MarshalJSON can encode the
// fields themselves.
type cardDtoFields CardDto

// MarshalJSON encodes the card with every nil list or map written as an empty
// one (see CardDto).
func (c CardDto) MarshalJSON() ([]byte, error) {
	f := cardDtoFields(c)
	emptyList(&f.Types)
	emptyList(&f.Subtypes)
	emptyList(&f.Supertypes)
	emptyList(&f.ClassLevels)
	emptyList(&f.SagaChapters)
	emptyList(&f.Choices)
	emptyList(&f.Keywords)
	emptyList(&f.AttachmentIDs)
	emptyList(&f.MergedCardIDs)
	if f.Counters == nil {
		f.Counters = map[string]int{}
	}
	return json.Marshal(f)
}

// emptyList replaces a nil slice with an empty one.
func emptyList[T any](s *[]T) {
	if *s == nil {
		*s = []T{}
	}
}

// MarshalJSON writes the visibility tag ahead of the card's own fields: the
// embedded CardDto's MarshalJSON would otherwise be promoted and drop it.
func (v VisibleCard) MarshalJSON() ([]byte, error) {
	card, err := v.CardDto.MarshalJSON()
	if err != nil {
		return nil, err
	}
	tag, err := json.Marshal(v.Visibility)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(card)+len(tag)+16)
	out = append(out, `{"visibility":`...)
	out = append(out, tag...)
	if len(card) > 2 {
		out = append(out, ',')
	}
	return append(out, card[1:]...), nil
}
func (HiddenCard) isCardViewValue() {}

func (c *CardView) UnmarshalJSON(b []byte) error {
	var h struct {
		Visibility string `json:"visibility"`
	}
	if err := json.Unmarshal(b, &h); err != nil {
		return err
	}
	var v CardViewValue
	switch h.Visibility {
	case "visible":
		var x VisibleCard
		if err := json.Unmarshal(b, &x); err != nil {
			return err
		}
		v = x
	case "hidden":
		var x HiddenCard
		if err := json.Unmarshal(b, &x); err != nil {
			return err
		}
		v = x
	default:
		return unknownDiscriminator("visibility", h.Visibility)
	}
	*c = CardView{Value: v}
	return nil
}

func (c CardView) MarshalJSON() ([]byte, error) {
	if c.Value == nil {
		return nil, unknownDiscriminator("visibility", "<nil>")
	}
	return json.Marshal(c.Value)
}

// ZoneDto is one (zone, owner) bucket. Battlefield is bucketed by controller,
// so ZoneDto.OwnerID names the owner for hand/library/graveyard/exile/command
// and the controller for battlefield buckets (game-view page).
type ZoneDto struct {
	Zone    ZoneKind   `json:"zone"`
	OwnerID string     `json:"ownerId"`
	Cards   []CardView `json:"cards"`
	Count   int        `json:"count"`
}

// PlayerDto is one player's public state (game-view page). encoding/json sorts
// the keys of the counters/manaPool/commanderDamage maps, which keeps encodes
// deterministic.
type PlayerDto struct {
	ID                  string                    `json:"id"`
	Name                string                    `json:"name"`
	Status              PlayerStatus              `json:"status"`
	IsHuman             bool                      `json:"isHuman"`
	Life                int                       `json:"life"`
	MaxHandSize         int                       `json:"maxHandSize"`
	UnlimitedHandSize   bool                      `json:"unlimitedHandSize"`
	LandsPlayedThisTurn int                       `json:"landsPlayedThisTurn"`
	MaxLandPlaysPerTurn int                       `json:"maxLandPlaysPerTurn"`
	UnlimitedLandPlays  bool                      `json:"unlimitedLandPlays"`
	CardsDrawnThisTurn  int                       `json:"cardsDrawnThisTurn"`
	DamagePrevention    int                       `json:"damagePrevention"`
	IsExtraTurn         bool                      `json:"isExtraTurn"`
	ExtraTurnCount      int                       `json:"extraTurnCount"`
	ControlledBy        string                    `json:"controlledBy,omitempty"`
	PlayerKeywords      []string                  `json:"playerKeywords"`
	CommanderCasts      map[string]int            `json:"commanderCasts"`
	DungeonState        json.RawMessage           `json:"dungeonState,omitempty"` // DungeonStateDto is UNDEFINED
	ActiveSchemeNames   []string                  `json:"activeSchemeNames,omitempty"`
	TeamNumber          *int                      `json:"teamNumber,omitempty"`
	Counters            map[PlayerCounterKind]int `json:"counters"`
	ManaPool            map[ManaColor]int         `json:"manaPool"`
	CommanderDamage     map[string]int            `json:"commanderDamage"`
	HasCityBlessing     bool                      `json:"hasCityBlessing"`
	HasEnduringStory    bool                      `json:"hasEnduringStory"`
	RingLevel           int                       `json:"ringLevel"`
	Speed               int                       `json:"speed"`
}

// CombatAssignmentDto is one blocker→attacker pairing.
type CombatAssignmentDto struct {
	BlockerID  string `json:"blockerId"`
	AttackerID string `json:"attackerId"`
}

// StackObjectDto is one object on the stack (game-view page).
type StackObjectDto struct {
	ID                string       `json:"id"`
	SourceID          string       `json:"sourceId"`
	ControllerID      string       `json:"controllerId"`
	OwnerID           string       `json:"ownerId"`
	Identity          CardIdentity `json:"identity"`
	Text              string       `json:"text"`
	SourceAbilityText string       `json:"sourceAbilityText,omitempty"`
	IsPermanentSpell  bool         `json:"isPermanentSpell"`
	IsCasting         bool         `json:"isCasting"`
	IsDoubleFaced     bool         `json:"isDoubleFaced"`
	FaceIndex         int          `json:"faceIndex"`
	Targets           []TargetRef  `json:"targets"`
}

// OwnDeckRow is one canonical name/count row of the x_gorge_own_deck_v1
// extension's manifest. It is NOT a card instance: it carries no object id,
// no zone membership, no printing, and no position in the library.
type OwnDeckRow struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// OwnDeckExtension is the payload of gorge's additive `x_gorge_own_deck_v1`
// gameView extension.
// It is the authenticated seat's OWN genesis deck manifest -- the same
// immutable, counted list the native view's `own_deck` member carries --
// rendered as name/count rows. It never describes another seat's deck and
// never reflects the current library: no object ids, no library order, no
// card instances. The member is optional JSON: a client that does not know
// the extension ignores it, and it is omitted from any state that has no
// manifest to publish. A zero-card main deck is `"main": []`, never null;
// an empty sideboard is omitted.
type OwnDeckExtension struct {
	Name       string       `json:"name"`
	Main       []OwnDeckRow `json:"main"`
	Sideboard  []OwnDeckRow `json:"sideboard,omitempty"`
	Commanders []string     `json:"commanders,omitempty"`
}

// GameViewDto is the authoritative full state carried by a `state` message
// (game-view page).
type GameViewDto struct {
	GameID             string                `json:"gameId"`
	Turn               int                   `json:"turn"`
	Step               StepKind              `json:"step"`
	CombatAssignments  []CombatAssignmentDto `json:"combatAssignments"`
	ActivePlayerID     string                `json:"activePlayerId"`
	PriorityPlayerID   string                `json:"priorityPlayerId"`
	Players            []PlayerDto           `json:"players"`
	Zones              []ZoneDto             `json:"zones"`
	Stack              []StackObjectDto      `json:"stack"`
	GameOver           bool                  `json:"gameOver"`
	WinnerID           *string               `json:"winnerId"`
	MonarchID          *string               `json:"monarchId"`
	InitiativeHolderID *string               `json:"initiativeHolderId"`
	DayTime            DayTime               `json:"dayTime"`
	ActivePlaneNames   []string              `json:"activePlaneNames,omitempty"`

	// OwnDeck is gorge's `x_gorge_own_deck_v1` extension (OwnDeckExtension):
	// a gorge-specific, additive member that is not part of the published
	// ManaBrew protocol. Old clients ignore the unknown JSON member.
	OwnDeck *OwnDeckExtension `json:"x_gorge_own_deck_v1,omitempty"`
}

// The AvailableAction / PaymentAction type discriminators
// (manabrew-protocol src/prompts/common.rs AvailableActionKind,
// PaymentActionKind).
const (
	ActionCast                = "cast"
	ActionActivateAbility     = "activateAbility"
	ActionUndoMana            = "undoMana"
	ActionActivateManaAbility = "activateManaAbility"
	ActionUseResource         = "useResource"
	ActionReleaseResource     = "releaseResource"
	ActionPayLife             = "payLife"
)

// PlayCardMode is how a cast action plays its card, a union on type
// (manabrew-protocol src/prompts/common.rs PlayCardMode): normal,
// backFaceLand, roomRightSplit, alternative{cost}, staticAlternative,
// foretellExile, unlockDoor. Cost is set only for alternative and names an
// AlternativeCostKind ("flashback", "harmonize", ...).
type PlayCardMode struct {
	Type string `json:"type"`
	Cost string `json:"cost,omitempty"`
}

// UnmarshalJSON also accepts a bare string, the published docs' example shape
// ("mode":"cast"), as the mode's type.
func (m *PlayCardMode) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*m = PlayCardMode{Type: s}
		return nil
	}
	type fields PlayCardMode
	var f fields
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	*m = PlayCardMode(f)
	return nil
}

// The PlayCardMode type values.
const (
	ModeNormal            = "normal"
	ModeBackFaceLand      = "backFaceLand"
	ModeRoomRightSplit    = "roomRightSplit"
	ModeAlternative       = "alternative"
	ModeStaticAlternative = "staticAlternative"
	ModeForetellExile     = "foretellExile"
	ModeUnlockDoor        = "unlockDoor"
)

// AvailableAction is one legal action in a chooseAction prompt: a union on
// `type` (manabrew-protocol src/prompts/common.rs AvailableAction, flattened
// AvailableActionKind): cast{cardId, mode, label}, activateAbility
// (ActivatableAbilityInfo: cardId, abilityIndex, description, isManaAbility,
// isClassLevelUp?, cost?, producedMana?) and undoMana{cardId}. gorge reads
// only id back, so it decodes into one flat struct; MarshalJSON writes
// exactly the fields of the member Type names, every required one present
// even when it is zero (abilityIndex 0, isManaAbility false).
type AvailableAction struct {
	ID             string       `json:"id"`
	Type           string       `json:"type"`
	CardID         string       `json:"cardId,omitempty"`
	Mode           PlayCardMode `json:"mode"`
	Label          string       `json:"label,omitempty"`
	AbilityIndex   int          `json:"abilityIndex,omitempty"`
	Description    string       `json:"description,omitempty"`
	IsManaAbility  bool         `json:"isManaAbility,omitempty"`
	IsClassLevelUp bool         `json:"isClassLevelUp,omitempty"`
	Cost           string       `json:"cost,omitempty"`
	ProducedMana   []Mana       `json:"producedMana,omitempty"`
}

// castActionWire, abilityActionWire and cardActionWire are the wire members
// of AvailableAction and PaymentAction.
type castActionWire struct {
	ID     string       `json:"id"`
	Type   string       `json:"type"`
	CardID string       `json:"cardId"`
	Mode   PlayCardMode `json:"mode"`
	Label  string       `json:"label"`
}

type abilityActionWire struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	CardID         string `json:"cardId"`
	AbilityIndex   int    `json:"abilityIndex"`
	Description    string `json:"description"`
	IsManaAbility  bool   `json:"isManaAbility"`
	IsClassLevelUp *bool  `json:"isClassLevelUp,omitempty"`
	Cost           string `json:"cost,omitempty"`
	ProducedMana   []Mana `json:"producedMana,omitempty"`
}

type cardActionWire struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	CardID string `json:"cardId"`
}

func abilityWire(id, typ, card string, index int, desc string, mana, levelUp bool, cost string, produced []Mana) abilityActionWire {
	w := abilityActionWire{ID: id, Type: typ, CardID: card, AbilityIndex: index, Description: desc,
		IsManaAbility: mana, Cost: cost, ProducedMana: produced}
	if levelUp {
		w.IsClassLevelUp = &levelUp
	}
	return w
}

// MarshalJSON writes the member Type names (see AvailableAction). A cast with
// no mode is a normal cast.
func (a AvailableAction) MarshalJSON() ([]byte, error) {
	switch a.Type {
	case ActionCast:
		mode := a.Mode
		if mode.Type == "" {
			mode.Type = ModeNormal
		}
		return json.Marshal(castActionWire{ID: a.ID, Type: a.Type, CardID: a.CardID, Mode: mode, Label: a.Label})
	case ActionActivateAbility:
		return json.Marshal(abilityWire(a.ID, a.Type, a.CardID, a.AbilityIndex, a.Description, a.IsManaAbility,
			a.IsClassLevelUp, a.Cost, a.ProducedMana))
	case ActionUndoMana:
		return json.Marshal(cardActionWire{ID: a.ID, Type: a.Type, CardID: a.CardID})
	default:
		// A type ManaBrew does not define (only a decoded foreign message
		// carries one): its fields as they are.
		type fields AvailableAction
		return json.Marshal(fields(a))
	}
}

// Deck and its children are the deck data format
// (https://docs.manabrew.app/protocol/deck/). No message carries a Deck yet;
// the shapes are modelled so the adapter can accept one later.
type Deck struct {
	Name            string              `json:"name"`
	Version         string              `json:"version,omitempty"`
	ID              string              `json:"id,omitempty"`
	Description     string              `json:"description,omitempty"`
	Color           ColorText           `json:"color,omitempty"`
	Format          string              `json:"format,omitempty"`
	Cards           []DeckCard          `json:"cards,omitempty"`
	Sideboard       []DeckCard          `json:"sideboard,omitempty"`
	Attractions     []DeckCard          `json:"attractions,omitempty"`
	Contraptions    []DeckCard          `json:"contraptions,omitempty"`
	Schemes         []DeckCard          `json:"schemes,omitempty"`
	Planes          []DeckCard          `json:"planes,omitempty"`
	Commanders      []DeckCard          `json:"commanders,omitempty"`
	Companion       *DeckCard           `json:"companion,omitempty"`
	Maybeboard      []DeckCard          `json:"maybeboard,omitempty"`
	Draft           json.RawMessage     `json:"draft,omitempty"`
	Labels          []DeckLabel         `json:"labels,omitempty"`
	CustomTags      []string            `json:"customTags,omitempty"`
	CardTags        map[string][]string `json:"cardTags,omitempty"`
	Editor          json.RawMessage     `json:"editor,omitempty"` // DeckEditorMetadata, opaque here
	CoverCardName   string              `json:"coverCardName,omitempty"`
	CoverCardFace   *int                `json:"coverCardFace,omitempty"`
	PlaymatURL      string              `json:"playmatUrl,omitempty"`
	PlaymatAssetID  string              `json:"playmatAssetId,omitempty"`
	PlaymatSettings json.RawMessage     `json:"playmatSettings,omitempty"`
	StackPositions  json.RawMessage     `json:"stackPositions,omitempty"`
	Tokens          []DeckCard          `json:"tokens,omitempty"`
}

// ColorText is a colour string ("WU"), ManaBrew's shape for a card's and a
// deck's colours (manabrew-protocol src/game/mod.rs CardDto.color,
// src/deck_dto.rs CardRulesSummary.color and Deck.color). It encodes as a
// string; decoding also accepts a list of colour letters (the published docs'
// examples, and the shape gorge once modelled) and joins it.
type ColorText string

func (c *ColorText) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*c = ColorText(s)
		return nil
	}
	var list []string
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	joined := ""
	for _, l := range list {
		joined += l
	}
	*c = ColorText(joined)
	return nil
}

// DeckCardIdentity identifies a deck entry; ID is a client instance id.
type DeckCardIdentity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	SetCode     string `json:"setCode"`
	CardNumber  string `json:"cardNumber"`
	OracleID    string `json:"oracleId,omitempty"`
	TokenScript string `json:"tokenScript,omitempty"`
	Foil        bool   `json:"foil,omitempty"`
}

// CardImageUris is INFERRED from the deck page's uris field; the field set is
// the Scryfall image-URI layout.
type CardImageUris struct {
	Small      string `json:"small,omitempty"`
	Normal     string `json:"normal,omitempty"`
	Large      string `json:"large,omitempty"`
	ArtCrop    string `json:"art_crop,omitempty"`
	BorderCrop string `json:"border_crop,omitempty"`
	PNG        string `json:"png,omitempty"`
}

// CardBackFaceSummary summarises the back face of a double-faced deck card
// (manabrew-protocol src/deck_dto.rs CardBackFaceSummary).
type CardBackFaceSummary struct {
	Name       string        `json:"name,omitempty"`
	ManaCost   string        `json:"manaCost,omitempty"`
	TypeLine   string        `json:"typeLine,omitempty"`
	OracleText string        `json:"oracleText,omitempty"`
	URIs       CardImageUris `json:"uris"`
}

// DeckCard is one card entry of a Deck.
type DeckCard struct {
	Identity      DeckCardIdentity     `json:"identity"`
	URIs          CardImageUris        `json:"uris"`
	ImageLanguage string               `json:"imageLanguage,omitempty"`
	AllParts      []json.RawMessage    `json:"allParts,omitempty"`
	Color         ColorText            `json:"color"`
	ColorIdentity []string             `json:"colorIdentity"`
	ManaCost      string               `json:"manaCost"`
	CMC           float64              `json:"cmc"`
	Types         []string             `json:"types"`
	Subtypes      []string             `json:"subtypes"`
	Supertypes    []string             `json:"supertypes"`
	Keywords      []string             `json:"keywords"`
	Power         string               `json:"power"`
	Toughness     string               `json:"toughness"`
	Text          string               `json:"text"`
	Layout        string               `json:"layout"`
	IsDoubleFaced bool                 `json:"isDoubleFaced"`
	BackFace      *CardBackFaceSummary `json:"backFace,omitempty"`
}

// DeckLabel is one named deck label with an optional colour.
type DeckLabel struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}
