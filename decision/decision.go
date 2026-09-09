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
	// KCommanderZone asks a commander's OWNER what happens to the commander
	// when it is about to be put into its owner's graveyard, hand or library
	// from anywhere, or exiled from anywhere: the owner may put it into the
	// command zone instead (CR 903.9). Min == Max == 1 over exactly two
	// options, in this order: index 0 Kind "command_zone" (put it into the
	// command zone), index 1 Kind "leave" (let the zone change happen as it
	// would have). It is the OWNER who answers, never the controller -- a
	// stolen commander is sent to its owner's command zone by its owner's
	// choice -- so Player is always the owner. A decline changes nothing: the
	// original zone change happens unchanged (the engine's park re-emits the
	// deferred event verbatim).
	KCommanderZone Kind = "commander_zone"
	// KChoose is one list-pick: choose between Min and Max of the offered
	// options. Every option in one decision shares a Kind that says what is
	// being chosen — "x" (a value for {X}; options ascend), "exile" (cards
	// to exile for Delve), "sacrifice" (a permanent to sacrifice as a cost),
	// "discard" (cards the active player's cleanup step discards down to the
	// maximum hand size, CR 514.1; Min == Max == len(hand) - maxHandSize over
	// exactly one option per hand card, in hand order), "name"/"type"/"number"
	// (an "as this enters" choice), "yes"/"no" (a may-cast such as Miracle).
	// The wire shape is the same as every other decision; only the vocabulary
	// of Option.Kind is new.
	KChoose Kind = "choose"
	// KReplacement is CR 616.1's order choice: two or more replacement effects
	// are trying to modify the way one event affects an object, and the
	// affected player (the controller of the affected object) chooses the
	// order in which they apply. Min == Max == 1 over one option per
	// competing replacement, in the deterministic scan order the engine
	// found them in; each option's Kind is "replacement" and its Obj is the
	// source permanent that owns that replacement. Posed BEFORE anything
	// relocates (the modified event is parked), so answering never sees the
	// object already moved.
	KReplacement Kind = "replacement"
	// KArrange is the ordered-subset ask a library-arranging effect poses
	// (Ruling J0): the engine offers N cards, and the answer is an ordered
	// subset of them -- the one general decision shape Scry, Surveil and
	// Dig later share.
	//
	// The contract, stated once because a client that gets it backwards gets
	// it silently wrong: the engine offers N cards. The answer is an ordered
	// subset of them. The chosen indices, IN THE ORDER THE ANSWER GIVES THEM,
	// become pile A in that order. The options NOT chosen, in the order they
	// were OFFERED, become pile B in that order. Min/Max bound pile A's size.
	// Every option in one KArrange decision shares an Option.Kind naming
	// pile B's destination -- "bottom", "graveyard", "exile", "hand" -- so
	// a rules-ignorant client can say "the ones you pick stay on top in the
	// order you pick them; the rest go to <destination>" without learning a
	// rule.
	//
	// DIRECTION, which is silent if a client gets it backwards (and which
	// KTriggerOrder's own comment phrases the same way): pile A index 0 is
	// the card that ends up CLOSEST TO THE TOP -- the next card drawn.
	//
	// The wire shape is exactly the one Validate already enforces for
	// KTriggerOrder: an ordered list of distinct in-range indices (a
	// permutation when Min == Max == N). Min/Max bound pile A's size; a
	// full order (RearrangeTopOfLibrary, Ponder) is Min == Max == N with
	// pile B empty, while a Scry-2 gives Min == Max == 1 over two options
	// with the unchosen one heading to pile B ("bottom"). No new wire
	// format is needed -- only the kind is new. The one option per offered
	// card carries that card in Obj, so pile A/B are rebuilt from the
	// answer and the option list without re-reading any zone.
	KArrange Kind = "arrange"
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
	// Group is an exclusivity marker: two options carrying the SAME non-empty
	// Group are mutually exclusive, and at most one of them may be selected
	// in a single answer. The whole contract is that sentence -- it says
	// nothing about blockers, creatures or combat, which is exactly so a
	// rules-ignorant client may enforce it without learning any rules. A
	// client that sees the player pick an option whose Group is already
	// represented in the picked set naturally REPLACES the previously picked
	// option from that group (moving a blocker from one attacker to another
	// should just work) rather than refusing the click. No two options of
	// one Group may be selected together, which Decision.Validate enforces as
	// a general rule.
	Group string `json:"group,omitempty"`
	// AltCostIndex says which cost a "cast" option pays: 0 is the card's own
	// (RaiseCost/ReduceCost-adjusted) cost, i+1 is alternativeCosts(p, id)[i]
	// -- an AlternativeCost static's cost instead -- so a client can show
	// which of several costs the option pays. omitempty mirrors Obj: an
	// option paying the card's own cost (the common case, and the default
	// every other Option literal in the tree relies on) carries no field, so
	// today's payloads are unchanged for it.
	AltCostIndex int `json:"alt_cost_index,omitempty"`
	// Mode distinguishes a "cast" option's payment kind: "" the card's own
	// cost, "kicked", "surged", "flashback", "miracle" -- what the engine
	// reads in beginCast's switch. A client renders a kicked/surged/
	// flashback/miracle cast differently from an ordinary one instead of
	// parsing the label for a keyword. omitempty: an ordinary cast (Mode "")
	// carries no field.
	Mode string `json:"mode,omitempty"`
	// Amount is the X value an "x" choose option represents. The option's
	// Index is its position in the list, not its value (see rules/cast.go's
	// xAsk), so without this field a client could not tell "X = 4" from
	// "X = 1" without rereading the label. omitempty: only x options carry
	// it, and on an x option a missing field is exactly X = 0 (the one value
	// that omits), which the option's own label "X = 0" already shows.
	Amount int `json:"amount,omitempty"`
	// Ability anchors an "ability" option to its exact activated ability:
	// the index into the source Face().Abilities being offered (Task 10), so
	// a client can pop that ability's own text up beside the right ability
	// on the card. The engine reads it in beginActivation, where a stale
	// index degrades to a no-op. omitempty: options that are not ability
	// options carry no field, and on an ability option a missing field is
	// index 0 (the first ability), the one value that omits.
	Ability int `json:"ability,omitempty"`
	// Grant is server-side only (json:"-") and present only on an "ability"
	// option whose whole activation is a PURE, IDEMPOTENT keyword grant (the
	// ability adds one or more keywords and nothing additive -- no
	// power/toughness change, no counters, no damage, no draw). It is what
	// lets the bot policy's no-op rule (A1) tell a keyword grant that can
	// gain nothing (already in effect, or an identical one already pending
	// from the same source) from an additive ability that genuinely stacks
	// and must stay freely repeatable. It is filled by rules/legal.go from
	// the engine's own derived-keyword facts and the stack -- a human
	// client never sees it, so it is never on the wire.
	Grant *Grant `json:"-"`
}

