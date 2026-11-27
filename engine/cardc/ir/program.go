package ir

import "encoding/json"

// Verb is the action of a Step.
type Verb uint16

const (
	VerbNone Verb = iota
	VerbAddMana
	VerbAttach
	VerbChoose
	VerbChooseColor
	VerbChooseModes
	VerbCopySpell
	VerbCounter
	VerbApplyEffect
	VerbDelayedTrigger
	VerbCreateReplacement
	VerbCreateToken
	VerbCreateTokenCopy
	VerbDestroy
	VerbDiscard
	VerbDraw
	VerbForEachPlayer
	VerbEndTurn
	VerbExile
	VerbFight
	VerbGainControl
	VerbGainLife
	VerbEmblem
	VerbIf
	VerbLookAtHand
	VerbLook
	VerbLoseLife
	VerbDo
	VerbPayCost
	VerbMill
	VerbDealDamage
	VerbMove
	VerbPutCounters
	VerbDoubleCounters
	VerbRegenerate
	VerbRemoveCounters
	VerbReveal
	VerbRevealHand
	VerbRevealUntil
	VerbSacrifice
	VerbSearch
	VerbShuffle
	VerbSurveil
	VerbExtraTurn
	VerbTap
	VerbUntap
	VerbTransform
	VerbModifyEnter
	VerbChooseCreatureType
	VerbChooseNumber
	VerbSetAmount
	VerbPrevent
	verbCount
)

var verbNames = [...]string{
	VerbNone:               "none",
	VerbAddMana:            "add-mana",
	VerbAttach:             "attach",
	VerbChoose:             "choose",
	VerbChooseColor:        "choose-color",
	VerbChooseModes:        "choose-modes",
	VerbCopySpell:          "copy-spell",
	VerbCounter:            "counter",
	VerbApplyEffect:        "apply-effect",
	VerbDelayedTrigger:     "delayed-trigger",
	VerbCreateReplacement:  "create-replacement",
	VerbCreateToken:        "create-token",
	VerbCreateTokenCopy:    "create-token-copy",
	VerbDestroy:            "destroy",
	VerbDiscard:            "discard",
	VerbDraw:               "draw",
	VerbForEachPlayer:      "for-each-player",
	VerbEndTurn:            "end-turn",
	VerbExile:              "exile",
	VerbFight:              "fight",
	VerbGainControl:        "gain-control",
	VerbGainLife:           "gain-life",
	VerbEmblem:             "emblem",
	VerbIf:                 "if",
	VerbLookAtHand:         "look-at-hand",
	VerbLook:               "look",
	VerbLoseLife:           "lose-life",
	VerbDo:                 "do",
	VerbPayCost:            "pay-cost",
	VerbMill:               "mill",
	VerbDealDamage:         "deal-damage",
	VerbMove:               "move",
	VerbPutCounters:        "put-counters",
	VerbDoubleCounters:     "double-counters",
	VerbRegenerate:         "regenerate",
	VerbRemoveCounters:     "remove-counters",
	VerbReveal:             "reveal",
	VerbRevealHand:         "reveal-hand",
	VerbRevealUntil:        "reveal-until",
	VerbSacrifice:          "sacrifice",
	VerbSearch:             "search",
	VerbShuffle:            "shuffle",
	VerbSurveil:            "surveil",
	VerbExtraTurn:          "extra-turn",
	VerbTap:                "tap",
	VerbUntap:              "untap",
	VerbTransform:          "transform",
	VerbModifyEnter:        "modify-enter",
	VerbChooseCreatureType: "choose-creature-type",
	VerbChooseNumber:       "choose-number",
	VerbSetAmount:          "set-amount",
	VerbPrevent:            "prevent",
}

// String returns the lower-kebab name of k.
func (k Verb) String() string { return enumString(verbNames[:], k, "Verb") }

// ParseVerb is the inverse of String.
func ParseVerb(s string) (Verb, bool) { return parseEnum[Verb](verbNames[:], s) }

