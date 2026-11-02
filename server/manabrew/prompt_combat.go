package manabrew

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// promptCombat: attackers are grouped per creature (the
// engine offers one option per creature×defender pair), blockers are the
// per-defender (blocker, attacker) pairs the engine asks one defender at a
// time. The per-attacker CR 509.1a/509.1c bounds ride the BlockableAttacker
// so a rules-ignorant client can size its declaration before the engine's
// own declaration gate rejects it.
func (t *Translator) promptCombat(d *decision.Decision, v *view.View) (mb.PromptMessage, error) {
	switch d.Kind {
	case decision.KAttackers:
		return t.promptAttackers(d, v)
	case decision.KBlockers:
		return t.promptBlockers(d, v)
	default:
		return mb.PromptMessage{}, ErrUnmapped
	}
}

// promptAttackers builds chooseAttackers. It needs the seat view: the
// attack-target labels and the planeswalker/battle kinds come from the
// permanent's projected type line, which is exactly the projection the
// client would render anyway.
func (t *Translator) promptAttackers(d *decision.Decision, v *view.View) (mb.PromptMessage, error) {
	if v == nil {
		return mb.PromptMessage{}, fmt.Errorf("manabrew: chooseAttackers needs the seat view for attack-target labels")
	}
	attackers := make([]mb.AttackerOptionDto, 0, len(d.Options))
	targets := make([]mb.AttackTargetDto, 0, 2)
	seenTarget := make(map[string]bool)
	seenAttacker := make(map[state.ObjID]int) // first-occurrence position in attackers
	for _, opt := range d.Options {
		refID := attackTargetID(opt)
		if !seenTarget[refID] {
			seenTarget[refID] = true
			targets = append(targets, t.attackTarget(opt, v))
		}
		if pos, ok := seenAttacker[opt.Obj]; ok {
			attackers[pos].ValidTargetIDs = append(attackers[pos].ValidTargetIDs, refID)
			attackers[pos].MustAttack = attackers[pos].MustAttack || opt.Required
			continue
		}
		seenAttacker[opt.Obj] = len(attackers)
		attackers = append(attackers, mb.AttackerOptionDto{
			AttackerID:     cardID(opt.Obj),
			ValidTargetIDs: []string{refID},
			MustAttack:     opt.Required,
		})
	}
	in := mb.ChooseAttackersInput{Attackers: attackers, AttackTargets: targets, Presentation: declarationPresentation(d, v, "attack")}
	return mb.PromptMessage{Kind: "prompt", AgentPrompt: mb.AgentPrompt{
		PromptID: promptID(d), DecidingPlayerID: playerID(d.Player),
		Input: mb.PromptInput{Value: in}}}, nil
}

// declarationPresentation frames a declaration whose legality a creature's own
// fields can't show, and is nil for any other. The sentences give each creature
// the id an answer names it by, and their wording is a contract with the clients
// that read them, mbtest's mock among them.
func declarationPresentation(d *decision.Decision, v *view.View, verb string) *mb.PromptPresentation {
	var sentences []string
	if names := aloneRestricted(d, v); names != nil {
		// CR 506.5.
		sentences = append(sentences, fmt.Sprintf("Can't %s alone: %s.", verb, strings.Join(names, ", ")))
	}
	if ids := chargeableTogether(d, verb); ids != nil {
		sentences = append(sentences, fmt.Sprintf("These can %s together: %s.", verb, strings.Join(ids, ", ")))
	}
	if sentences == nil {
		return nil
	}
	return &mb.PromptPresentation{Title: d.Prompt, Description: strings.Join(sentences, " "), Targets: []mb.TargetRef{}}
}

// aloneRestricted names the creatures that can't be declared alone.
func aloneRestricted(d *decision.Decision, v *view.View) []string {
	var names []string
	seen := make(map[state.ObjID]bool)
	for _, opt := range d.Options {
		if !opt.AloneRestricted || seen[opt.Obj] {
			continue
		}
		seen[opt.Obj] = true
		name := "A creature"
		if c := findCard(v, opt.Obj); c != nil && c.Printing.Name != "" {
			name = c.Printing.Name
		}
		names = append(names, fmt.Sprintf("%s (%s)", name, cardID(opt.Obj)))
	}
	return names
}

