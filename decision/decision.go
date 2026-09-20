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
	// KModes is a modal pick: MinCharmNum$ (default CharmNum$) through
	// CharmNum$ (default 1) over one "mode" option per Choices$ sub-ability,
	// in Choices$ order. Spell modes
	// are announced during casting (CR 601.2b), trigger modes at placement
	// (CR 603.3c), while nested Charm and unless-pay asks may suspend
	// resolution. handleModes records ModeChosen; ResumeKind and the trigger
	// drain flag select the appropriate continuation.
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
	// exactly one option per hand card, in hand order), "search" (an ordered
	// subset of matching cards from a hidden library, with Min 0), "dig" (the
	// cards a Dig look-and-take moves from the top DigNum$ window to
	// DestinationZone$, Min 0 when Optional$ True else ChangeNum, one option
	// per ELIGIBLE card in library order), "name"/"type"/"number" (an "as this
	// enters" choice), "yes"/"no" (a may-cast such as Miracle).
	// The wire shape is the same as every other decision; only the vocabulary
	// of Option.Kind is new.
	KChoose Kind = "choose"
	// KReplacement is a choice about applying a replacement effect. For CR
	// 616.1 competition it is Min == Max == 1 over the currently applicable
	// replacements, in deterministic scan order; the affected player chooses
	// which applies next, each option has Kind "replacement", and Obj names
	// its source permanent. The event is parked, and applicability is checked
	// again after each rewrite. A choice-valued mana replacement then uses five
	// Kind "mana" options labelled Add W/U/B/R/G while that ManaAdd remains
	// parked. BeginPhase competition uses the same replacement options; after
	// an Optional$ effect is selected, options "apply" and "decline" ask
	// whether it gets its opportunity. A decline continues through every
	// remaining applicable phase replacement before the StepChange is logged.
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

