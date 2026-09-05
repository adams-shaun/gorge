// Package decision defines the only vocabulary a client needs: the engine asks
// a Decision listing every legal Option, and the client answers with an Intent
// naming option indices. No rules knowledge crosses the wire.
package decision

import (
	"fmt"

	"github.com/adams-shaun/gorge/state"
)

type Kind string

const (
	KPriority  Kind = "priority"
	KTarget    Kind = "target"
	KAttackers Kind = "attackers"
	KBlockers  Kind = "blockers"
	KMulligan  Kind = "mulligan"
	KModes     Kind = "modes"
	// KTriggerOrder asks one controller for the order of the two or more
	// triggered abilities they control that triggered simultaneously (CR
	// 603.3b). It is Min == Max == len(Options) over exactly that
	// controller's own pending triggers, so Validate's existing "N distinct
	// in-range indices" rule already means "a permutation" and no new wire
	// format is needed (Ruling U2).
	//
	// DIRECTION, which is silent if a client gets it backwards: Choices[0]
	// is the trigger put on the stack FIRST, and therefore the one that
	// resolves LAST. That matches the between-player rule the engine applies
	// either side of this choice -- CR 603.3b's APNAP puts the active
	// player's triggers on the stack first and resolves them last -- so one
	// sentence describes the whole placement. The Decision's own Prompt says
	// the same thing in words a client can show a player unchanged.
	KTriggerOrder Kind = "trigger_order"
	// KTriggerOptional asks whether an optional triggered ability (Forge's
	// OptionalDecider$ on a T: line) is put on the stack at all. Min == Max
	// == 1 over exactly two options, Kind "yes" and Kind "no", in that
	// order. There is no default: an unanswered optional trigger never
	// reaches the stack, and neither does a declined one.
	KTriggerOptional Kind = "trigger_optional"
	// KChoose is one list-pick: choose between Min and Max of the offered
	// options. Every option in one decision shares a Kind that says what is
	// being chosen — "x" (a value for {X}; options ascend), "exile" (cards
	// to exile for Delve), "sacrifice" (a permanent to sacrifice as a cost),
	// "name"/"type"/"number" (an "as this enters" choice), "yes"/"no" (a
	// may-cast such as Miracle). The wire shape is the same as every other
	// decision; only the vocabulary of Option.Kind is new.
	KChoose Kind = "choose"
)

// Option is one legal choice. Obj and Player are echoed only so a client can
// highlight the object; selection is by Index.
type Option struct {
	Index int         `json:"index"`
	Kind  string      `json:"kind"`
	Label string      `json:"label"`
	Obj   state.ObjID `json:"obj,omitempty"`
	// Player is always emitted because 0 is a valid seat (0-indexed), unlike
	// Obj where 0 means "no object".
	Player state.PlayerID `json:"player"`
	// Attacker is server-side only: for a block option, which attacker this
	// blocker would block.
	Attacker state.ObjID `json:"-"`
	// AltCostIndex is server-side only: for a "cast" option, which cost this
	// option pays. Zero (the default, so every other Option literal in the
	// tree needs no change) means the card's own (RaiseCost/ReduceCost-
	// adjusted) cost; a value of i+1 means alternativeCosts(p, id)[i] -- an
	// AlternativeCost static's cost instead. Ruling T19b-b: without this,
	// castSpell had no way to tell which cost the player actually agreed to
	// pay when a card offered more than one "cast" option, and always paid
	// the card's own cost regardless of which option was chosen.
	AltCostIndex int `json:"-"`
}

// Decision is the engine asking one player for one answer.
type Decision struct {
	Seq     uint64         `json:"seq"`
	Player  state.PlayerID `json:"player"`
	Kind    Kind           `json:"kind"`
	Prompt  string         `json:"prompt"`
	Min     int            `json:"min"`
	Max     int            `json:"max"`
	Options []Option       `json:"options"`
	// Source is server-side only: the object this decision resolves for.
	Source state.ObjID `json:"-"`
}

// Intent is a client's answer.
type Intent struct {
	Seq     uint64         `json:"seq"`
	Player  state.PlayerID `json:"player"`
	Choices []int          `json:"choices"`
}

// Validate rejects anything the engine did not offer. Everything a client can
// get wrong is caught here, which is what lets the client stay rules-ignorant.
func (d *Decision) Validate(in Intent) error {
	if in.Seq != d.Seq {
		return fmt.Errorf("intent seq %d, pending decision seq %d", in.Seq, d.Seq)
	}
	if in.Player != d.Player {
		return fmt.Errorf("intent from player %d, decision is for player %d", in.Player, d.Player)
	}
	if len(in.Choices) < d.Min || len(in.Choices) > d.Max {
		return fmt.Errorf("expected %d..%d choices, got %d", d.Min, d.Max, len(in.Choices))
	}
	seen := make(map[int]bool, len(in.Choices))
	for _, c := range in.Choices {
		if c < 0 || c >= len(d.Options) {
			return fmt.Errorf("choice %d out of range (%d options)", c, len(d.Options))
		}
		if seen[c] {
			return fmt.Errorf("duplicate choice %d", c)
		}
		seen[c] = true
	}
	return nil
}

// Chosen resolves an intent to options, in the order the client sent them.
// Returns nil if any index is out of range [0, len(d.Options)).
// Validate is the sanctioned path for validation; Chosen returns nil rather than
// panicking on indices it was not given a chance to validate.
func (d *Decision) Chosen(in Intent) []Option {
	// All-or-nothing: if ANY index is out of range, return nil
	for _, c := range in.Choices {
		if c < 0 || c >= len(d.Options) {
			return nil
		}
	}
	out := make([]Option, 0, len(in.Choices))
	for _, c := range in.Choices {
		out = append(out, d.Options[c])
	}
	return out
}