// chargeableTogether lists creatures a declaration can hold at once, when
// something bounds the declaration as a whole: an attack or block tax in mana or
// life, a tax that taps creatures from a pool the declared ones leave, or a cap on
// attackers per defender. None of those show on a creature, and fewer of the
// listed creatures are legal too. Each creature is priced at its first offered
// option, the first target its prompt lists. Those required to attack or block are
// taken first. Nil when nothing bounds the declaration.
func chargeableTogether(d *decision.Decision, verb string) []string {
	if d.ChargeTapPool <= 0 && !d.HasBudget() && d.PayerLife <= 0 && (verb != "attack" || !optionsHaveGroups(d.Options)) {
		return nil
	}
	var chosen []int
	sum := 0
	groups := map[string]int{}
	seen := make(map[state.ObjID]bool)
	take := func(required bool) {
		for _, o := range d.Options {
			if seen[o.Obj] || o.Required != required {
				continue
			}
			if !d.ChargeOptionsFit(append(chosen[:len(chosen):len(chosen)], o.Index)) ||
				(d.HasBudget() && sum+o.Value > d.MaxSum) || !d.GroupAdmits(groups, o.Group) {
				continue
			}
			seen[o.Obj] = true
			chosen = append(chosen, o.Index)
			sum += o.Value
			if o.Group != "" {
				groups[o.Group]++
			}
		}
	}
	take(true)
	take(false)
	ids := make([]string, 0, len(chosen))
	for _, c := range chosen {
		ids = append(ids, cardID(d.Options[c].Obj))
	}
	return ids
}

// attackTargetID is the wire ref id of an attacker option's target: the
// battle/planeswalker permanent for a permanent attack, the defender's
// player id for a player attack.
func attackTargetID(opt decision.Option) string {
	if opt.Battle != 0 {
		return cardID(opt.Battle)
	}
	return playerID(opt.Player)
}

// attackTarget projects one attack target's label and kind from the view.
// The kind is derived from the permanent's projected type half, not from a
// guess: a permanent that projects neither Battle nor Planeswalker falls
// back to planeswalker, which is what CR 310.7's permanent-attack family
// has been in every non-battle case so far.
func (t *Translator) attackTarget(opt decision.Option, v *view.View) mb.AttackTargetDto {
	if opt.Battle != 0 {
		c := findCard(v, opt.Battle)
		kind := "planeswalker"
		label := ""
		if c != nil {
			if hasType(c, "Battle") {
				kind = "battle"
			}
			label = c.Printing.Name
			if label == "" {
				label = c.Name
			}
		}
		return mb.AttackTargetDto{ID: cardID(opt.Battle), Label: label, Kind: kind}
	}
	return mb.AttackTargetDto{ID: playerID(opt.Player), Label: playerLabel(v, opt.Player), Kind: "player"}
}