// Kinds lists every decision Kind in declaration order. It is the static
// universe a coverage report needs to say which kinds a run NEVER asked --
// an engine ask cannot use a kind outside this slice, so universe and
// observed cannot drift the way a hand-copied list in another package
// would. Keep it in the same order as the constants above.
var Kinds = []Kind{
	KPriority, KTarget, KAttackers, KBlockers, KMulligan, KModes,
	KTriggerOrder, KTriggerOptional, KCommanderZone, KChoose, KReplacement,
	KArrange,
}

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
	// Required marks an attacker option whose creature MUST attack this
	// combat (CR 508.1d): a goaded creature (CR 701.38) or one under an
	// unconditional MustAttack static. A rules-ignorant client needs the
	// flag because the engine REJECTS a declaration that omits a required
	// creature it could have included (validateAttackDeclaration) -- an
	// omission that looks legal on the wire otherwise. omitempty: a
	// non-required option emits no field, so every existing option list
	// serialises byte-identically.
	Required bool `json:"required,omitempty"`
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
	// SVar anchors a "granted" option (rules/speed.go, the kw:Start your
	// engines max-speed static's AddAbility$): the SVar name on the source
	// face whose AB the activation resolves through. A granted ability is
	// not a Face().Abilities index (the ordinary "ability" anchor), so it
	// carries the name instead; beginGrantedActivation re-resolves it, so a
	// stale name degrades to a no-op. omitempty: only granted options carry
	// it.
	SVar string `json:"svar,omitempty"`
	// Cost is the activation cost of a priority-window "activate" option whose
	// mana ability costs MORE than a bare tap, in the same whitespace-delimited
	// Forge notation AbilityCosts uses (rules/mana.go's formatCost over
	// ParseCost of the ability's Cost$ param) — "T Sac<1/Lion's Eye Diamond>"
	// for Lion's Eye Diamond, "T PayLife<1>" for Mana Confluence. It is the
	// wire marker the client's empty-priority-window floor and auto-pass need
	// (fb-20260917T192520Z-26136705): isActionKind excludes every "activate"
	// because a bare tap is offered at every window and is not a play, but a
	// costly activation is exactly the play a ritual-combo deck needs the
	// window for, and an empty hand leaves it the window's ONLY action — the
	// floor passed it away unseen, and the card was unreachable for the rest
	// of the game. A bare tap (every plain land) omits the field, so every
	// existing option list and every plain-land window serialises
	// byte-identically. omitempty: only a beyond-tap activation carries it.
	Cost string `json:"cost,omitempty"`
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
	// Repeatable relaxes Validate's no-duplicate-index rule: when true the
	// SAME option index may be chosen more than once in one answer. It is
	// set only by a modal (Charm) decision whose SA carries
	// CanRepeatModes$ True -- CR 601.2b's "you may choose the same mode more
	// than once" -- where the option list is the distinct modes and the
	// answer is an ordered multiset of them. Every other decision keeps the
	// strict rule. omitempty: a non-repeatable decision carries no field, so
	// every existing payload serialises byte-identically.
	Repeatable bool `json:"repeatable,omitempty"`
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
	// ResumeKind, ResumeSA, ResumeModes, ResumeTarget, ResumeChoices and
	// ResumeRemembered are server-side only.
	// ResumeKind selects a cast/placement/resolution continuation ("cast_modes",
	// "modes", "unless_pay", "discard", "arrange", "search", "imprint",
	// "untap", "dig"); ResumeSA
	// names the exact sub-ability involved. ResumeModes maps a filtered cast-time
	// mode option back to its SVar name while keeping wire indices dense.
	// ResumeTarget is Dig's index into the deterministic Defined$ target list:
	// re-entry applies the answer to exactly the library that asked, skips
	// targets already completed before suspension, and preserves deterministic
	// processing for later targets. rules alone selects these fields; clients
	// never see them. Card data is shared immutable compiled corpus, so the SA
	// pointer is safe across Clone/replay.
	ResumeKind   string    `json:"-"`
	ResumeSA     *cards.SA `json:"-"`
	ResumeModes  []string  `json:"-"`
	ResumeTarget int       `json:"-"`
	// Rolls is engine-internal context for the one KChoose that asks a
	// player to choose among ALREADY-ROLLED dice (effects/dice.go's
	// ChosenSVar$/OtherSVar$ shape, the Endeavor cycle): the per-die results
	// the asking first pass rolled, in roll order, so a rules-side resume
	// point can carry them across the suspension and publish chosen/other
	// without re-rolling (a re-roll would both re-draw the seeded generator
	// and make the choice answer a different question). Each "roll" option's
	// Index names a slot in this slice. Server-side only (json:"-"): a
	// replay re-derives the same rolls from the same seeded draws.
	Rolls []int32 `json:"-"`
	// ResumeChoices carries selections completed by earlier per-player choice
	// asks. It is runtime continuation state, never client input.
	ResumeChoices     []state.Target `json:"-"`
	ResumeChosenValid bool           `json:"-"`
	ResumeRemembered  []state.Target `json:"-"`
	// ResumeMoved carries the objects a ShuffleNonMandatory$ search's first
	// pass already moved (Path to Exile, Stoneforge Mystic): the may-shuffle
	// confirm suspends after the moves, and the re-entry's LibraryPosition$
	// placement needs the moved list the suspension lost. It is runtime
	// continuation state, never client input, the same class as
	// ResumeRemembered.
	ResumeMoved []state.ObjID `json:"-"`
	// ResumeUptoIdx/ResumeUptoCount ride an Upto$ Draw's in-flight per-target
	// state across a Dredge ask parked inside that target's answered batch
	// (Arcane Denial's "may draw up to two"): the re-entering upto branch
	// resumes exactly that target's remaining draws instead of re-asking a
	// decision already answered. Idx -1 (the default every non-upto caller
	// leaves) means no upto is in flight. Runtime continuation state, never
	// client input, the same class as ResumeMoved.
	ResumeUptoIdx   int   `json:"-"`
	ResumeUptoCount int32 `json:"-"`
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
		if seen[c] && !d.Repeatable {
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

// PotentialAction is one action a seat COULD take if it first floated every
// mana its untapped sources could produce: the engine's own legal-offer walk
// (rules/legal.go) priced against a hypothetical pool instead of the floating
// one. It is the server-side answer to the float-then-cast payment model --
// the engine prices a cast against the FLOATING pool only, so the priority
// window carries no cast option yet, and a client that re-derives
// "castable after tapping" on its own (printed costs, no live modifiers,
// no command zone, no flashback) drifts from the engine on every cost rule
// (Thalia's RaiseCost, a Medallion's ReduceCost, an Indeterminate Tron
// source, an X spell at 0). The projection carries only what the client's
// stop decisions need: which kind of action and where its object lives.
// A card/ability id is NOT a promise the action is currently offered -- it is
// a promise the engine WOULD offer it once the mana floated.
type PotentialAction struct {
	// Kind is the action kind, the same vocabulary decision.Option uses but
	// restricted to real plays: "cast", "ability" and "play_land". The mana
	// tap ("activate"), pass and concede are deliberately absent -- they are
	// offered at every priority window and are never a play.
	Kind string `json:"kind"`
	// Obj is the card or permanent the action names (the spell to cast from
	// hand/command zone/graveyard, or the source of the ability), 0 when the
	// action has no object. The id is the CardView id the client already has.
	Obj state.ObjID `json:"obj,omitempty"`
	// Ability anchors an "ability" potential action to its exact activated
	// ability, the index into the source Face().Abilities, exactly as
	// Option.Ability does. omitempty: casts and land drops carry no field.
	Ability int `json:"ability,omitempty"`
	// Mode distinguishes a "cast" potential action's payment kind ("",
	// "kicked", "surged", "flashback", "miracle"), exactly as Option.Mode
	// does. omitempty: an ordinary cast carries no field.
	Mode string `json:"mode,omitempty"`
	// Label is the offer label ("Cast X", "Name: ability text") -- the same
	// string the corresponding Option would carry, so a client can surface
	// the action without re-deriving it. omitempty: never empty in practice,
	// but a defensive omit keeps the wire free of empty strings.
	Label string `json:"label,omitempty"`
}
