// Package decision defines the only vocabulary a client needs: the engine asks
// a Decision listing every legal Option, and the client answers with an Intent
// naming option indices. No rules knowledge crosses the wire.
package decision

import (
	"fmt"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

type Kind string

const (
	KPriority  Kind = "priority"
	KTarget    Kind = "target"
	KAttackers Kind = "attackers"
	KBlockers  Kind = "blockers"
	// KMulligan drives the pre-game London mulligan round (rules/mulligan.go),
	// which Config.Mulligans > 0 runs between the opening deal and turn 1. It
	// is one kind with two phases distinguished by option vocabulary and
	// Min/Max: a keep/mulligan ask is Min == Max == 1 over a "keep" (and,
	// while the seat still has a free mulligan, a "mulligan") option; a
	// bottoming ask is Min == Max == taken over one "bottom" option per kept-
	// hand card -- exactly the distinct-index shape Validate already enforces
	// for KTriggerOrder, so no new wire format is needed (Ruling U2).
	KMulligan Kind = "mulligan"
	// KModes is the modal pick a mid-resolution ask poses (M2d-2, closes
	// R-8): Min == Max == CharmNum$ (default 1) over one "mode" option per
	// Choices$ sub-ability, in Choices$ order, Player = the effect's
	// controller (or, for an UnlessCost$ may-pay, the payer). A charm's
	// mode choice executes the chosen modes in the chosen order through
	// ordinary Resolve; the engine's handleModes records the choice with a
	// ModeChosen event and re-enters the suspended resolution. The
	// "engines" ask and the unless-pay ask both use this kind -- ResumeKind
	// on the Decision tells the engine which continuation the answer
	// resumes.
	KModes Kind = "modes"
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
	// Attacker tells a block option's client which attacker this blocker
	// would block, so a human can see the pairing an in-process bot already
	// can (the declare-blockers step is otherwise guessing). omitempty
	// mirrors Obj: an ObjID of 0 means "no object", so an option that has
	// no attacker (any non-block option) emits no field.
	Attacker state.ObjID `json:"attacker,omitempty"`
	// AltCostIndex is server-side only: for a "cast" option, which cost this
	// option pays. Zero (the default, so every other Option literal in the
	// tree needs no change) means the card's own (RaiseCost/ReduceCost-
	// adjusted) cost; a value of i+1 means alternativeCosts(p, id)[i] -- an
	// AlternativeCost static's cost instead. Ruling T19b-b: without this,
	// castSpell had no way to tell which cost the player actually agreed to
	// pay when a card offered more than one "cast" option, and always paid
	// the card's own cost regardless of which option was chosen.
	AltCostIndex int `json:"-"`
	// Mode is server-side only: how a "cast" option pays -- "" the card's
	// own cost, "kicked", "surged", "flashback", "miracle".
	Mode string `json:"-"`
	// Amount is server-side only: for an "x" choose option, the X value the
	// option represents (the option's Index is its position in the list, not
	// its value -- see rules/cast.go's xAsk).
	Amount int `json:"-"`
	// Ability is server-side only: for an "ability" option, the index into
	// the source Face().Abilities of the activated ability being offered
	// (Task 10). Its zero value (default, so every other Option literal needs
	// no change) means "the card's own spell ability" -- the same reuse
	// AltCostIndex's zero value already describes.
	Ability int `json:"-"`
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
	// ResumeKind and ResumeSA are server-side only: how an effects.Host.Ask
	// mid-resolution decision (M2d-2) suspends and re-enters the resolution
	// it interrupted. The asking primitive sets them -- ResumeKind tags the
	// continuation ("modes" | "unless_pay") and ResumeSA names the exact
	// sub-ability whose effect asked, so the engine's resumeResolution can
	// re-enter the suspended chain at that point without re-running the
	// sub-abilities before it. Both are selected by the engine only inside
	// rules (handleModes); a client never sees them. Card data is shared
	// immutable compiled corpus, so the pointer is safe to carry across a
	// Clone and a replay like every other *cards.SA the engine holds.
	ResumeKind string    `json:"-"`
	ResumeSA   *cards.SA `json:"-"`
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
