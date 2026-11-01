package heuristic

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// Bot is a deterministic policy with its own RNG, independent of the
// engine's (engine/rules/rng.go), so a match is reproducible from (engine seed, bot
// seed). It picks from the options the engine offered and nothing else,
// which is the same contract a human client has (Ruling P8).
//
// The policy itself lives in botpolicy (bots/botpolicy/policy.go), shared with
// the rules package's fuzz testBot (engine/rules/testbot_test.go): rules cannot
// import seat without running the dependency order backwards (Ruling F7),
// so both sides build a botpolicy.Board from what they can see and call the
// same botpolicy.Decide. This file is the view-shaped half of that -- it
// converts the projected View into the Board and seeds the bot's own rng.
// There is no second copy of the policy to keep in step; engine/seat/integration_test.go's
// TestBotAdaptersAgree* pins the two halves to the same Board for the same
// game facts.
type Bot struct {
	r              *rand.Rand
	lethalPressure bool
	combinedLethal bool
	blocksAssign   bool
	explore        bool
	// autoPayMana makes priority use an offered payment plan instead of
	// manually activating mana abilities. It is configured once by the hosted
	// table and does not add state or RNG to the match.
	autoPayMana bool
	// targets selects the opt-in A2 targeting rules (botpolicy.TargetRules).
	// Configured once by an opt-in constructor; the zero value leaves the
	// default ranking untouched, so every hosted policy is unchanged.
	targets botpolicy.TargetRules
	// removal selects the opt-in A1 removal-now rule (botpolicy.RemovalRules).
	// Configured once by an opt-in constructor; the zero value leaves every
	// cast decision unchanged.
	removal botpolicy.RemovalRules
	// posture selects the opt-in A4 deck-shape posture rules
	// (botpolicy.PostureRules). Configured once by an opt-in constructor;
	// the zero value leaves every combat and cast decision unchanged.
	posture botpolicy.PostureRules
	// timing selects the opt-in A6 "is now a good time" rules
	// (botpolicy.TimingRules). Configured once by an opt-in constructor; the
	// zero value leaves every cast and tap decision unchanged.
	timing botpolicy.TimingRules
	// aiHints selects the opt-in A3 Forge AI-hint rules
	// (botpolicy.AIHints). Configured once by an opt-in constructor; the zero
	// value leaves the cast scorer unchanged.
	aiHints botpolicy.AIHints
	// attach selects the A5 attach rules (botpolicy.AttachRules). NewBotOn
	// arms AuraPolarity so a boon Aura never lands on the opponent's
	// creature; Carrier and HoldOwnAura stay opt-in.
	attach botpolicy.AttachRules
	// skipLifePlans withholds every offered payment plan with a step that
	// pays life (spec §6: a caretaker standing in for a human never
	// auto-selects a life-paying plan -- a human confirms life payments in
	// the client). Set only for a hosted human seat's timeout caretaker; a
	// hosted bot or tool seat that submits such a plan has consented.
	skipLifePlans bool
	// cast/castSet are the cast-profile policy's weights: when castSet is
	// true every decision's Board gets brd.Cast = cast before the policy
	// runs, so the cast scorer (cardWorth/castScore/chooseCast) dots its
	// features with the profile instead of the default. Set once at
	// construction from a parsed profile; the Board refill (BoardFromGame /
	// BoardFromView) never touches Board.Cast, so the profile survives the
	// reuse contract untouched. With the embedded default profile (whose
	// weights equal DefaultCastWeights, pinned in bots/botpolicy/profile_test.go)
	// the decisions are identical to NewBot's by the L1 equivalence table.
	cast    botpolicy.CastWeights
	castSet bool
	// attackSim, when non-nil, answers KAttackers with the opt-in combat
	// simulation (botpolicy.AttackSimDecide); set only by NewAttackSimBot.
	attackSim *botpolicy.AttackSimParams
	// mulligan is the London-mulligan rule (botpolicy.MulliganRule), copied
	// onto every decision's Board like cast. The zero value is the default
	// 1/3 coin; WithMulligan / EnableLandMulligan set another.
	mulligan botpolicy.MulliganRule

	// paymentIntent scratch (POC: heap-object attack). Reused across calls
	// instead of the per-call d.Clone() + three maps + Options slice. A Bot
	// is used by one goroutine at a time; nothing here escapes the call.
	payPlans map[state.ObjID]decision.PaymentAction
	payTo    map[int]int
	payLeg   map[state.ObjID]bool
	payOpts  []decision.Option
	payCand  decision.Decision
}

// M4: a compile-time assertion that Bot keeps satisfying Seat, since
// seat.go and bot.go otherwise never reference each other.
var _ seat.Seat = (*Bot)(nil)

// M4: Bot also satisfies BoardSeat — the game-shaped half of the adapter
// pair, answered without a projected View. host builds the botpolicy.Board
// (via BoardFromGame, the same Board BoardFromView would lift off the View)
// under the match's exclusive lock and calls this instead of Decide, so a
// bot seat never forces cardViews' string round-trip.
var _ BoardSeat = (*Bot)(nil)

// NewBot seeds the bot's own PCG source. Never math/rand's global functions
// and never the engine's rng: a match's outcome must be a pure function of
// (engine seed, bot seed), nothing else.
func NewBot(seed uint64) *Bot {
	return NewBotOn(BotSource(seed))
}

// BotSource is the PCG source NewBot(seed) draws from.
func BotSource(seed uint64) *rand.PCG {
	return rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)
}

