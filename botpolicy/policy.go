// Package botpolicy holds the one bot decision policy, in one copy.
//
// Both hosts that need a bot policy -- seat.Bot, answering a view.View, and
// the rules package's fuzz-test testBot, answering with only a *state.Game
// in hand -- build a Board and call Decide. rules cannot import seat without
// running the dependency order backwards (cards -> state -> decision ->
// events -> effects -> rules -> view -> seat; Ruling F7), so before this
// package existed the two sides each carried a line-for-line copy of the
// policy under a "keep the two in step" comment. This package is that
// comment made code: there is one policy, and the two adapters are thin
// value constructions (seat/bot.go's boardFromView; rules/testbot_test.go's
// answer) feeding the same Decide with the same rng consumption points.
// seat/integration_test.go's TestBotAdaptersAgree* pins the two halves to
// the same Board for the same game facts.
package botpolicy

import (
	"math/rand/v2"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// Board is the plain-data picture of the game the policy reads. Every fact
// the policy branches on lives here, produced identically by the two
// adapter halves — seat/bot.go's boardFromView lifts it off the projected
// View a real client receives, and BoardFromGame (combat.go) lifts it off
// the engine's state.Game — so whatever the heuristic sees, whichever host
// asks, is the same board (pinned over a whole game by seat/integration_test.go's
// TestBotAdaptersAgreeOverWholeGame, and the commander twin pinned the
// same way by TestBotAdaptersAgreeOverCommanderGame). The fields are
// deliberately not speculative: a board fact no policy branch reads would
// be untested surface. Priority reads IsMain, Pool, Cards, Life and
// Commanders; combat reads Creatures, Life and Commanders; the cast scorer
// (cast.go) additionally reads Cast, FirstMain and MyTurn.
type Board struct {
	// IsMain reports whether sorcery-speed actions are legal right now.
	// The seat adapter lifts it off the projected View's Phase
	// ("main1"/"main2", seat/bot.go); the rules test adapter lifts it off
	// the engine's own step (g.Step.IsMain), which is the same function of
	// the same five-phase string.
	IsMain bool
	// Creatures is every creature on every battlefield, keyed by object id:
	// the attacker and blocker census the combat heuristic reads. Both
	// adapters fill it from public facts only — a battlefield is public
	// for every seat — so the bot reasons about exactly the creatures the
	// seat can legally see, and the two halves agree on every one of them.
	Creatures map[state.ObjID]Creature
	// Life is every player's life total, keyed by seat. The blocking rule
	// reads the defender's own life to decide when a chump block is
	// warranted, which is public on both halves.
	Life map[state.PlayerID]int32
	// Cards is the casting ranking's card-facts table, keyed by ObjID and
	// filled by both adapters for the DECIDING seat's own legally-seen
	// zones (its hand, graveyard, battlefield and command zone) from
	// cast.go (creature, power, mana value, basic-ness). The casting
	// policy (cast.go) reads an offered "cast"/"play_land" option's facts
	// here by Obj; the tap gate (tap.go) reads a card's printed mana cost
	// and its castability (which zone it sits in). The seat adapter fills
	// it off the projected CardViews the viewer receives; BoardFromGame
	// fills it off state.Game for the deciding seat — the two fill exactly
	// the same legal zones with the same derived facts, so a card ranks
	// the same whichever host asks (pinned over a whole game by
	// seat/integration_test.go's TestBotAdaptersAgreeOverWholeGame). A
	// hand/graveyard/battlefield/command-zone fact is the deciding seat's
	// own, so carrying it in the Board is no information leak (Ruling C0):
	// it is exactly what that seat may see.
	Cards map[state.ObjID]Card
	// Pool is the deciding seat's current mana pool. The tap gate
	// (tap.go) reads it to decide whether another tap could newly enable a
	// cast: a pool that already pays some castable card's cost is a pool
	// that needs no more tapping, and a pool that pays none of them is the
	// "keep tapping" signal. Both halves fill it from the same numbers —
	// the projected View's own-pool map (poolView, the exact pool the
	// engine state carries) and state.Game's Pool field — so the gate sees
	// the same pool whichever host asks (pinned non-vacuously by
	// seat/integration_test.go's pool agreement). A mana pool is the
	// deciding seat's own private state, like its hand, so carrying it in
	// the Board is no information leak (Ruling C0).
	Pool state.Mana
	// Commanders is the CR 903.6/903.10 commander bookkeeping, keyed by
	// object id: every commander object in the match (each player's
	// Commanders list, in Config order), with the CR 903.8 tax base
	// (Casts: times it has been cast from the command zone — the next such
	// cast costs an additional {2} per entry), whether it currently sits
	// in its owner's command zone (InCommandZone — the only zone a cast
	// of it is taxed), and the 21-damage clock (Damage: cumulative combat
	// damage it has dealt, keyed by the player who took it; nil when it
	// has dealt none). Both adapter halves fill it from public facts — the
	// command zone is public (m30 made ZCommand project), and commander
	// identity, cast counts and damage are open information — so the
	// casting rule and the combat clock read the same facts whichever
	// host asks. Damage is keyed by the commander, not its controller:
	// CR 903.10 charges a commander's tally whether the object currently
	// attacks for its owner or anyone else. A fact the policy never reads
	// (the commander's owner) is deliberately not carried.
	//
	//	Stack is the public stack census, bottom to top in the stack's own
	//	order (state.Game.Stack and view.View.Stack are both order-preserving,
	//	so the census is an ordered slice and no map iteration reaches any
	//	choice that reads it). The stack is a public zone (Ruling T23-u;
	//	view.View.Stack already carries Controller and Kind for exactly this),
	//	so carrying it is no information leak (Ruling C0). The casting rule
	//	(cast.go's C8) reads it to tell whose spells are on the stack: a
	//	counter cast is worth its mana only when a FOREIGN spell is there to
	//	counter, never at an own-spells-only (or empty) stack. Both adapter
	//	halves fill it identically (seat/bot.go's boardFromView off the
	//	projected StackView list; BoardFromGame off state.Game.Stack), pinned
	//	on every intent of a whole game by seat/integration_test.go's parity
	//	tests.
	Commanders map[state.ObjID]Commander
	// Stack is the public stack census, bottom to top in the stack's own
	// order (state.Game.Stack and view.View.Stack are both
	// order-preserving, so the census is an ordered slice and no map
	// iteration reaches any choice that reads it). The stack is a public
	// zone (Ruling T23-u; view.View.Stack already carries Controller and
	// Kind for exactly this), so carrying it is no information leak
	// (Ruling C0). The casting rule (cast.go's C8) reads it to tell whose
	// spells are on the stack: a counter cast is worth its mana only when
	// a FOREIGN spell is there to counter, never at an own-spells-only (or
	// empty) stack. Both adapter halves fill it identically (seat/bot.go's
	// boardFromView off the projected StackView list; BoardFromGame off
	// state.Game.Stack), pinned on every intent of a whole game by
	// seat/integration_test.go's parity tests.
	Stack []StackEntry
	// Cast is the learned cast profile the cast scorer dots its feature
	// vector with (cast.go's CastWeights). It is CONFIGURATION, not board
	// state: the adapters never fill it (BoardFromGameInto's refill contract
	// leaves it untouched, so a profile set once on a reused Board survives
	// every refill), and the zero value is treated as DefaultCastWeights --
	// the pre-refactor arithmetic, byte for byte (cast.go's castWeights).
	// A Board nobody configured plays the default bot.
	Cast CastWeights
	// FirstMain reports whether the current main phase is the FIRST one
	// (main1, not main2): the Precombat feature's board half. It is filled
	// exactly like IsMain on both adapter halves -- the projected View's
	// Phase string ("main1") on the view half, g.Step == state.StepMain1 on
	// the game half -- so both halves agree on it wherever they agree on
	// IsMain itself.
	FirstMain bool
	// MyTurn reports whether the deciding seat is the ACTIVE player (the
	// seat whose turn it is): the InstantOnOwnTurn feature's board half. A
	// main phase can belong to another seat (an opponent holding priority
	// during the active seat's main phase), so IsMain alone cannot say
	// "own main phase". Filled from the projected View's Active field on
	// the view half and g.Active on the game half, the same field both
	// halves already agree is public.
	MyTurn bool
}

// Commander is the Board's per-commander commander-format bookkeeping,
// filled identically by both adapter halves (seat/bot.go's boardFromView
// off the projected View's Commanders/CommanderCasts/CmdDamage fields,
// BoardFromGame off state.Game's Player.Commanders/CmdCasts/CmdDamage —
// combat.go). Casts is the CR 903.8 tax base: the next command-zone cast
// of this commander costs an additional {2} for each prior such cast, and
// the casting rule prices exactly that. InCommandZone is whether the
// commander currently sits in its owner's command zone — the only zone a
// "cast" of it is taxed (a hand cast is an ordinary cast). Damage is the
// CR 903.10 21-clock: cumulative combat damage this commander has dealt,
// keyed by the player who took it; nil (on BOTH halves) until it has
// dealt some.
type Commander struct {
	Casts         int32
	InCommandZone bool
	Damage        map[state.PlayerID]int32
}

// StackEntry is one object on the stack, the plain-data shape of a
// StackView the policy can read without importing view (Ruling F7).
// IsSpell is true for a spell object (a card cast onto the stack, which a
// counter can target) and false for an ability object (minted by a
// TriggerPush/AbilityPush — Face-less; the engine's target census skips
// Face-less stack objects (rules/stack.go), so no counter can be cast at one
// today. Ability-targeting counters (Stifle) and Defined$ ValidStack
// counter-all spells (Glen Elendra's Answer) are open engine gaps; revisit
// C8 when they land). Controller is the object's controller, which for a
// stack spell is the seat that cast it. Both adapter halves fill it the
// same way, so C8 judges the same stack whichever host asked.
type StackEntry struct {
	ID         state.ObjID
	Controller state.PlayerID
	IsSpell    bool
}

// closesClock reports whether an unblocked swing from the creature id —
// whose creature facts are a — would take player p to 21 or more
// commander damage from that commander (CR 903.10: a loss that ignores
// life). It is the single expression behind all four clock rules
// (cast.go's CR1 reads Damage only indirectly through the tax; combat.go's
// AR5/BR3/BR4 read it directly), so the attack and block heuristics price
// the same second track. A creature that is not a commander (no
// Commanders entry) or whose commander has never hit p (no Damage entry)
// never closes a clock.
func (b Board) closesClock(p state.PlayerID, id state.ObjID, a Creature) bool {
	cm, ok := b.Commanders[id]
	if !ok {
		return false
	}
	return cm.Damage[p]+a.Power >= 21
}

// Decide implements every decision.Kind the engine (or a future one) can
// ask, so the policy needs no rules knowledge either (Ruling P7):
//
//   - KPriority: tap for mana only in a main phase (Ruling T25-b -- tapping
//     during the upkeep or combat empties the pool before there is anything
//     worth spending it on), then make a land drop, then cast, then pass.
//     IsMain comes from b, so whichever adapter built the Board decides
//     the gate (the seat reads the View's Phase; the rules test host reads
//     e.G.Step.IsMain()). The "activate" block before everything reaches
//     the bot's tap-for-mana activations: legalActions (rules/legal.go)
//     offers only mana abilities as "activate" options ("Tap X for mana"),
//     never non-mana activated abilities (those are "ability"), so it
//     fills the pool in a main phase before the land drop and cast. Which
//     "activate" is tapped stays position-first within the offered group;
//     WHAT the gate asks before tapping any of them is need (tap.go, T1):
//     activate only while the pool cannot currently pay ANY card the seat
//     could cast from a castable zone (hand, command zone, graveyard with
//     Flashback), priced with the CR 903.8 commander tax when the card sits
//     in the command zone, and fall through to the land drop and the cast
//     the moment it can. The pool the gate reads is b.Pool, filled
//     identically by both adapter halves. The old position-first block
//     tapped until nothing untapped was left, floating mana in bulk and
//     letting each step's end (CR 500.4) empty whatever a cast never
//     spent; the need gate keeps the tap until the first cast opens and
//     lets the cast take over (measured, op6: the waste share of floated
//     mana fell from 32.9% to 22.8% constructed and 28.0% to 24.6%
//     four-seat commander, see the task report). The land
//     drop and the cast then order with chooseLand before chooseCast (G0,
//     cast.go).
//   - "concede" (M2d-3): never picked. It is another priority option kind,
//     served last after "pass", but no policy wants to leave the game it
//     is winning; the explicit kind scans below return before any blind
//     fallback, and clamp's top-up prefers "pass" (Ruling T25-g), which
//     legalActions (rules/legal.go) always offers. Unknown option kinds are
//     otherwise never chosen by this switch at all.
//   - KTarget: the targeting heuristic in target.go's chooseTargets (R1-R5,
//     stated there): never target your own permanent while an opposing
//     option exists, prefer the opponent's best battlefield creature over
//     their face (ranked by threat(), not pt() or raw Power), and honour
//     Min/Max without leaning on clamp. Consumes no rng.
//   - KAttackers/KBlockers: the combat heuristic in combat.go's
//     chooseAttackers/chooseBlockers (AR1-AR6 / BR1-BR4, stated there),
//     including the per-attacker defender choice (AR6) and the commander
//     clock (AR5/BR3/BR4). The opt-in LethalPressureDecide variant adds
//     AR7. Neither consumes the rng: the choice is a pure
//     function of the offered options and the board facts both adapters
//     supply.
//   - KTriggerOrder: a permutation of the offered indices drawn from the
//     bot's own rng, so ordering paths get fuzz coverage too.
//   - KTriggerOptional: a coin from the bot's own rng between the two
//     offered options ("yes" first, "no" second, per askTriggerOptional),
//     so both branches get coverage.
//   - KChoose: every option in one decision shares a Kind (Option.Kind, not
//     d.Kind) that says what is being chosen. "x" takes the highest option
//     (the most an {X} cost can pay for -- options ascend); "exile"/
//     "sacrifice" take the first Max options; "yes"/"no" always answers yes
//     (Kind "yes" is first, per how askers build the two-option list);
//     "name"/"type"/"number" take the first offer. No rng is consumed,
//     unlike KBlockers/KTriggerOrder/KTriggerOptional above.
//   - KMulligan: the London round (Config.Mulligans > 0) offers two shapes on
//     one kind. A bottoming decision (every option Kind "bottom", Min ==
//     Max == taken) bottoms the taken lowest-indexed cards -- Choices
//     [0,1,...,taken-1] in ascending index order. A keep/mulligan decision
//     mulligans with probability 1/3 off the bot's own rng when a
//     "mulligan" option is offered (the determinism mirror of
//     KTriggerOptional), otherwise keeps (the "keep" option at index 0). The
//     rng is consumed only where a real mulligan choice exists.
//   - KModes: a modal announcement or mid-resolution pick -- choose the first Min options
//     in order (Choices [0, 1, …, Min-1]), the recorded mirror of the
//     engine-side first-mode stand-in, no rng. This also answers an
//     UnlessCost$ may-pay, shaped as the same KModes kind: option 0 is
//     "Pay … — make a copy", so the policy always offers to pay and the
//     engine declines for it only when the payer's pool cannot cover the
//     cost.
//
// Anything else — any kind added later, and any case
// above that found nothing to pick — falls to the last resort: pass if one
// is offered and Min == 0, otherwise whatever clamp below tops up with.
//
// Ruling T25-c (fix round 1): every branch used to return its pick
// unclamped, so it only ever validated by coincidence of the Min/Max shapes
// today's engine happens to emit (rules/stack.go's askTarget already names
// TargetMin/TargetMax as coming). clamp is now the last thing every return
// does, so the totality guarantee holds by construction for any Min/Max the
// wire format allows, not only today's. Every access into d.Options remains
// guarded against the list being empty.
func Decide(b Board, d *decision.Decision, r *rand.Rand) decision.Intent {
	// main's 9be52252 promoted AR7 lethal pressure into the default bot, so the
	// default carries lethalPressure; the AR8 combined-lethal test stays opt-in
	// (CombinedLethalDecide only), per this branch's 162a8acc.
	return decide(b, d, r, true, false, false)
}

// LethalPressureDecide is the measured opt-in policy used by botbench. It is
// identical to Decide except that a combat attack which is lethal if
// unblocked is made even when the defender can trade for it cheaply.
func LethalPressureDecide(b Board, d *decision.Decision, r *rand.Rand) decision.Intent {
	return decide(b, d, r, true, false, false)
}

// CombinedLethalDecide is the opt-in AR8 bench policy: on top of the AR7
// per-attacker lethal test (LethalPressureDecide), a combat attack whose
// ATTACKING SET is lethal once the defender's minimum blocking response is
// subtracted is made even when no single attacker would be. It is exposed to
// cmd/botbench as the "ar8" policy and is NEVER wired into the hosted or
// production bot (the default Decide carries only AR7 lethal pressure, never
// the combined-lethal test).
func CombinedLethalDecide(b Board, d *decision.Decision, r *rand.Rand) decision.Intent {
	return decide(b, d, r, true, true, false)
}

// BlocksDecide is the opt-in BLK bench policy: identical to Decide except
// that KBlockers is answered by the whole-assignment heuristic in
// blocks.go (chooseBlockAssignment, rules B0-B3) instead of the per-blocker
// chooseBlockers. It is exposed to cmd/botbench as the "blocks" policy and
// is NEVER wired into the hosted or production bot.
func BlocksDecide(b Board, d *decision.Decision, r *rand.Rand) decision.Intent {
	return decide(b, d, r, true, false, true)
}

func decide(b Board, d *decision.Decision, r *rand.Rand, lethalPressure, combinedLethal, blocksAssignment bool) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	switch d.Kind {
	case decision.KPriority:
		if b.IsMain {
			// T1 (tap.go): the need-aware tap gate. The block this replaced
			// tapped the first "activate" option in every main phase until
			// every source was spent -- need-blind and position-first -- which
			// floated mana in bulk and let each step's end (CR 500.4) empty
			// whatever the cast never spent. chooseTap taps only while the
			// pool cannot pay any card the seat could cast from a castable
			// zone, and falls through (returns -1) the moment it can or there
			// is nothing to pay for, so the land drop below is reached with
			// unmade taps instead of exhausted ones.
			if pick := b.chooseTap(d); pick >= 0 {
				in.Choices = []int{pick}
				return clamp(d, in)
			}
		}
		// G0 (cast.go): the land drop ranks before the cast. A land drop is
		// free and unconditional and only raises the mana ceiling for the
		// rest of the turn, so taking it first is weakly strictly better
		// than casting first (which would only tie — you would play the
		// land next priority anyway — and can never enable more). The one
		// "cast before land" argument, choosing the right-coloured land for
		// the spell you intend, is the chooseLand ranking's own job (L1),
		// not this ordering's. chooseLand picks the best land, chooseCast the
		// best spell.
		if pick := b.chooseLand(d); pick >= 0 {
			in.Choices = []int{pick}
			return clamp(d, in)
		}
		if pick := b.chooseCast(d); pick >= 0 {
			in.Choices = []int{pick}
			return clamp(d, in)
		}
		// chooseAbility (ability.go, A1-A4) ranks the offered "ability"
		// options by value instead of taking the first: a provable no-op
		// (an equip on an already-attached permanent, or one with no
		// creature to attach to, A1 — equipNoOp reads the attachment state
		// itself, never a projected target) is never activated, cheaper
		// abilities outrank costlier ones (A2), ties break on option index
		// (A3), and the block falls through to the explicit pass below when
		// nothing ranks as worth taking (A4). legalActions only offers
		// non-mana activated abilities as legal sorcery-speed actions here,
		// so no extra isMain gate is needed beyond this block's own check.
		if b.IsMain {
			if pick := b.chooseAbility(d); pick >= 0 {
				in.Choices = []int{pick}
				return clamp(d, in)
			}
		}
		// Ruling T25-g (fix round 2): explicitly pass here, before clamp
		// ever runs. This is the common case -- outside a main phase, or
		// with nothing affordable in one -- and legalActions
		// (rules/legal.go) lists every "activate" option before "pass", so
		// without this, clamp's blind first-unused-index top-up (needed
		// because a priority decision is always Min:1/Max:1, so falling
		// through with in.Choices empty is not itself a legal answer) would
		// reach for an activation and reintroduce I-1(b) through the
		// fallback path -- exactly what fix round 1 missed, because its own
		// synthetic regression decision carried a play_land option the loop
		// above matches unconditionally, a shape the live engine never
		// offers outside sorcery speed. Pass is offered on every priority
		// decision the engine emits; if it is somehow absent, this falls
		// through to the shared last resort below, same as any other kind.
		// M2d-3: the "concede" option sits directly after "pass" in the
		// option list, so this explicit scan is also what keeps the bot from
		// ever conceding -- it returns pass before clamp, or any
		// position-based fallback, can reach the new final option.
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return clamp(d, in)
			}
		}

	case decision.KTarget:
		in.Choices = b.chooseTargets(d)
		return clamp(d, in)

	case decision.KAttackers:
		in.Choices = b.chooseAttackersMode(d, lethalPressure, combinedLethal)
		return clamp(d, in)

	case decision.KBlockers:
		if blocksAssignment {
			// BLK: the opt-in whole-assignment policy (blocks.go, B0-B3).
			in.Choices = b.chooseBlockAssignment(d)
		} else {
			in.Choices = b.chooseBlockers(d)
		}
		return clamp(d, in)

	case decision.KTriggerOrder:
		// dp1: no more Fisher-Yates. The order the bot's own simultaneous
		// triggers hit the stack is a deterministic ranking of the offered
		// triggers by their source card's worth (trigger.go's
		// chooseTriggerOrder), reading the same card-worth arithmetic the
		// cast rule reads and consuming no rng -- so the same game state
		// always orders the same way, and the randomisation that widened
		// the bench's confidence intervals is gone.
		if n := len(d.Options); n > 0 {
			in.Choices = b.chooseTriggerOrder(d)
			return clamp(d, in)
		}

	case decision.KTriggerOptional:
		// dp1: no more coin flip. An optional trigger is a controller
		// benefit the policy cannot read, so it accepts it (see the rule
		// stated in trigger.go) rather than gambling -- deterministic, so the same
		// game state always answers the same way and the coin's variance is
		// gone. The "yes" option is index 0 (askTriggerOptional builds
		// yes-first, per the kind's contract).
		if len(d.Options) > 0 && d.Options[0].Kind == "yes" {
			in.Choices = []int{d.Options[0].Index}
		}
		return clamp(d, in)

	case decision.KCommanderZone:
		// Share CR1's CMC-scaled penalty and its nonnegative acceptance
		// boundary. Leaving does not guarantee access from the destination.
		if len(d.Options) > 0 && d.Options[0].Kind == "command_zone" {
			src := d.Options[0].Obj
			if b.cardWorth(src)-b.commandTax(src) >= 0 {
				in.Choices = []int{d.Options[0].Index}
			} else {
				for _, o := range d.Options {
					if o.Kind == "leave" {
						in.Choices = []int{o.Index}
						break
					}
				}
			}
		}
		return clamp(d, in)

	case decision.KChoose:
		if len(d.Options) == 0 {
			break
		}
		switch d.Options[0].Kind {
		case "x":
			in.Choices = []int{d.Options[len(d.Options)-1].Index} // the most it can pay for
		case "discard":
			in.Choices = b.chooseDiscard(d)
		case "exile":
			// Preserve the existing exile policy: this task adds sacrifice
			// choices, not a new policy for unrelated exile effects.
			in.Choices = b.chooseWorst(d)
		case "sacrifice":
			// Player-facing sacrifice asks use the same least-value choice as
			// the pre-existing mandatory give-up decisions. Optional asks still
			// take their first offered permanent, matching the no-host fallback.
			if d.Min > 0 {
				in.Choices = b.chooseWorst(d)
			} else {
				in.Choices = []int{d.Options[0].Index}
			}
		case "dig", "hand_move", "hidden_pick", "counter_dist", "counter_pick":
			// A Dig look-and-take, a "choose N matching cards from hand"
			// ChangeZone (handmove1), a Hidden$ True public-origin pick
			// (hiddenpick1), a DividedAsYouChoose$ PutCounter distribution
			// pick (Vastwood Hydra), or a bare-Choices$ PutCounter pick
			// (Promise of Loyalty's vow): take the first Max options in offered
			// (zone) order
			// -- the exact mirror of effDig's / effChangeZoneHand's /
			// effHiddenPick's / putCounterPickDistribute's no-ask stand-in (R-9),
			// so a bot-answered ask emits
			// the same MoveZone events the silent build did and no golden game
			// moves for the ask alone. An Optional$ Min-0 ask still takes the
			// full Max: the stand-in it mirrors plays "you may" as "do",
			// deterministically.
			for j := 0; j < len(d.Options) && j < d.Max; j++ {
				in.Choices = append(in.Choices, d.Options[j].Index)
			}
		case "pay_life", "pay_W", "pay_U", "pay_B", "pay_R", "pay_G":
			// A mana pip's payment alternatives (manaAsk): hybrid colours plus
			// a phyrexian pip's "Pay 2 life". The first offer is a POOL colour,
			// and taking it can strand the cost's generic remainder -- measured
			// (commander bench, seed 1295, Solphim's {1}{R/P}{R/P} ability):
			// the bot paid the only R into a pip, the generic {1} went unpaid,
			// and the activation aborted with no progress -- every window,
			// forever, until the intent cap fired. Pool mana is the scarcer
			// resource (it is the ONLY thing that can pay a generic pip; life
			// pays only these alternatives), so prefer the life payment when
			// it is offered and the seat has life to spare; otherwise take the
			// first offer (the old behaviour). The 6 threshold leaves two pips'
			// worth of buffer, so a two-pip cost never lands the seat at 0.
			in.Choices = []int{d.Options[0].Index}
			for _, o := range d.Options {
				if o.Kind == "pay_life" && b.Life[d.Player] >= 6 {
					in.Choices = []int{o.Index}
					break
				}
			}
		case "search":
			// A hidden-library search whose options carry no Group keeps the
			// first-offer answer it has always taken (Min 0, so one card). An
			// EACH "EACH Forest & Plains" search (each1) builds one option per
			// eligible card with the type's ordinal in Group -- at most one per
			// Group may be selected -- so the answer takes the FIRST option of
			// each new Group up to Max: one card per listed type, in the
			// decision's deterministic option order (types in spec order,
			// library order within a type). A DifferentNames search's
			// name-Groups get the same shape, which is what the constraint
			// itself asks for (distinct names, filled to Max). Both are legal
			// under Decision.Validate by construction; clamp's group skip is
			// the backstop, never the path.
			if d.Options[0].Group == "" {
				in.Choices = []int{d.Options[0].Index}
				break
			}
			groups := make(map[string]bool)
			for _, o := range d.Options {
				if len(in.Choices) >= d.Max {
					break
				}
				if o.Group == "" || groups[o.Group] {
					continue
				}
				groups[o.Group] = true
				in.Choices = append(in.Choices, o.Index)
			}
		default: // yes/no (yes is first), name, type, number: the first offer
			in.Choices = []int{d.Options[0].Index}
		}
		return clamp(d, in)

	case decision.KMulligan:
		// The London round, two shapes on one kind (rules/mulligan.go).
		// Bottoming is a hand-retention decision, like discard.
		if len(d.Options) > 0 && d.Options[0].Kind == "bottom" {
			in.Choices = b.chooseDiscard(d)
			return clamp(d, in)
		}
		// Keep/mulligan: mulligan with probability 1/3 when one is offered
		// (consuming the bot rng only where a real choice exists, the
		// determinism mirror of KTriggerOptional), else keep.
		if len(d.Options) > 1 {
			for _, o := range d.Options {
				if o.Kind == "mulligan" && r.IntN(3) == 0 {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}
		if len(d.Options) > 0 {
			in.Choices = []int{d.Options[0].Index} // keep
			return clamp(d, in)
		}

	case decision.KModes:
		// The Sacrifice unless-pay damage offer (Vexing Devil: "any opponent
		// may have it deal 4 damage to them") gets a deliberate arm, not the
		// first-option clamp: an opponent offered "take N to kill it"
		// decides on the offered permanent's worth against the life the
		// damage costs.
		if c := b.unlessSacrificeOffer(d); c != nil {
			in.Choices = c
			return clamp(d, in)
		}
		// A modal announcement or mid-resolution pick: choose the first Min options
		// in order — the recorded mirror of the engine-side first-mode
		// stand-in, so bot-vs-bot behaviour is largely unchanged, and the
		// answer stays seed-deterministic. This also answers an UnlessCost$
		// may-pay, shaped as the same KModes kind: option 0 is "Pay … — make
		// a copy", so the bot always offers to pay and the engine declines
		// for it only when the payer's pool cannot cover the cost. No rng is
		// consumed: the first modes are a fixed policy, not a coin.
		for j := 0; j < len(d.Options) && j < d.Min; j++ {
			in.Choices = append(in.Choices, d.Options[j].Index)
		}
		return clamp(d, in)
	}

	// Last resort: pass if Min == 0 and one is offered; clamp below handles
	// everything else, including topping up to Min when nothing above (or
	// this) picked enough.
	if d.Min == 0 {
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return clamp(d, in)
			}
		}
	}
	return clamp(d, in)
}