// promptBlockers builds chooseBlockers: one BlockableAttacker per distinct
// attacker (first-occurrence order), with the CR 509.1a MinMaxBlocker
// bounds and the CR 509.1c must-be-blocked flag the options carried
// server-side (Option.AttackMust), the blockers valid against that
// attacker, and every blocker id offered. A MaxBlockers of 0 (the omitempty
// zero) means "no ceiling", so the pointer is set only above zero.
//
// The engine's blocker options are blocker-major (one creature's candidate
// attackers contiguous, then the next creature's), NOT attacker-major, so
// a repeated attacker's options are scattered through the option list.
// Each option must therefore join ITS OWN attacker's group via a
// first-occurrence position map (the same shape promptAttackers uses) --
// the earlier "append to the last group" shortcut silently moved a
// blocker's later attackers onto whichever attacker happened to be last
// (seed 2004: option [4: 82->36] landed in o39's ValidBlockerIDs, so
// a real client could never block o82 against o36).
func (t *Translator) promptBlockers(d *decision.Decision, v *view.View) (mb.PromptMessage, error) {
	attackers := make([]mb.BlockableAttackerDto, 0, len(d.Options))
	blockers := make([]string, 0, len(d.Options))
	seenAttacker := make(map[string]int) // attacker ref -> first-occurrence position in attackers
	seenBlocker := make(map[state.ObjID]bool)
	for _, opt := range d.Options {
		aid := cardID(opt.Attacker)
		if pos, ok := seenAttacker[aid]; ok {
			last := &attackers[pos]
			last.MustBeBlocked = last.MustBeBlocked || opt.AttackMust
			last.ValidBlockerIDs = append(last.ValidBlockerIDs, cardID(opt.Obj))
		} else {
			seenAttacker[aid] = len(attackers)
			attackers = append(attackers, mb.BlockableAttackerDto{
				AttackerID:      aid,
				ValidBlockerIDs: []string{cardID(opt.Obj)},
				MustBeBlocked:   opt.AttackMust,
			})
			if opt.MinBlockers > 0 {
				attackers[len(attackers)-1].MinBlockers = opt.MinBlockers
			}
			if opt.MaxBlockers > 0 {
				attackers[len(attackers)-1].MaxBlockers = &opt.MaxBlockers
			}
		}
		if !seenBlocker[opt.Obj] {
			seenBlocker[opt.Obj] = true
			blockers = append(blockers, cardID(opt.Obj))
		}
	}
	in := mb.ChooseBlockersInput{
		Attackers:           attackers,
		AvailableBlockerIDs: blockers,
		Presentation:        declarationPresentation(d, v, "block"),
	}
	return mb.PromptMessage{Kind: "prompt", AgentPrompt: mb.AgentPrompt{
		PromptID: promptID(d), DecidingPlayerID: playerID(d.Player),
		Input: mb.PromptInput{Value: in}}}, nil
}

// parseAttackers maps declareAttackers: each assignment names one offered
// (attacker, target) option; the response order is the declaration order.
// An unknown pairing is invalidShape ("not offered"), not unknownActionId --
// the ids are refs the prompt minted, not action ids.
func (t *Translator) parseAttackers(out mb.PromptOutputValue, p *Pending) Outcome {
	d := p.Decision
	cur := p.Prompt.PromptID
	dec, ok := out.(mb.DeclareAttackersDecision)
	if !ok {
		return Outcome{Err: errCode(mb.CodeWrongPromptType, "not a declareAttackers answer", idPtr(cur))}
	}
	choices := make([]int, 0, len(dec.Assignments))
	for _, a := range dec.Assignments {
		idx := -1
		for _, opt := range d.Options {
			if cardID(opt.Obj) != a.AttackerID || attackTargetID(opt) != a.TargetID {
				continue
			}
			idx = opt.Index
			break
		}
		if idx < 0 {
			return Outcome{Err: errCode(mb.CodeInvalidShape,
				fmt.Sprintf("attack %s -> %s is not offered", a.AttackerID, a.TargetID), idPtr(cur))}
		}
		choices = append(choices, idx)
	}
	intent := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
	if err := d.Validate(intent); err != nil {
		return Outcome{Err: errCode(mb.CodeInvalidShape, err.Error(), idPtr(cur))}
	}
	return Outcome{Intent: &intent}
}

// parseBlockers maps declareBlockers the same way: one offered
// (blocker, attacker) pair per assignment, response order kept.
func (t *Translator) parseBlockers(out mb.PromptOutputValue, p *Pending) Outcome {
	d := p.Decision
	cur := p.Prompt.PromptID
	dec, ok := out.(mb.DeclareBlockersDecision)
	if !ok {
		return Outcome{Err: errCode(mb.CodeWrongPromptType, "not a declareBlockers answer", idPtr(cur))}
	}
	choices := make([]int, 0, len(dec.Assignments))
	for _, a := range dec.Assignments {
		idx := -1
		for _, opt := range d.Options {
			if cardID(opt.Obj) == a.BlockerID && cardID(opt.Attacker) == a.AttackerID {
				idx = opt.Index
				break
			}
		}
		if idx < 0 {
			return Outcome{Err: errCode(mb.CodeInvalidShape,
				fmt.Sprintf("block %s -> %s is not offered", a.BlockerID, a.AttackerID), idPtr(cur))}
		}
		choices = append(choices, idx)
	}
	intent := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
	if err := d.Validate(intent); err != nil {
		return Outcome{Err: errCode(mb.CodeInvalidShape, err.Error(), idPtr(cur))}
	}
	return Outcome{Intent: &intent}
}