// NewBotOn is NewBot over a caller-held source, so a caller that must save
// and restore the bot's stream mid-game (bots/azmcts's node cache) keeps
// it: the source is the bot's only state. NewBotOn(BotSource(seed)) is
// NewBot(seed).
func NewBotOn(src *rand.PCG) *Bot {
	return &Bot{r: rand.New(src), attach: botpolicy.AttachRules{AuraPolarity: true}}
}

// EnableTargetRules arms the A2 target-selection rules (botpolicy.TargetRules)
// on this bot and returns it, so an opt-in constructor chains like the other
// Enable* methods. The default bot never calls it; the forge-heuristic policy
// (bots/forgeheuristic) is the registered constructor that does.
func (b *Bot) EnableTargetRules(r botpolicy.TargetRules) *Bot {
	b.targets = r
	return b
}

// TargetRules reports the A2 target-selection rules armed on this bot. It is
// the read side of EnableTargetRules, so a registry test can assert the
// forge-heuristic constructor actually armed the rules it advertises.
func (b *Bot) TargetRules() botpolicy.TargetRules {
	return b.targets
}

// EnableRemovalRules arms the A1 removal-now rule (botpolicy.RemovalRules)
// on this bot and returns it, so an opt-in constructor chains like the other
// Enable* methods. The default bot never calls it.
func (b *Bot) EnableRemovalRules(r botpolicy.RemovalRules) *Bot {
	b.removal = r
	return b
}

// RemovalRules reports the A1 removal-now rule armed on this bot.
func (b *Bot) RemovalRules() botpolicy.RemovalRules {
	return b.removal
}

// EnablePostureRules arms the A4 deck-shape posture rules
// (botpolicy.PostureRules) on this bot and returns it, so an opt-in
// constructor chains like the other Enable* methods. The default bot never
// calls it.
func (b *Bot) EnablePostureRules(r botpolicy.PostureRules) *Bot {
	b.posture = r
	return b
}

// PostureRules reports the A4 posture rules armed on this bot.
func (b *Bot) PostureRules() botpolicy.PostureRules {
	return b.posture
}

// EnableTimingRules arms the A6 "is now a good time" rules
// (botpolicy.TimingRules) on this bot and returns it, so an opt-in
// constructor chains like the other Enable* methods. The default bot never
// calls it.
func (b *Bot) EnableTimingRules(r botpolicy.TimingRules) *Bot {
	b.timing = r
	return b
}

// TimingRules reports the A6 timing rules armed on this bot.
func (b *Bot) TimingRules() botpolicy.TimingRules {
	return b.timing
}

// EnableAIHints arms the A3 Forge AI-hint rules (botpolicy.AIHints) on this
// bot and returns it, replacing the default (AuraPolarity only). The
// forge-heuristic policy is the registered constructor that also arms Carrier
// and HoldOwnAura.
func (b *Bot) EnableAIHints(h botpolicy.AIHints) *Bot {
	b.aiHints = h
	return b
}

// AIHints reports the A3 AI-hint rules armed on this bot.
func (b *Bot) AIHints() botpolicy.AIHints {
	return b.aiHints
}

// EnableAttachRules arms the A5 attach rules (botpolicy.AttachRules) on this
// bot and returns it, replacing the default (AuraPolarity only). The
// forge-heuristic policy is the registered constructor that also arms Carrier
// and HoldOwnAura.
func (b *Bot) EnableAttachRules(r botpolicy.AttachRules) *Bot {
	b.attach = r
	return b
}

// AttachRules reports the A5 attach rules armed on this bot.
func (b *Bot) AttachRules() botpolicy.AttachRules {
	return b.attach
}

// EnableAutoPayMana configures this bot to select payment-plan witnesses at
// priority. It returns b so every hosted policy constructor can retain its
// existing deterministic policy shape and seed derivation.
func (b *Bot) EnableAutoPayMana() *Bot {
	b.autoPayMana = true
	return b
}

// WithMulligan configures the bot's London-mulligan rule (botpolicy.
// MulliganRule): MulliganLands keeps or mulligans on the hand's land count
// and bottoms toward a sensible land count, MulliganNever always keeps, and
// MulliganCoin (the zero value) is the default 1/3 coin. A rule other than
// the coin consumes no rng at the mulligan, so the bot's later draws shift
// relative to the default bot's. It returns b, like EnableAutoPayMana.
func (b *Bot) WithMulligan(rule botpolicy.MulliganRule) *Bot {
	b.mulligan = rule
	return b
}

// EnableLandMulligan is WithMulligan(botpolicy.MulliganLands): the opt-in
// Limited land-count mulligan heuristic.
func (b *Bot) EnableLandMulligan() *Bot { return b.WithMulligan(botpolicy.MulliganLands) }

// MulliganRule reports the London-mulligan rule armed on this bot.
func (b *Bot) MulliganRule() botpolicy.MulliganRule { return b.mulligan }

// SkipLifePlans makes the bot treat an offered payment action whose plan pays
// life as having no plan, so its policy sees the ordinary options for that
// cast. Only the host's caretaker for a human seat sets it. It returns b.
func (b *Bot) SkipLifePlans() *Bot {
	b.skipLifePlans = true
	return b
}

// paymentPlanPaysLife reports whether any step of plan pays life.
func paymentPlanPaysLife(plan decision.PaymentPlan) bool {
	for _, a := range plan.Activations {
		if a.Consequence != nil && a.Consequence.Life > 0 {
			return true
		}
	}
	return false
}

// WantsPaymentActions reports the bot's explicit opt-in to planned payment.
func (b *Bot) WantsPaymentActions() bool { return b.autoPayMana }