// unlessSacrificeOffer answers the Sacrifice unless-pay damage offer —
// Vexing Devil's "any opponent may have it deal N damage to them; if a
// player does, sacrifice it", posed as a KModes ask whose option 0 is
// "Take N damage" and option 1 "Refuse — it stays". The arm is deliberate,
// not the clamp fallback:
//
//   - accept when the offered permanent is worth more alive than the life
//     the damage costs (cardWorth >= n — a creature prices at 30+4×power,
//     so a real creature always clears a small N) AND the damage is not
//     lethal (n < the deciding seat's life) — taking four to kill a 4/3 is
//     normally correct; taking lethal damage to kill anything is not;
//   - decline otherwise (a worthless or unreadable permanent, or lethal
//     damage).
//
// It returns nil — leaving the ordinary KModes arm in charge — for every
// other KModes ask, including Sacrifice's mana unless-pay ("pay {1} or
// sacrifice it"), where the first-option arm's "pay" answer is right and the
// engine declines it only when the pool cannot cover the cost.
func (b Board) unlessSacrificeOffer(d *decision.Decision) []int {
	if d.ResumeKind != "unless_pay" || d.ResumeSA == nil || d.ResumeSA.API != "Sacrifice" {
		return nil
	}
	n, dmg := effects.ParseDamageUnlessCost(d.ResumeSA.Params["UnlessCost"])
	if !dmg || len(d.Options) == 0 {
		return nil
	}
	if b.cardWorth(d.Options[0].Obj) >= int32(n) && b.Life[d.Player] > int32(n) {
		return []int{d.Options[0].Index}
	}
	for _, o := range d.Options {
		if o.Index != d.Options[0].Index {
			return []int{o.Index}
		}
	}
	return []int{d.Options[0].Index}
}

