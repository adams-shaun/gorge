package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// UnlessCost$ is Forge's "unless a player pays <cost>, <effect happens>"
// mid-resolution gate. Forge resolves it in AbilityUtils.handleUnlessCost:
// the UnlessPayer$ (default TargetedController) is offered the cost; the
// effect's main body resolves only when `alreadyPaid == isSwitched`, i.e.
//
//   - UnlessSwitched$ absent (the default): paying PREVENTS the effect —
//     "counter target spell unless its controller pays {1}" (Mana Leak),
//     "enters tapped unless you pay 2 life" (Hallowed Fountain), "target
//     player sacrifices a creature unless they pay {3}".
//   - UnlessSwitched$ True: paying CAUSES the effect — "may pay {2}. If
//     that player does, copy that spell" (Chain Lightning's copy clause,
//     whose script carries the flag explicitly).
//
// SubAbility$ chains resolve whether the cost was paid or not (Forge's
// UnlessResolveSubs$ default 'Always'), which is why this gate decides only
// whether the effect's own body runs: Resolve continues down sa.Sub either
// way.
//
// Before this gate existed only Counter and CopySpellAbility read the
// parameter; every other API (Sacrifice 155 raw corpus lines, Tap 56, Draw
// 43, LoseLife 42, ChangeZone 28, ...) ignored an UnlessCost$ entirely. The
// gate below runs in Resolve before EVERY effect dispatch, so one
// implementation now serves every API. Counter and CopySpellAbility keep
// only their label wording; their semantics are exactly these two
// orientations and always were.
//
// Payment happens in rules (rules' resumeResolution unless_pay arm): the ask
// is a KModes decision with ResumeKind "unless_pay", the answer re-enters
// this SA with Ctx.UnlessPay set ("pay" = the cost was charged to the payer
// by the engine's payment path, "decline" = it was not), and this gate
// applies the orientation. A cost the payment API cannot price is a hard
// decline there, never a free pass. The payer index rides the decision's
// ResumeTarget (Ask stores it on the resume point), so a multi-payer
// UnlessPayer$ asks each payer in turn until one pays.
//
// UnlessResolveSubs$ WhenPaid/WhenNotPaid (40 raw corpus lines) is not read:
// the default 'Always' behaviour — subs run regardless — is what every
// other UnlessCost$ line gets, and is what this build does.

// unlessProceed reports whether the effect's body should run for this pass.
// Called from Resolve immediately before the dispatch, for every SA; a
// zero-cost SA returns true with no work. On the first pass (no recorded
// answer) it poses the pay decision and reports false for the suspended
// pass; the answered re-entry applies the orientation.
func unlessProceed(h Host, c *Ctx, sa *cards.SA) bool {
	cost := strings.TrimSpace(sa.Params["UnlessCost"])
	if cost == "" {
		return true
	}
	if sa.API == "Mana" {
		// A mana ability is structurally off the stack (CR 605.3a) and its
		// resolution is driven synchronously inside the payment window — a
		// suspension there would leave the payment flow holding a pending
		// decision with no stack object to resume and drop the mana. The
		// one corpus carrier (Thomil, the Destroyer's "You may sacrifice a
		// creature. If you do, add {B}{B}{B}") keeps its pre-gate
		// behaviour: the UnlessCost$ is ignored and the mana is produced.
		// A real mid-payment ask for a mana ability is not built.
		return true
	}
	switched := strings.EqualFold(strings.TrimSpace(sa.Params["UnlessSwitched"]), "True")
	// The answer and payer cursor are consumed (and cleared) at the top of
	// every pass — the fx42 scoping discipline: an unless SA reached below a
	// consuming SA in the same walk poses its own ask instead of inheriting
	// the outer answer. ctx.UnlessNext is the index of the payer whose
	// answer this pass applies (0 on a first pass; rules' resume arm copies
	// it off the resume point).
	ans := c.UnlessPay
	c.UnlessPay = ""
	idx := c.UnlessNext
	c.UnlessNext = 0
	payers := unlessPayers(h, c, sa)
	switch ans {
	case "pay":
		// Orientation: a pay runs the body exactly when the shape is
		// switched ("if that player does, ...").
		return switched
	case "decline":
		// A decline moves on to the next payer; only when every payer has
		// declined does the orientation decide the body. idx is the payer
		// whose answer this is.
		if idx+1 < len(payers) {
			return !poseUnlessAsk(h, c, sa, cost, payers, idx+1)
		}
		return !switched
	}
	// First pass: pose the pay decision to the first payer. With no
	// resolvable payer the resolving controller is asked — the same fallback
	// the pre-gate Counter primitive used, so the untargeted unpriceable-X
	// counter shapes keep asking today's player.
	if len(payers) == 0 {
		payers = []state.Target{{Player: c.Controller, IsPlayer: true}}
	}
	return !poseUnlessAsk(h, c, sa, cost, payers, 0)
}