// NewLethalPressureBot returns the measured opt-in AR7 policy. Both Seat
// adapters use the same variant, preserving the Board/View parity contract.
func NewLethalPressureBot(seed uint64) *Bot {
	return &Bot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), lethalPressure: true}
}

// NewCombinedLethalBot returns the opt-in AR8 bench policy: the AR7
// per-attacker lethal test plus the combined-attacker subset search
// (botpolicy.CombinedLethalDecide). It is constructed only by a bench runner's
// "ar8" policy entry -- it is deliberately absent from the hosted policy
// vocabulary (host.NormalizeBotPolicy), so it can never reach a live table.
func NewCombinedLethalBot(seed uint64) *Bot {
	return &Bot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), lethalPressure: true, combinedLethal: true}
}

// NewBlocksBot returns the opt-in BLK bench policy: the default policy with
// KBlockers answered by the whole-assignment heuristic (botpolicy.
// BlocksDecide). It is constructed only by a bench runner's "blocks" policy
// entry -- it is deliberately absent from the hosted policy vocabulary
// (host.NormalizeBotPolicy), so it can never reach a live table.
func NewBlocksBot(seed uint64) *Bot {
	return &Bot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), lethalPressure: true, blocksAssign: true}
}

// NewExploreBot returns the opt-in coverage-exploration policy
// (botpolicy.ExploreDecide). It is constructed only by cmd/cardfuzz -- it is
// deliberately absent from the hosted policy vocabulary
// (host.NormalizeBotPolicy), so it can never reach a live table.
func NewExploreBot(seed uint64) *Bot {
	return &Bot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), lethalPressure: true, explore: true}
}

// NewAttackSimBot returns the opt-in combat-simulation attacker
// (botpolicy.AttackSimDecide): the default policy (AR7 lethal pressure
// included) with KAttackers answered by a whole-attacking-set search scored
// by a static evaluator. Same PCG derivation as NewBot, and the attacker
// consumes no rng, so every other decision draws exactly the default bot's
// stream. With auto-pay enabled it is the hosted default auto-pay bot
// (host.NewBotPolicySeatWithAutoPayMana); a bench runner also builds it
// without auto-pay as the "attack-sim" arm.
func NewAttackSimBot(seed uint64, p botpolicy.AttackSimParams) *Bot {
	return &Bot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), lethalPressure: true, attackSim: &p}
}

// CombatSim reports whether the bot answers attacks with the combat
// simulation (NewAttackSimBot).
func (b *Bot) CombatSim() bool { return b.attackSim != nil }

// AttackSimParams returns the combat-sim parameters the bot was built with and
// true, or the zero value and false when it has no combat sim.
func (b *Bot) AttackSimParams() (botpolicy.AttackSimParams, bool) {
	if b.attackSim == nil {
		return botpolicy.AttackSimParams{}, false
	}
	return *b.attackSim, true
}

// NewCastProfileBot returns the cast-profile policy playing the named
// embedded profile (today: the default one). The only error is an embedded
// profile that fails its own strict loader -- never reachable for a valid
// committed file (pinned by botpolicy's profile tests), surfaced as an error
// rather than a panic so the host factory can report it the same way it
// reports an unknown policy name.
func NewCastProfileBot(seed uint64) (*Bot, error) {
	w, err := botpolicy.LoadCastProfile(botpolicy.DefaultCastProfileName)
	if err != nil {
		return nil, fmt.Errorf("seat: %w", err)
	}
	return NewCastProfileBotWithWeights(seed, w), nil
}

// NewCastProfileBotWithWeights returns the cast-profile policy playing the
// given weights -- the shape botbench's -profile flag builds after parsing a
// candidate file, so a profile is benched without a rebuild. Same PCG
// derivation as NewBot, so a profile's RNG consumption matches the
// production bot's exactly.
func NewCastProfileBotWithWeights(seed uint64, w botpolicy.CastWeights) *Bot {
	return &Bot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), cast: w, castSet: true}
}

func (b *Bot) decide(brd botpolicy.Board, d *decision.Decision) decision.Intent {
	if b.castSet {
		brd.Cast = b.cast
	}
	if b.mulligan != botpolicy.MulliganCoin {
		brd.Mulligan = b.mulligan
	}
	// The opt-in A2 rules travel on the Board like cast and mulligan, so they
	// are applied HERE rather than in an entry point: a caller builds the
	// Board and cannot know this bot's policy configuration, and a per-entry
	// assignment is one forgotten line away from silently un-arming the
	// rules. The zero value leaves brd unchanged, so the default bot is
	// unaffected.
	brd.Targets = b.targets
	brd.Removal = b.removal
	brd.Posture = b.posture
	brd.Timing = b.timing
	brd.AIHints = b.aiHints
	brd.Attach = b.attach
	if b.autoPayMana {
		if in, ok := b.paymentIntent(brd, d); ok {
			return in
		}
	}
	return b.decideWithoutAutoPay(brd, d)
}

