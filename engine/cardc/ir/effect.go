package ir

import "encoding/json"

// EffectKind is the kind of an Effect: a change an ability makes to an object,
// a player or the rules.
type EffectKind uint16

const (
	EffNone EffectKind = iota
	EffAdjustPT
	EffSetPT
	EffAddAbility
	EffAddCardType
	EffAddSubtype
	EffSetCardType
	EffSetSubtype
	EffAllCreatureTypes
	EffSetColor
	EffLoseAllAbilities
	EffSetName
	EffCDAPower
	EffCDAToughness
	EffCDAColor
	EffCantBlock
	EffCantBeBlocked
	EffCantBeBlockedBy
	EffCantBeTargeted
	EffNoUntap
	EffMustAttack
	EffCantCast
	EffCantLose
	EffCantWin
	EffSpellsCostLess
	EffSpellsCostMore
	EffNoMaxHandSize
	EffHexproof
	EffMayPlay
	EffMayCast
	EffFlash
	EffCastWithoutPaying
	EffExtraLand
	EffCantBeCountered
	EffAdditionalCost
	EffAlternativeCost
	EffSelfCostLess
	effectKindCount
)

var effectKindNames = [...]string{
	EffNone:              "none",
	EffAdjustPT:          "adjust-pt",
	EffSetPT:             "set-pt",
	EffAddAbility:        "add-ability",
	EffAddCardType:       "add-card-type",
	EffAddSubtype:        "add-subtype",
	EffSetCardType:       "set-card-type",
	EffSetSubtype:        "set-subtype",
	EffAllCreatureTypes:  "all-creature-types",
	EffSetColor:          "set-color",
	EffLoseAllAbilities:  "lose-all-abilities",
	EffSetName:           "set-name",
	EffCDAPower:          "cda-power",
	EffCDAToughness:      "cda-toughness",
	EffCDAColor:          "cda-color",
	EffCantBlock:         "cant-block",
	EffCantBeBlocked:     "cant-be-blocked",
	EffCantBeBlockedBy:   "cant-be-blocked-by",
	EffCantBeTargeted:    "cant-be-targeted",
	EffNoUntap:           "no-untap",
	EffMustAttack:        "must-attack",
	EffCantCast:          "cant-cast",
	EffCantLose:          "cant-lose",
	EffCantWin:           "cant-win",
	EffSpellsCostLess:    "spells-cost-less",
	EffSpellsCostMore:    "spells-cost-more",
	EffNoMaxHandSize:     "no-max-hand-size",
	EffHexproof:          "hexproof",
	EffMayPlay:           "may-play",
	EffMayCast:           "may-cast",
	EffFlash:             "flash",
	EffCastWithoutPaying: "cast-without-paying",
	EffExtraLand:         "extra-land",
	EffCantBeCountered:   "cant-be-countered",
	EffAdditionalCost:    "additional-cost",
	EffAlternativeCost:   "alternative-cost",
	EffSelfCostLess:      "self-cost-less",
}

// String returns the lower-kebab name of k.
func (k EffectKind) String() string { return enumString(effectKindNames[:], k, "EffectKind") }

// ParseEffectKind is the inverse of String.
func ParseEffectKind(s string) (EffectKind, bool) {
	return parseEnum[EffectKind](effectKindNames[:], s)
}

// MarshalJSON encodes k by name.
func (k EffectKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON decodes a name, rejecting unknown ones.
func (k *EffectKind) UnmarshalJSON(b []byte) error {
	return unmarshalEnum(k, b, "EffectKind", ParseEffectKind)
}

// Effect is one change a step applies (VerbApplyEffect) or a token copy or
// entering object carries.
type Effect struct {
	Kind EffectKind `json:"kind,omitempty"`
	// P and T are power and toughness amounts.
	P *Value `json:"p,omitempty"`
	T *Value `json:"t,omitempty"`
	// N is a count or generic amount.
	N *Value `json:"n,omitempty"`
	// Words are type, subtype or name words.
	Words  []string `json:"words,omitempty"`
	Colors ColorSet `json:"colors,omitempty"`
	// Abilities are the granted abilities.
	Abilities []*Ability `json:"abilities,omitempty"`
	Filter    *Filter    `json:"filter,omitempty"`
	Ref       *Ref       `json:"ref,omitempty"`
	Cost      *Cost      `json:"cost,omitempty"`
	Cond      *Cond      `json:"cond,omitempty"`
	Src       string     `json:"src,omitempty"`
}

// Enter is how an object enters the battlefield.
type Enter struct {
	Tapped bool `json:"tapped,omitempty"`
	// UnderOwner: under its owner's control.
	UnderOwner bool `json:"underOwner,omitempty"`
	// Controller: under that player's control.
	Controller *Ref `json:"controller,omitempty"`
	// Counter is a counter word; N is how many.
	Counter string `json:"counter,omitempty"`
	N       *Value `json:"n,omitempty"`
	// Effects: it enters with these effects.
	Effects []*Effect `json:"effects,omitempty"`
	Src     string    `json:"src,omitempty"`
}
