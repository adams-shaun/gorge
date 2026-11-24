package ir

import "encoding/json"

// AbilityKind is the class of an Ability.
type AbilityKind uint8

const (
	AbNone AbilityKind = iota
	AbSpell
	AbActivated
	AbTriggered
	AbStatic
	AbReplacement
	AbKeyword
	abilityKindCount
)

var abilityKindNames = [...]string{
	AbNone:        "none",
	AbSpell:       "spell",
	AbActivated:   "activated",
	AbTriggered:   "triggered",
	AbStatic:      "static",
	AbReplacement: "replacement",
	AbKeyword:     "keyword",
}

// String returns the lower-kebab name of k.
func (k AbilityKind) String() string { return enumString(abilityKindNames[:], k, "AbilityKind") }

// ParseAbilityKind is the inverse of String.
func ParseAbilityKind(s string) (AbilityKind, bool) {
	return parseEnum[AbilityKind](abilityKindNames[:], s)
}

// MarshalJSON encodes k by name.
func (k AbilityKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *AbilityKind) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "AbilityKind", ParseAbilityKind)
}

// EventKind is a game event a Trigger watches.
type EventKind uint16

const (
	EvNone EventKind = iota
	EvEndStep
	EvUpkeep
	EvBeginCombat
	EvOr
	EvAttacks
	EvDealsCombatDamage
	EvDealsCombatDamageToPlayer
	EvDealsNoncombatDamageToPlayer
	EvBecomesTarget
	EvUntaps
	EvDies
	EvEnters
	EvPutIntoGraveyard
	EvLeaves
	EvPlayerAttacks
	EvCast
	EvDiscards
	EvDraws
	EvGainsLife
	EvLosesLife
	EvPlaysLand
	EvCountersPut
	EvNextEndStep
	EvReflexive
	EvWouldEnter
	EvWouldPutCounters
	EvWouldDealDamage
	EvWouldCreateTokens
	EvWouldDie
	eventKindCount
)

var eventKindNames = [...]string{
	EvNone:                         "none",
	EvEndStep:                      "end-step",
	EvUpkeep:                       "upkeep",
	EvBeginCombat:                  "begin-combat",
	EvOr:                           "or",
	EvAttacks:                      "attacks",
	EvDealsCombatDamage:            "deals-combat-damage",
	EvDealsCombatDamageToPlayer:    "deals-combat-damage-to-player",
	EvDealsNoncombatDamageToPlayer: "deals-noncombat-damage-to-player",
	EvBecomesTarget:                "becomes-target",
	EvUntaps:                       "untaps",
	EvDies:                         "dies",
	EvEnters:                       "enters",
	EvPutIntoGraveyard:             "put-into-graveyard",
	EvLeaves:                       "leaves",
	EvPlayerAttacks:                "player-attacks",
	EvCast:                         "cast",
	EvDiscards:                     "discards",
	EvDraws:                        "draws",
	EvGainsLife:                    "gains-life",
	EvLosesLife:                    "loses-life",
	EvPlaysLand:                    "plays-land",
	EvCountersPut:                  "counters-put",
	EvNextEndStep:                  "next-end-step",
	EvReflexive:                    "reflexive",
	EvWouldEnter:                   "would-enter",
	EvWouldPutCounters:             "would-put-counters",
	EvWouldDealDamage:              "would-deal-damage",
	EvWouldCreateTokens:            "would-create-tokens",
	EvWouldDie:                     "would-die",
}

// String returns the lower-kebab name of k.
func (k EventKind) String() string { return enumString(eventKindNames[:], k, "EventKind") }

// ParseEventKind is the inverse of String.
func ParseEventKind(s string) (EventKind, bool) { return parseEnum[EventKind](eventKindNames[:], s) }

// MarshalJSON encodes k by name.
func (k EventKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *EventKind) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "EventKind", ParseEventKind)
}

// Ability is one ability of a face.
type Ability struct {
	Kind AbilityKind `json:"kind,omitempty"`
	Cost *Cost       `json:"cost,omitempty"`
	// Trigger is the trigger, or the replaced event for AbReplacement.
	Trigger *Trigger `json:"trigger,omitempty"`
	Cond    *Cond    `json:"cond,omitempty"`
	Body    *Program `json:"body,omitempty"`
	Keyword *Keyword `json:"keyword,omitempty"`
	Zones   ZoneSet  `json:"zones,omitempty"`
	// SorceryOnly: activate only as a sorcery.
	SorceryOnly bool `json:"sorceryOnly,omitempty"`
	// InstantOnly: activate only as an instant (Lion's Eye Diamond).
	InstantOnly bool `json:"instantOnly,omitempty"`
	// Once: activate only once.
	Once bool   `json:"once,omitempty"`
	Src  string `json:"src,omitempty"`
}

// Keyword is a keyword ability. Name is the keyword's rules name (Flying,
// Ward), never reminder text.
type Keyword struct {
	Name   string  `json:"name,omitempty"`
	N      *Value  `json:"n,omitempty"`
	Cost   *Cost   `json:"cost,omitempty"`
	Filter *Filter `json:"filter,omitempty"`
	Src    string  `json:"src,omitempty"`
}

// Trigger is a trigger condition.
type Trigger struct {
	Event       EventKind `json:"event,omitempty"`
	Subject     *Filter   `json:"subject,omitempty"`
	Ref         *Ref      `json:"ref,omitempty"`
	Zones       ZoneSet   `json:"zones,omitempty"`
	OncePerTurn bool      `json:"oncePerTurn,omitempty"`
	// Object is the event's other party: the recipient, the spell, the
	// permanent counters go on.
	Object *Filter `json:"object,omitempty"`
	// Cmp is the nth such event this turn, or an attacker count.
	Cmp *Compare `json:"cmp,omitempty"`
	// Word is a counter word, or combat/noncombat.
	Word string `json:"word,omitempty"`
	// Cond is a condition on the trigger event itself, not an intervening if.
	Cond *Cond `json:"cond,omitempty"`
	// Args are EvOr's alternatives.
	Args []*Trigger `json:"args,omitempty"`
	Src  string     `json:"src,omitempty"`
}