// paymentIntent lets the ordinary casting policy rank offered payment-plan
// casts without ever manually floating mana. It constructs a private priority
// decision in which mana activations are absent, every legacy option is kept
// at its own candidate index, and each payable object whose ordinary cast is
// not already offered as a legacy option gains an additional plan-only cast
// candidate. Thus the existing policy still takes a land drop first, keeps
// all non-payment decisions unchanged, and selects the same preferred card
// among payable spells. The pick is mapped back by candidate option
// identity: a legacy option (an evoke, pitch, dash, surge or other
// alternative mode the policy deliberately chose) is submitted as itself, so
// the chosen mode -- not an ordinary plan -- reaches the engine; only a pick
// of the plan-only entry, or of a legacy ordinary cast (Mode == "" &&
// AltCostIndex == 0), pays the plan. The submitted witness is copied from the
// exact offered plan.
func (b *Bot) paymentIntent(brd botpolicy.Board, d *decision.Decision) (decision.Intent, bool) {
	if d == nil || d.Kind != decision.KPriority || (brd.MyTurn && !brd.IsMain) {
		return decision.Intent{}, false
	}
	payable := b.payPlans
	if payable == nil {
		payable = make(map[state.ObjID]decision.PaymentAction, len(d.PaymentActions))
		b.payPlans = payable
	} else {
		clear(payable)
	}
	for _, a := range d.PaymentActions {
		// A plan the policy would never take must not count as payable: C8
		// refuses a counter with no foreign spell (CounterIsDead), so its
		// plan is dead -- leaving it in `payable` would let the private
		// candidate lose to pass and hide the manual path (an instant the
		// bot wanted, castable only by hand) for the rest of the window.
		// Drop it here so a window whose ONLY plan is a dead counter is
		// treated as a no-plan window (payable empty) and the manual path is
		// reconsidered.
		//
		// A caretaker never auto-selects a life-paying plan (SkipLifePlans):
		// such an action is treated as having no plan.
		if len(a.Plans) != 0 && !brd.CounterIsDead(d.Player, a.Cast.Object) &&
			!(b.skipLifePlans && paymentPlanPaysLife(a.Plans[0])) {
			if _, dup := payable[a.Cast.Object]; !dup {
				payable[a.Cast.Object] = a
			}
		}
	}
	if b.wantsManual(brd, d, payable) {
		// The tap gate has a real cast to make now and the plan path will
		// strand it: take the full manual policy, whose same tap gate aims the
		// window's mana at that card, instead of spending it on a lesser
		// planned spell. Once the manual taps float mana the engine withholds
		// plans for the rest of the window (paymentPlanPoolOK), so the turn
		// cannot oscillate back onto the plan path mid-sequence.
		//
		// This is the single gate for BOTH the plan-bearing window and the
		// no-plan window (len(payable)==0): when no cast is plausibly castable
		// now, neither takes the manual answer. The candidate policy below
		// then answers with land drop / ability / legacy cast / pass and no
		// activations, so the window never floats mana toward a card it cannot
		// cast.
		return decision.Intent{}, false
	}

	candidate := &b.payCand
	*candidate = *d
	candidate.Options = b.payOpts[:0]
	candidateToOriginal := b.payTo
	if candidateToOriginal == nil {
		candidateToOriginal = make(map[int]int, len(d.Options))
		b.payTo = candidateToOriginal
	} else {
		clear(candidateToOriginal)
	}
	legacyOrdinary := b.payLeg
	if legacyOrdinary == nil {
		legacyOrdinary = make(map[state.ObjID]bool, len(d.Options))
		b.payLeg = legacyOrdinary
	} else {
		clear(legacyOrdinary)
	}
	for _, o := range d.Options {
		// These are precisely legalActions' mana abilities. A payment plan
		// performs the required activations atomically, so exposing one here
		// would defeat auto-pay before cast ranking runs.
		if o.Kind == "activate" {
			continue
		}
		originalIndex := o.Index
		o.Index = len(candidate.Options)
		candidateToOriginal[o.Index] = originalIndex
		if planShaped(o) {
			// An ordinary legacy cast whose object has a plan is a
			// plan-backed candidate: choosing it submits the plan, so the
			// cast scorer prices it against producible mana (C7). A
			// non-ordinary mode (an evoke, pitch, dash, surge) pays its
			// own cost by hand and stays false. A primary flashback cast
			// is plan-backed only by a flashback-origin action
			// (decision.OriginFlashback), and only then stands in for the
			// action's plan-only entry.
			a, ok := payable[o.Obj]
			matched := ok && planModeOf(a) == o.Mode
			if o.Mode == "" || matched {
				legacyOrdinary[o.Obj] = true
			}
			if matched {
				o.PlanBacked = true
			}
		}
		candidate.Options = append(candidate.Options, o)
	}
	for _, a := range d.PaymentActions {
		if len(a.Plans) == 0 || legacyOrdinary[a.Cast.Object] {
			continue
		}
		// The plan-only entry carries its cast's mode (planModeOf), so a
		// flashback plan is ranked as the flashback cast it is.
		candidate.Options = append(candidate.Options, decision.Option{
			Index: len(candidate.Options), Kind: "cast", Obj: a.Cast.Object, Label: a.Label,
			Mode: planModeOf(a), PlanBacked: true,
		})
	}

	// Keep the (possibly regrown) Options backing array for the next call.
	b.payOpts = candidate.Options[:0]

	// Use the exact policy variant (including a cast profile) on the private
	// candidate list. Priority choices consume no RNG for hosted policies, so
	// this neither changes subsequent choices nor asks the policy twice.
	in := b.decideWithoutAutoPay(brd, candidate)
	if len(in.Choices) != 1 {
		return decision.Intent{}, false
	}
	for _, o := range candidate.Options {
		if o.Index != in.Choices[0] {
			continue
		}
		// Only an ordinary-shaped cast pick pays the plan -- the plan-only
		// entry, or a legacy ordinary cast whose object has a plan. A legacy
		// non-ordinary pick (an evoke, pitch, dash, surge or other
		// alternative mode the policy deliberately chose) reaches the engine
		// as itself: substituting the ordinary plan would silently replace
		// the chosen mode.
		if planShaped(o) {
			if a, ok := payable[o.Obj]; ok && planModeOf(a) == o.Mode {
				return decision.Intent{Seq: d.Seq, Player: d.Player, Payment: &decision.PaymentSelection{
					ActionID: a.ID, Plan: decision.ClonePaymentPlan(a.Plans[0]),
				}}, true
			}
		}
		break
	}
	// The private policy may have preferred a land drop, an ability, a
	// legacy cast mode, or pass. Translate that choice back to the original
	// option index so removing mana activations never changes the decision's
	// public index contract.
	if original, ok := candidateToOriginal[in.Choices[0]]; ok {
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{original}}, true
	}
	return decision.Intent{}, false
}