// clamp enforces [Min, Max] on top of whatever Decide's switch (or its
// fallback) picked (Ruling T25-c): truncate to at most Max, in the order
// already chosen, then -- if that leaves fewer than Min -- top up with the
// lowest-index unused options until Min is reached or none remain. This is
// the last thing every return in Decide does, so Decision.Validate's
// Min..Max requirement holds for any shape the wire format allows, not only
// the ones reachable today.
//
// Ruling T25-g (fix round 2): the top-up prefers an unused "pass" option
// over anything else. The KPriority branch above already returns "pass"
// explicitly before clamp ever runs, so this is defense in depth for
// whatever reaches here anyway (an absent "pass", or a future caller) --
// without it, a blind first-unused-index top-up would reach for whatever
// legalActions (rules/legal.go) happens to list first, which is every
// "activate" option, before "pass". That is precisely how fix round 1's own
// clamp reintroduced I-1(b): a Min:1 priority decision falling through with
// nothing chosen got topped up into an activation instead of a pass.
func clamp(d *decision.Decision, in decision.Intent) decision.Intent {
	max := d.Max
	if max < 0 {
		max = 0
	}
	if len(in.Choices) > max {
		in.Choices = append([]int(nil), in.Choices[:max]...)
	}
	min := d.Min
	if min < 0 {
		min = 0
	}
	if len(in.Choices) < min {
		have := make(map[int]bool, len(in.Choices)) // membership only -- never ranged.
		groups := make(map[string]bool)             // an option Group already represented.
		for _, c := range in.Choices {
			have[c] = true
			if c >= 0 && c < len(d.Options) && d.Options[c].Group != "" {
				groups[d.Options[c].Group] = true
			}
		}
		for _, o := range d.Options {
			if len(in.Choices) >= min {
				break
			}
			if o.Kind == "pass" && !have[o.Index] {
				have[o.Index] = true
				in.Choices = append(in.Choices, o.Index)
			}
		}
		for _, o := range d.Options {
			if len(in.Choices) >= min {
				break
			}
			if have[o.Index] {
				continue
			}
			// Two options sharing one non-empty Group are mutually exclusive
			// (Decision.Validate's general rule): topping up with a second
			// same-group option would hand back an intent Validate rejects --
			// an answer the engine cannot accept and clamp cannot repair, so
			// the group is skipped the way a duplicate index is.
			if o.Group != "" && groups[o.Group] {
				continue
			}
			have[o.Index] = true
			if o.Group != "" {
				groups[o.Group] = true
			}
			in.Choices = append(in.Choices, o.Index)
		}
	}
	return in
}