// Grant describes the idempotent keyword grant of one "ability" option
// (decision.Option.Grant, server-side only). A nil Grant on an ability
// means the activation is NOT a pure keyword grant -- it has an additive
// component (a stat change, a counter, damage, a draw) or is not a keyword
// grant at all -- and such abilities always stack, so they are never a
// no-op. Only a non-nil Grant can be redundant, and it is redundant exactly
// when a granted keyword is already in effect on the granting permanent
// (Already) or an identical grant from the same source is already on the
// stack unresolved (Duplicate) -- the two independent halves of "does
// activating this again change anything".
type Grant struct {
	// Keywords are the keywords this activation adds, in the ability's own
	// KW$ order.
	Keywords []string
	// Already is true when the granting permanent already has every keyword
	// in Keywords -- the grant is already in effect from an earlier
	// resolution this turn, so activating it again changes nothing.
	Already bool
	// Duplicate is true when an identical activation from the same source
	// (same granted keywords) is already on the stack unresolved, so
	// resolving another copy would not add the keyword a second time.
	Duplicate bool
}

// TargetEffect describes only the active SA being targeted, not its parent,
// sub-abilities or the eventual outcome. API is the compiled primitive name
// (e.g. DealDamage, Destroy, Counter, Draw). Consumers must treat unfamiliar
// APIs conservatively; ChangeZone alone does not imply hostile removal.
// This contains no script text, hidden state or server continuation pointers.
type TargetEffect struct {
	API string `json:"api"`
	// Damage is present only for recognised direct damage primitives
	// (DealDamage and DamageAll). Absence is not proof that a whole spell's
	// other abilities cannot deal damage.
	Damage *DamageEffect `json:"damage,omitempty"`
}