// planShaped reports whether a cast option is one a payment plan can stand
// for: the ordinary cast (no Mode, no alternative cost), or the primary
// flashback cast (Mode "flashback", no further-instance Cost), whose plan is
// a decision.OriginFlashback action. Which plan, if any, is planModeOf's
// match on the option's Mode.
func planShaped(o decision.Option) bool {
	if o.Kind != "cast" || o.AltCostIndex != 0 {
		return false
	}
	return o.Mode == "" || (o.Mode == "flashback" && o.Cost == "" && o.MayPlayPerm == "")
}

// planModeOf is the cast-option Mode a payment action's plan pays for as the
// bot's candidate list names it: "flashback" for a flashback-origin action,
// "" for every other (the hand and command-zone plain casts, and the may-play
// origins, whose plan-only entry has always been ranked as an ordinary cast).
func planModeOf(a decision.PaymentAction) string {
	if a.Cast.Origin == decision.OriginFlashback {
		return "flashback"
	}
	return ""
}

// wantsManual reports whether the auto-pay adapter should answer this
// priority window with the full manual policy (tap gate and all) rather than
// the plan path. The manual answer is only worth taking when the trap-gate
// intent is a cast the manual policy could actually make now; otherwise the
// window's mana would float toward a card it cannot cast and empty at step
// end, exactly the waste the auto-pay adapter exists to avoid.
//
// The test is branch-dependent because the two windows have different things
// to protect:
//
//   - no plan (len(payable)==0): there is no planned cast to defer to, so
//     ANY unpayable card that is castable now justifies the manual answer
//     (Board.AnyCastableNow). This is the class reading the dead-counter
//     shape needs: the tap gate's one best intent can be an uncostable or
//     C8-dead card while a DIFFERENT card in the hand is a real cast, and
//     looking only at the intent would drop that cast;
//   - plan-bearing (len(payable)>0): a planned cast is at stake, so only the
//     tap gate's own best intent justifies diverting to manual, and only
//     when it has NO plan (Board.CastableNow plus the planned filter). This
//     keeps the plan the policy will take from being abandoned for a lesser
//     unplanned card -- the same one-intent scoping the unplanned-bot intent
//     fallback always had.
func (b *Bot) wantsManual(brd botpolicy.Board, d *decision.Decision, payable map[state.ObjID]decision.PaymentAction) bool {
	if len(payable) == 0 {
		return brd.AnyCastableNow(d.Player, d)
	}
	id, ok := brd.CastableNow(d.Player, d)
	if !ok {
		return false
	}
	if _, planned := payable[id]; planned {
		return false
	}
	return true
}

// decideWithoutAutoPay is the existing policy dispatch, factored so the
// payment wrapper can ask it to rank a private candidate decision once.
func (b *Bot) decideWithoutAutoPay(brd botpolicy.Board, d *decision.Decision) decision.Intent {
	if b.attackSim != nil {
		return botpolicy.AttackSimDecide(brd, d, b.r, *b.attackSim)
	}
	if b.explore {
		return botpolicy.ExploreDecide(brd, d, b.r)
	}
	if b.combinedLethal {
		return botpolicy.CombinedLethalDecide(brd, d, b.r)
	}
	if b.blocksAssign {
		return botpolicy.BlocksDecide(brd, d, b.r)
	}
	if b.lethalPressure {
		return botpolicy.LethalPressureDecide(brd, d, b.r)
	}
	return botpolicy.Decide(brd, d, b.r)
}

// Decide answers d with the combat-aware policy in botpolicy. v is read
// for two things -- whether it is currently a main phase, and the public
// battlefield/life facts the combat heuristic reads (both halves of
// BoardFromView below) -- the reason Decide takes a View at all rather
// than acting on d alone. engine/rules/testbot_test.go's testBot has no View and
// gets the same facts from the engine (e.G.Step.IsMain, botpolicy.BoardFromGame)
// instead -- the game-shaped half of the same adapter pair;
// TestBotAdaptersAgree* (integration_test.go) pins the two halves to the
// same Board for the same game facts.
func (b *Bot) Decide(_ context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	brd := BoardFromView(v)
	return b.decide(brd, &d), nil
}

// DecideBoard is the game-shaped half of Decide: the Board is already built
// (under the match lock, from botpolicy.BoardFromGame reading state.Game and
// the engine's derived P/T/keywords) and handed in as a value, so the bot
// answers without the view->Board string round-trip. The engine the Board
// was built from derives exactly the facts the projected View would have
// carried (TestBotAdaptersAgreeOverWholeGame pins the two halves).
func (b *Bot) DecideBoard(_ context.Context, brd botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	return b.decide(brd, &d), nil
}