// poseUnlessAsk offers payer payers[i] the unless cost. Reports whether the
// resolution suspended; a false return means the host could not ask (an
// effects-package test double, R-9) and the deterministic decline applies —
// with the Note the no-host path has always carried.
func poseUnlessAsk(h Host, c *Ctx, sa *cards.SA, cost string, payers []state.Target, i int) bool {
	payer := c.Controller
	if int(i) < len(payers) && payers[i].IsPlayer {
		payer = payers[i].Player
	}
	shown := unlessCostLabel(cost)
	prompt, payLabel, declineLabel := "Pay "+shown+", or decline", "Pay "+shown, "Don't pay"
	switch sa.API {
	case "Counter":
		prompt = "Pay " + shown + " to save the spell, or decline"
		payLabel = "Pay " + shown + " — don't counter"
	case "CopySpellAbility":
		if strings.EqualFold(strings.TrimSpace(sa.Params["UnlessSwitched"]), "True") {
			prompt = "Pay " + cost + " to copy the spell, or decline"
			payLabel = "Pay " + cost + " — make a copy"
		} else {
			prompt = "Pay " + cost + " to stop the copy, or decline to copy"
			payLabel = "Pay " + cost + " — no copy"
			declineLabel = "Don't pay — make a copy"
		}
	}
	d := &decision.Decision{Player: payer, Kind: decision.KModes,
		Min: 1, Max: 1, Source: c.Source, ResumeKind: "unless_pay",
		ResumeSA: sa, ResumeTarget: i, Prompt: prompt,
		Options: []decision.Option{
			{Index: 0, Kind: "mode", Label: payLabel, Obj: c.Source, Player: payer},
			{Index: 1, Kind: "mode", Label: declineLabel, Obj: c.Source, Player: payer},
		}}
	if Ask(h, d) {
		return true // resolution suspended; the answer re-enters this SA.
	}
	// Fuzz/no-engine host: the deterministic decline (R-9). The pay was
	// never posed, so resolve as if the player declined.
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
		Text: "may pay declined (UnlessCost not asked on this host)"})
	return false
}