// DamageEffect describes nominal scripted damage, NEVER guaranteed damage.
// Prevention, replacement, conditions, division among targets and resolution
// legality are not evaluated. Spell damage is not commander combat damage.
type DamageEffect struct {
	// Amount is a nonnegative literal, or nil (JSON null) if absent, dynamic,
	// invalid or outside the supported literal range. In particular X and
	// SVar expressions stay unknown even if the engine could evaluate them.
	// A known zero is a non-nil pointer to 0. There is deliberately no numeric
	// default: Go consumers must check nil before dereferencing; wire consumers
	// must check null before arithmetic. This is not a lethal-damage claim.
	Amount *int `json:"amount"`
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
	// Source names the object this decision resolves for -- the spell whose
	// {X} is being chosen, the card whose "as it enters" choice is pending
	// -- so a prompt can always name its source (survey #18) without the
	// client guessing it from the option labels. omitempty mirrors Obj: an
	// ObjID of 0 means "no object", so decisions that do not resolve for a
	// specific object (priority, mulligan, trigger order) carry no field and
	// today's payloads are unchanged for them.
	Source state.ObjID `json:"source,omitempty"`
	// TargetEffect is host-independent targeting context. It is absent on
	// other decision kinds and on older servers; absent means unknown.
	TargetEffect *TargetEffect `json:"target_effect,omitempty"`
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

// New is a convenience constructor that fills a Decision's Player, Kind,
// Prompt, Min, Max and Options fields from positionally-presented arguments.
// It also enforces Options[i].Index == i for the options it is handed -- the
// position/Index identity a client's intent and Chosen both rely on (a
// client names option i by choosing index i, and Chosen returns Options[i])
// -- so a call that passes a drifting list fails loudly here rather than on
// the path to a seat.
//
// Note that New is NOT the enforcement point for that identity: the engine's
// authoritative guard lives in rules' Engine.ask, through which every
// Decision that can reach a seat flows (casting a decision away from there
// leaves it not pending, so no seat is ever offered it). New's own check
// therefore backstops call sites that use it -- today only the mulligan
// round -- and is redundant there, not load-bearing. The great majority of
// construction sites build &decision.Decision struct literals directly and
// are covered by ask alone. New deliberately panics on a mis-indexed list
// rather than returning an error, and it preserves that behaviour as a
// convenience for its handful of callers, but the invariant is guarded
// regardless of which side of the constructor an option list arrives on.
// Source, if any, is set by the caller on the returned Decision; it carries
// no invariant.
func New(player state.PlayerID, kind Kind, prompt string, min, max int, options []Option) *Decision {
	for i := range options {
		if options[i].Index != i {
			panic(fmt.Sprintf("decision: option %d has Index %d, want position %d (%s)",
				i, options[i].Index, i, kind))
		}
	}
	return &Decision{Player: player, Kind: kind, Prompt: prompt, Min: min, Max: max, Options: options}
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
	seenGroups := make(map[string]int, len(in.Choices))
	for _, c := range in.Choices {
		if c < 0 || c >= len(d.Options) {
			return fmt.Errorf("choice %d out of range (%d options)", c, len(d.Options))
		}
		if seen[c] {
			return fmt.Errorf("duplicate choice %d", c)
		}
		seen[c] = true
		// The exclusivity rule: two options sharing one non-empty Group are
		// mutually exclusive, so an intent must not select both. This is a
		// general wire contract, not a combat rule -- the group field says
		// nothing about what its members are, only that they are exclusive.
		if g := d.Options[c].Group; g != "" {
			if first, ok := seenGroups[g]; ok {
				return fmt.Errorf("choices %d and %d are mutually exclusive (group %q)", first, c, g)
			}
			seenGroups[g] = c
		}
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