// BoardFromView is the view-shaped adapter: the Board the policy reads,
// lifted off the projected View a real client would receive. The combat
// half (Creatures, Life) is every public battlefield creature and life
// total the seat can see -- exactly the facts botpolicy.BoardFromGame
// derives from the engine's state.Game (same zones, same derived P/T and
// keywords, same filters: cardless ability objects and off-battlefield
// ephemerals are dropped by the View itself, and the creature test below
// mirrors engine/cards/face.go's hasType membership check on the joined type
// list). The rules test host computes the same Board from the engine;
// engine/seat/integration_test.go's TestBotAdaptersAgreeOverWholeGame pins the
// two halves to the same facts over a whole game.
func BoardFromView(v view.View) botpolicy.Board {
	b := botpolicy.Board{
		OwnDeck: v.OwnDeck,
		IsMain:  v.Phase == "main1" || v.Phase == "main2",
		// The cast scorer's two board-half features (bots/botpolicy/cast.go):
		// FirstMain is the FIRST main phase (the Precombat feature), MyTurn
		// whether the deciding seat is the active player (the
		// InstantOnOwnTurn feature's "own main phase" half — a main phase can
		// belong to another seat, so IsMain alone cannot say it). Same facts
		// the game half derives from g.Step == state.StepMain1 and
		// g.Active == me.
		FirstMain: v.Phase == "main1",
		MyTurn:    v.Active == v.Viewer,
		// The exact engine step (the cast scorer's timing features): the
		// projected View.Step is g.Step.String() (engine/view/view.go), so parsing
		// it back names the same state.Step the game half reads off g.Step
		// directly. An unrecognised string (never produced by the projector)
		// leaves the zero step, StepUntap, exactly the game half's zero.
		Step: parsedStep(v.Step),
	}
	// The honest own-library composition (Board.OwnLibrary): folded off
	// this View alone by view.OwnLibrary, the same per-card rules the game
	// half's fillOwnLibrary applies to state.Game.
	view.OwnLibrary(v, v.Viewer, &b.OwnLibrary)
	// The public stack census (C8's facts): the projected StackView list is
	// the stack's own bottom-to-top order, so the census slice is that same
	// order and every read of it is deterministic. IsSpell mirrors the
	// game-shaped half's o.Ability == nil test exactly: the View's Kind is
	// "spell" precisely for the objects whose Ability is nil (an ability
	// object projects as "trigger" or "ability", engine/view/view.go's
	// stackViews).
	b.Stack = make([]botpolicy.StackEntry, 0, len(v.Stack))
	for _, sv := range v.Stack {
		var cmc int32
		var manaCost string
		var pumpPower, pumpToughness int32
		// A trigger can include its source card for display, but the stack
		// ability itself has no face or mana cost. Only spells have a CMC
		// in the game-shaped adapter (BoardFromGameInto).
		if sv.Kind == "spell" && sv.Card != nil {
			cmc = botpolicy.CmcOf(sv.Card.ManaCost)
			manaCost = sv.Card.ManaCost
			pumpPower, pumpToughness = sv.Card.PumpPower, sv.Card.PumpToughness
		}
		b.Stack = append(b.Stack, botpolicy.StackEntry{ID: sv.ID, Controller: sv.Controller, IsSpell: sv.Kind == "spell", CMC: cmc, ManaCost: manaCost, PumpPower: pumpPower, PumpToughness: pumpToughness})
	}
	for _, p := range v.Players {
		b.Life.Set(p.ID, p.Life)
		if p.ID == v.Viewer {
			b.LibrarySize = int32(p.LibrarySize)
			b.HandSize = int32(p.HandSize)
			// The tap gate's pool (tap.go): the projecting viewer's own mana
			// pool, the same numbers poolView lifted off the engine state
			// that BoardFromGameInto reads directly, so the two halves agree
			// (pinned non-vacuously by integration_test.go's pool agreement).
			for sym, n := range p.Pool {
				b.Pool[state.ManaIndex(sym[0])] = n
			}
			// The restricted share of that pool (Board.PoolRestricted): the
			// View names exactly the batches with a spend limit, carrying the
			// producing counter verbatim, the game half's source too.
			for _, r := range p.PoolRestrictions {
				if r.Amount > 0 {
					b.PoolRestricted[state.ManaSlot(r.Color)] += r.Amount
				}
			}
		}
		for _, cv := range p.Battlefield {
			if !isCreatureView(cv) {
				continue
			}
			var defender state.PlayerID
			if cv.Attacking && cv.AttackingPlayer != nil {
				defender = *cv.AttackingPlayer
			}
			b.Creatures.Set(cv.ID, botpolicy.Creature{
				Power:      cv.Power,
				Toughness:  cv.Toughness,
				Damage:     cv.Damage,
				Keywords:   cv.Keywords,
				Tapped:     cv.Tapped,
				Controller: cv.Controller,
				Attacking:  cv.Attacking,
				Defender:   defender,
			})
		}
		// The commander bookkeeping: the projected roster (p.Commanders,
		// never shrunk as commanders are cast) carries identity and the CR
		// 903.8 cast counts; p.Command's zone-list membership is
		// InCommandZone's exact mirror of the game half's
		// g.Zone(ZCommand, owner) read; every player's CmdDamage re-keys
		// the CR 903.10 clock by commander object id exactly as the game
		// half's dense-index transpose does.
		for k, cv := range p.Commanders {
			var casts int32
			if k < len(p.CommanderCasts) {
				casts = p.CommanderCasts[k]
			}
			cmdr := botpolicy.Commander{Casts: casts}
			for _, cz := range p.Command {
				if cz.ID == cv.ID {
					cmdr.InCommandZone = true
					break
				}
			}
			b.Commanders.Set(cv.ID, cmdr)
		}
	}
	// The blocking second pass: the projected attackers carry their
	// BlockedBy list, so record on each blocked creature the lowest
	// attacker id blocking it, exactly as the game half's fillBlocks does.
	// Reading the View's own order (players, then battlefield) makes the
	// pass deterministic, and the lowest-id fold makes it independent of
	// the order the block lists were populated in.
	for _, p := range v.Players {
		for _, cv := range p.Battlefield {
			if !cv.Attacking {
				continue
			}
			for _, blocker := range cv.BlockedBy {
				cr, ok := b.Creatures.Lookup(blocker)
				if !ok {
					continue
				}
				if cr.Blocks == 0 || cv.ID < cr.Blocks {
					cr.Blocks = cv.ID
					b.Creatures.Set(blocker, cr)
				}
			}
		}
	}
	// The public-card colour census (A2): every player's battlefield,
	// graveyard and exile, then the stack -- the same canonical order
	// BoardFromGameInto's fillPublicCards walks. A face-down card carries
	// FaceDown and a ZERO colour, because the projection redacts its printed
	// identity (CR 708.5); the A2 fold must never read it.
	for _, p := range v.Players {
		for _, zone := range [][]view.CardView{p.Battlefield, p.Graveyard, p.Exile} {
			for _, cv := range zone {
				pc := botpolicy.PublicCard{Controller: cv.Controller, FaceDown: cv.FaceDown}
				if !cv.FaceDown {
					pc.Colour = cv.ColourIdentity
				}
				b.PublicCards.Set(cv.ID, pc)
			}
		}
	}
	for _, sv := range v.Stack {
		// Only a spell is a card. A trigger/ability StackView may carry its
		// source CardView for display, but the game half's fillPublicCards
		// skips the face-less ability object, so including it here would make
		// the two public-card censuses disagree.
		if sv.Kind != "spell" || sv.Card == nil {
			continue
		}
		pc := botpolicy.PublicCard{Controller: sv.Controller, FaceDown: sv.Card.FaceDown}
		if !sv.Card.FaceDown {
			pc.Colour = sv.Card.ColourIdentity
		}
		b.PublicCards.Set(sv.ID, pc)
	}
	// The CR 903.10 clock, filled from every player's damage keys. The map
	// iteration order is irrelevant: each entry lands in b.Commanders[id]'s
	// own Damage map keyed by the player who took it, and no policy branch
	// reads anything in order this fill could disturb (ties break on ObjID
	// or option index). A commander a damaged player names is always
	// already in b.Commanders (damage only ever accrues to a commander) —
	// the nil-map read would be a zero entry otherwise, never a crash.
	for _, q := range v.Players {
		for id, tally := range q.CmdDamage {
			cmdr := b.Commanders.Get(id)
			if cmdr.Damage == nil {
				cmdr.Damage = make(map[state.PlayerID]int32, 2)
			}
			cmdr.Damage[q.ID] = tally
			b.Commanders.Set(id, cmdr)
		}
	}
	// The casting Card census: the viewer's own hand, graveyard, battlefield
	// and command-zone CardViews — the deciding seat's own legally-seen
	// zones — mirroring exactly what BoardFromGame fills from state.Game for
	// that seat (cast.go's CmcOf and the type-word check on the same printed
	// fields, and the engine's derived Power that the View already projects
	// as cv.Power), so the casting policy ranks the same card the same way
	// on both halves. The fill is per zone, because the tap gate (tap.go)
	// reads Castable from the zone a card sits in: a hand or command-zone
	// card is worth mana (its cast is offered as soon as the pool pays the
	// cost; a command-zone card is a commander the CR 903.8 tax prices), a
	// graveyard card only when the View's derived keyword list carries
	// Flashback (the same Derived list the engine's own flashback gate
	// reads), and a battlefield permanent never.
	//
	// The public battlefield census (vote_card1): another seat's battlefield
	// permanents are public CR 400.2 facts projected for every viewer, and
	// they are exactly what a ballot offers a voter (VoteCard$'s Council's
	// Judgment filter names only permanents the caster does not control),
	// so the loop below fills their WORTH facts (Creature/Power/Toughness/
	// CMC/Basic/ManaCost, what cardWorth prices) for every player, not only
	// the viewer — botpolicy.BoardFromGameInto's opponent-battlefield walk
	// is this call's exact mirror. Activated (the public per-object
	// activation census) is filled too: A5's budget must see an "any
	// player may activate" source's activations (Lethal Vapors). The other
	// seat-relative facts stay ZERO on a
	// foreign entry (OnBattlefield/Produces/Tapped/Castable/
	// InstantSpeed/AttachedTo are the deciding seat's own-board facts; the
	// land-drop greedy's mana readers would otherwise count an opponent's
	// lands as the seat's own), so the foreign fill below is its own walk,
	// not a fillZone call. A facedown opponent permanent projects stripped
	// (no printed Types/Power), so its entry lands all-zero here — exactly
	// the zero-fact Card the game half writes for the same object.
	fillZone := func(zone []view.CardView, castable func(view.CardView) bool, battlefield bool) {
		for _, cv := range zone {
			var produces cards.ManaProduction
			if cv.Produces != nil {
				produces = *cv.Produces
			}
			printedName := cv.CardName
			if printedName == "" {
				printedName = cv.Name
			}
			b.Cards.Set(cv.ID, botpolicy.Card{
				PrintedName:    printedName,
				Creature:       isCreatureView(cv),
				Power:          cv.Power,
				Toughness:      cv.Toughness,
				CMC:            botpolicy.CmcOf(cv.ManaCost),
				Basic:          hasBasicView(cv),
				Land:           hasTypeView(cv, "Land"),
				Colour:         cv.ColourIdentity,
				TypeSig:        botpolicy.TypeSigOf(strings.Fields(cv.Types)),
				AttachedTo:     cv.AttachedTo,
				Activated:      cv.ActivatedThisTurn,
				ManaCost:       cv.ManaCost,
				Castable:       castable(cv),
				OnBattlefield:  battlefield,
				Tapped:         cv.Tapped,
				Sick:           cv.SummonSick,
				Produces:       produces,
				InstantSpeed:   instantSpeedView(cv),
				Counter:        cv.SpellAPI == "Counter",
				Draw:           cv.SpellAPI == "Draw",
				Token:          cv.SpellAPI == "Token",
				RemovalKind:    cv.RemovalKind,
				RemovalTargets: cv.RemovalTargets,
				RemovalDamage:  cv.RemovalDamage,
				PumpPower:      cv.PumpPower,
				PumpToughness:  cv.PumpToughness,
				PumpOwn:        cv.PumpOwn,
				CombatTrick:    cv.CombatTrick,
				PlayMain1:      cv.PlayMain1,
				RemoveDeck:     cv.RemoveDeck,
				AuraLogic:      cv.AuraLogic,
			})
		}
	}
	aCastable := func(view.CardView) bool { return true }
	notCastable := func(view.CardView) bool { return false }
	for _, p := range v.Players {
		if p.ID != v.Viewer {
			for _, cv := range p.Battlefield {
				printedName := ""
				if !cv.FaceDown {
					printedName = cv.CardName
					if printedName == "" {
						printedName = cv.Name
					}
				}
				b.Cards.Set(cv.ID, botpolicy.Card{
					PrintedName:    printedName,
					Creature:       isCreatureView(cv),
					Power:          cv.Power,
					Toughness:      cv.Toughness,
					CMC:            botpolicy.CmcOf(cv.ManaCost),
					Basic:          hasBasicView(cv),
					Land:           hasTypeView(cv, "Land"),
					Colour:         cv.ColourIdentity,
					TypeSig:        botpolicy.TypeSigOf(strings.Fields(cv.Types)),
					ManaCost:       cv.ManaCost,
					Activated:      cv.ActivatedThisTurn,
					RemovalKind:    cv.RemovalKind,
					RemovalTargets: cv.RemovalTargets,
					RemovalDamage:  cv.RemovalDamage,
					PlayMain1:      cv.PlayMain1,
					RemoveDeck:     cv.RemoveDeck,
					AuraLogic:      cv.AuraLogic,
				})
			}
			continue
		}
		fillZone(p.Hand, aCastable, false)
		fillZone(p.Graveyard, hasFlashbackView, false)
		fillZone(p.Battlefield, notCastable, true)
		fillZone(p.Command, aCastable, false)
	}
	return b
}

