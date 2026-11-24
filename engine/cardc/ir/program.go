package ir

import "encoding/json"

// Verb is the action of a Step.
type Verb uint16

const (
	VerbNone Verb = iota
	verbCount
)

var verbNames = [...]string{
	VerbNone: "none",
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
type Program struct {
	Steps []*Step `json:"steps,omitempty"`
	Src   string  `json:"src,omitempty"`
}

// Step is one action of a Program.
type Step struct {
	Verb     Verb     `json:"verb,omitempty"`
	Optional bool     `json:"optional,omitempty"`
	Who      *Ref     `json:"who,omitempty"`
	Obj      *Ref     `json:"obj,omitempty"`
	N        *Value   `json:"n,omitempty"`
	Cond     *Cond    `json:"cond,omitempty"`
	Then     *Program `json:"then,omitempty"`
	Else     *Program `json:"else,omitempty"`
	Dur      Duration `json:"dur,omitempty"`
	From     Zone     `json:"from,omitempty"`
	To       Zone     `json:"to,omitempty"`
	Word     string   `json:"word,omitempty"`
	Colors   ColorSet `json:"colors,omitempty"`
	Src      string   `json:"src,omitempty"`
}