// MarshalJSON encodes k by name.
func (k Verb) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *Verb) UnmarshalJSON(b []byte) error { return unmarshalEnum(k, b, "Verb", ParseVerb) }

// Duration is how long an effect lasts.
type Duration uint8

const (
	DurNone Duration = iota
	DurEndOfTurn
	DurPermanent
	DurUntilYourNextTurn
	DurWhileSourceOnBattlefield
	DurEndOfCombat
	DurWhileActive
	DurUntilEndOfYourNextTurn
	durationCount
)

var durationNames = [...]string{
	DurNone:                     "none",
	DurEndOfTurn:                "end-of-turn",
	DurPermanent:                "permanent",
	DurUntilYourNextTurn:        "until-your-next-turn",
	DurWhileSourceOnBattlefield: "while-source-on-battlefield",
	DurEndOfCombat:              "end-of-combat",
	DurWhileActive:              "while-active",
	DurUntilEndOfYourNextTurn:   "until-end-of-your-next-turn",
}

// String returns the lower-kebab name of k.
func (k Duration) String() string { return enumString(durationNames[:], k, "Duration") }

// ParseDuration is the inverse of String.
func ParseDuration(s string) (Duration, bool) { return parseEnum[Duration](durationNames[:], s) }

// MarshalJSON encodes k by name.
func (k Duration) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *Duration) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "Duration", ParseDuration)
}

// Program is a sequence of steps.
//
// Targets declares the targets of the program's steps. A RefTarget's Slot
// indexes the Targets of the nearest enclosing Program that has any.
type Program struct {
	Steps   []*Step   `json:"steps,omitempty"`
	Targets []*Target `json:"targets,omitempty"`
	Src     string    `json:"src,omitempty"`
}

// Step is one action of a Program.
type Step struct {
	Verb     Verb `json:"verb,omitempty"`
	Optional bool `json:"optional,omitempty"`
	// Who is the acting player for this step and every step nested under it
	// that has no Who of its own; nil means the ability's controller.
	Who    *Ref     `json:"who,omitempty"`
	Obj    *Ref     `json:"obj,omitempty"`
	N      *Value   `json:"n,omitempty"`
	Cond   *Cond    `json:"cond,omitempty"`
	Then   *Program `json:"then,omitempty"`
	Else   *Program `json:"else,omitempty"`
	Dur    Duration `json:"dur,omitempty"`
	From   Zone     `json:"from,omitempty"`
	To     Zone     `json:"to,omitempty"`
	Word   string   `json:"word,omitempty"`
	Colors ColorSet `json:"colors,omitempty"`
	// Source is the acting object (damage source, the first fighter).
	Source *Ref `json:"source,omitempty"`
	// Other is a second object operand (what Obj is attached to).
	Other *Ref `json:"other,omitempty"`
	// Filter is a selection filter (VerbRevealUntil's "until a card matching").
	Filter *Filter `json:"filter,omitempty"`
	// Cost is VerbPayCost's cost.
	Cost *Cost `json:"cost,omitempty"`
	// Mana is VerbAddMana's mana. A ManaColored symbol with more than one
	// colour is one mana of any one of those colours.
	Mana []ManaSym `json:"mana,omitempty"`
	// Modes are VerbChooseModes' modes; N is how many are chosen.
	Modes []*Program `json:"modes,omitempty"`
	// Effects are VerbApplyEffect's and VerbCreateTokenCopy's effects.
	Effects []*Effect `json:"effects,omitempty"`
	// Abilities are abilities the step creates (VerbEmblem,
	// VerbDelayedTrigger, VerbCreateReplacement).
	Abilities []*Ability `json:"abilities,omitempty"`
	// Token is VerbCreateToken's token.
	Token *Face `json:"token,omitempty"`
	// Enter is how a moved or created object enters the battlefield.
	Enter *Enter `json:"enter,omitempty"`
	Src   string `json:"src,omitempty"`
}