// hasFlashbackView is the view-shaped half of the tap gate's
// graveyard-castability test (botpolicy.combat.go's game-shaped half): the
// projected keyword list is the engine's Derived() output, so checking the
// card's keywords for "Flashback" (cards.KeywordHead-stripped, the same
// head test the game half runs) agrees with the engine's own flashback
// gate read.
func hasFlashbackView(cv view.CardView) bool {
	for _, k := range cv.Keywords {
		if strings.EqualFold(cards.KeywordHead(k), "Flashback") {
			return true
		}
	}
	return false
}

// hasTypeView reports whether the projected type line names t, mirroring
// cards.Face.TypeLineHas's EqualFold word match on the game half.
func hasTypeView(cv view.CardView, t string) bool {
	for w := range strings.FieldsSeq(cv.Types) {
		if strings.EqualFold(w, t) {
			return true
		}
	}
	return false
}

// isCreatureView is the view-shaped half of "is this battlefield object a
// creature": the game-shaped half reads the object's own Face().IsCreature
// (engine/cards/face.go's hasType, an EqualFold membership check), and the View
// carries only the same type words joined into one string, so this mirrors
// exactly that check on the joined list.
func isCreatureView(cv view.CardView) bool {
	for t := range strings.FieldsSeq(cv.Types) {
		if strings.EqualFold(t, "Creature") {
			return true
		}
	}
	return false
}