// unlessPayers resolves the UnlessPayer$ selector to the players who get the
// pay offer, in deterministic AliveFrom order, deduplicated. The default
// (Forge's own default) is TargetedController — the controller of the first
// target — with the resolving controller as the fallback when nothing
// resolvable is named, so an untargeted SA still asks today's player.
func unlessPayers(h Host, c *Ctx, sa *cards.SA) []state.Target {
	g := h.Game()
	if g == nil {
		return nil
	}
	spec := strings.TrimSpace(sa.Params["UnlessPayer"])
	var out []state.Target
	seen := map[state.PlayerID]bool{}
	add := func(p state.PlayerID) {
		if int(p) < 0 || int(p) >= len(g.Players) || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, state.Target{Player: p, IsPlayer: true})
	}
	addTargets := func(ts []state.Target) {
		for _, t := range ts {
			if t.IsPlayer {
				add(t.Player)
			} else if o := g.Obj(t.Obj); o != nil {
				add(o.Controller)
			}
		}
	}
	switch spec {
	case "", "TargetedController", "TargetedPlayer", "ThisTargetedController", "TargetedOrController":
		addTargets(c.Targets)
	case "You":
		add(c.Controller)
	case "Targeted", "ParentTarget":
		addTargets(c.Targets)
	case "TriggeredTarget":
		addTargets([]state.Target{c.TriggerTarget})
	case "Remembered", "RememberedController", "Player.IsRemembered":
		addTargets(c.Remembered)
	case "Player":
		for _, p := range g.AliveFrom(0) {
			add(p)
		}
	case "Opponent", "Player.Opponent":
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				add(p)
			}
		}
	case "ChosenPlayer":
		addTargets(c.Chosen)
	case "TriggeredPlayer":
		if c.TriggerPlayer.IsPlayer {
			add(c.TriggerPlayer.Player)
		}
	case "TriggeredCardController", "TriggeredCardLKIController":
		if p, ok := TriggeredCardController(g, c.TriggerContext, c.Remembered); ok {
			add(p)
		}
	case "TriggeredActivator":
		if c.TriggerActivator.IsPlayer {
			add(c.TriggerActivator.Player)
		} else if o := g.Obj(c.TriggerActivator.Obj); o != nil {
			add(o.Controller)
		}
	case "TriggeredDefendingPlayer", "DefendingPlayer":
		if c.DefendingPlayer.IsPlayer {
			add(c.DefendingPlayer.Player)
		}
	case "TriggeredAttackingPlayer", "TriggeredAttackerController":
		if c.AttackingPlayer.IsPlayer {
			add(c.AttackingPlayer.Player)
		}
	default:
		// TriggeredSourceSAController, TriggeredSpellAbilityController,
		// TriggeredSourceController, NonTriggeredCardController,
		// EnchantedController, ImprintedController, ... — every form whose
		// referent this build does not model resolves like the default: the
		// targets' controller, else the resolving controller. The
		// TriggeredSourceSA family is the corpus's Counter shape (Reality
		// Smasher): the triggered SA's controller IS the first target's
		// controller there, so the fallback asks the right player without
		// modelling the referent.
		addTargets(c.Targets)
	}
	// Deterministic AliveFrom order, whoever named them.
	alive := g.AliveFrom(0)
	rank := map[state.PlayerID]int{}
	for i, p := range alive {
		rank[p] = i
	}
	sortTargets(out, rank)
	return out
}

// sortTargets orders player targets by the seat order rank (AliveFrom
// position), keeping the slice's own order for ties (no ties arise — add
// dedupes — but the comparator is total regardless).
func sortTargets(ts []state.Target, rank map[state.PlayerID]int) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && rank[ts[j].Player] < rank[ts[j-1].Player]; j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}

// unlessCostLabel renders an UnlessCost$ value for the humans a
// decision.Decision can reach. A plain mana cost ("1", "3", "2 U", "R R") is
// already readable and comes back verbatim -- that is every repo-deck Counter
// with an UnlessCost$ except Mausoleum Wanderer and Reality Smasher.
// Everything else is raw Forge script: a bare SVar name (X, Y, Z, whose value
// this engine does not read at all) or a bracket form (Discard<1/Hand>,
// ExileFromGrave<1/All>, PayLife<5>). Those must not reach a player's screen,
// so they render as "the cost". Display only: the amount actually charged is
// decided by rules' unless-payment path.
func unlessCostLabel(cost string) string {
	fields := strings.Fields(cost)
	if len(fields) == 0 {
		return "the cost"
	}
	for _, f := range fields {
		if _, err := strconv.Atoi(f); err == nil {
			continue // generic amount
		}
		if strings.Trim(f, "WUBRGC") == "" {
			continue // colour/colourless symbols
		}
		return "the cost"
	}
	return cost
}