// hasBasicView is the view-shaped half of "is this a basic land": the
// game-shaped half reads the face's own type list (BoardFromGame's
// Card.Basic via hasTypeWord), and the View carries the same type words
// joined into one string, so this mirrors exactly that check on the
// joined list.
func hasBasicView(cv view.CardView) bool {
	for t := range strings.FieldsSeq(cv.Types) {
		if strings.EqualFold(t, "Basic") {
			return true
		}
	}
	return false
}

// instantSpeedView is the view-shaped half of "can this card be cast at
// instant speed" (botpolicy.Card.InstantSpeed): the card is an Instant, or
// it carries the Flash keyword. The game-shaped half (BoardFromGame) reads
// the face's own type list and the engine's derived keyword list, which the
// View carries as cv.Types and cv.Keywords, so the two halves agree.
func instantSpeedView(cv view.CardView) bool {
	for t := range strings.FieldsSeq(cv.Types) {
		if strings.EqualFold(t, "Instant") {
			return true
		}
	}
	for _, k := range cv.Keywords {
		if strings.EqualFold(cards.KeywordHead(k), "Flash") {
			return true
		}
	}
	return false
}

// parsedStep maps a projected step string back to the engine's state.Step.
// view.Project sets View.Step = g.Step.String(), so state.ParseStep is that
// producer's own inverse; an unknown name yields the zero step (StepUntap),
// the same value a Board the timing features never reach already carries.
func parsedStep(name string) state.Step {
	if s, ok := state.ParseStep(name); ok {
		return s
	}
	return 0
}
