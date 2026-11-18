package manabrew

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// The combat damage-division mapping (CR 510.1c). The engine asks it from
// engine/rules/combat.go's askNextDivision — a KChoose whose options are
// uniform Kind "division", one per legal composition of the attacker's
// combat damage into its live blockers, each option's Label listing the
// per-recipient amounts in engine recipient order. The ManaBrew protocol has
// a dedicated prompt for exactly this ask (chooseCombatDamageAssignment) and
// a dedicated answer (combatDamageAssignmentDecision), so the mapping routes
// it there instead of the generic label selection the other uniform-Kind
// asks get.
//
// gorge's decision never names the defending player as a recipient: under
// current CR 510.1c there is no damage-assignment order to declare and the
// engine poses the ask only for non-trample attackers (a trample attacker's
// lethal-then-remainder assignment is decided deterministically engine-side,
// never asked), so the prompt's defenderId stays empty (omitempty).

// promptDamageAssignment maps the combat damage-division ask onto
// chooseCombatDamageAssignment. The bool is the fallback flag: whenever the
// seat's view cannot resolve the ask's recipients — the attacker missing or
// unblocked in the view, a BlockedBy entry off the battlefield, or option
// labels whose segment count does not match the resolved recipients — the
// ask falls back to the generic, always-answerable chooseFromSelection over
// the native option labels rather than posing a prompt the client could
// not answer (the same never-unanswerable contract promptChoose holds).
// TotalDamage is read off the option labels themselves, never re-derived
// from view characteristics: the splits sum to the engine's
// combatDamageAmount, which no view field names directly.
func (t *Translator) promptDamageAssignment(d *decision.Decision, v *view.View) (mb.PromptMessage, bool, error) {
	atk, blockers, ok := divisionRecipientsOf(d, v)
	if !ok {
		return t.fallbackChoose(d, v, true)
	}
	total, ok := divisionTotals(d, len(blockers))
	if !ok {
		return t.fallbackChoose(d, v, true)
	}
	ids := make([]string, 0, len(blockers))
	for _, b := range blockers {
		ids = append(ids, cardID(b.ID))
	}
	dt := false
	if c := findCard(v, atk); c != nil {
		for _, kw := range c.Keywords {
			if strings.EqualFold(strings.TrimPrefix(kw, "kw:"), "Deathtouch") {
				dt = true
				break
			}
		}
	}
	// The protocol's input shape is fixed (no PromptBase field: ManaBrew
	// renders the prompt from its attacker/blocker/total fields alone, as
	// its own doc case shows), so the engine ask's prose does not travel.
	in := mb.ChooseCombatDamageAssignmentInput{AttackerID: cardID(atk), BlockerIDs: ids,
		TotalDamage: total, AttackerHasDeathtouch: dt}
	return mb.PromptMessage{Kind: "prompt", AgentPrompt: mb.AgentPrompt{PromptID: promptID(d),
		DecidingPlayerID: playerID(d.Player), SourceCard: t.sourceCard(v, d.Source),
		Input: mb.PromptInput{Value: in}}}, false, nil
}

// divisionRecipientsOf resolves a combat damage-division ask's attacker and
// its recipients off the seat's view: the options' shared Obj is the
// attacker, the attacker's view BlockedBy lists the blockers in engine
// recipient order (both the engine's liveBlockers and the view's projection
// walk Object.BlockedBy, the fold zeroing every blocker that left), and each
// must still resolve on a battlefield list — a recipient is a battlefield
// creature by definition, so a non-battlefield id means the view and the
// engine disagree and the caller must fall back.
func divisionRecipientsOf(d *decision.Decision, v *view.View) (state.ObjID, []*view.CardView, bool) {
	if d == nil || len(d.Options) == 0 || d.Min != 1 || d.Max != 1 {
		return 0, nil, false
	}
	atk := d.Options[0].Obj
	if atk == 0 {
		return 0, nil, false
	}
	for _, o := range d.Options {
		if o.Obj != atk {
			return 0, nil, false
		}
	}
	if v == nil {
		return 0, nil, false
	}
	c := battlefieldCard(v, atk)
	if c == nil || len(c.BlockedBy) < 2 {
		return 0, nil, false
	}
	blockers := make([]*view.CardView, 0, len(c.BlockedBy))
	for _, bid := range c.BlockedBy {
		b := battlefieldCard(v, bid)
		if b == nil {
			return 0, nil, false
		}
		blockers = append(blockers, b)
	}
	return atk, blockers, true
}

// battlefieldCard resolves an id against the view's battlefield lists only:
// findCard's zone walk is broader than the battlefield a combat recipient
// must sit on.
func battlefieldCard(v *view.View, id state.ObjID) *view.CardView {
	if v == nil {
		return nil
	}
	for _, p := range v.Players {
		for i := range p.Battlefield {
			if p.Battlefield[i].ID == id {
				return &p.Battlefield[i]
			}
		}
	}
	return nil
}

// divisionTotals parses every option's Label into its per-recipient amounts
// (parseDivisionLabel) and reports the ask's total — the sum every legal
// split shares, i.e. the engine's combatDamageAmount for the attacker. One
// label that does not parse into exactly n recipients, or two that sum
// differently, means the labels are not the shapes this mapping recognises
// and the caller falls back.
func divisionTotals(d *decision.Decision, n int) (int, bool) {
	if len(d.Options) == 0 {
		return 0, false
	}
	total := -1
	for _, o := range d.Options {
		amts, ok := ParseDivisionLabel(o.Label, n)
		if !ok {
			return 0, false
		}
		sum := 0
		for _, a := range amts {
			sum += a
		}
		if total < 0 {
			total = sum
		} else if sum != total {
			return 0, false
		}
	}
	return total, true
}

// ParseDivisionLabel reads one option label the engine's divisionOptions
// prints: n comma-joined "N to <name>" segments, segment i naming recipient
// i. Each later segment starts at a comma followed by a digit — that is how
// a card name containing its own comma ("Nissa, Who Shakes the World") is
// skipped — and the whole label must be consumed, so an extra segment cannot
// hide in the last name. A label that does not parse exactly is false, which
// both the prompt builder and the answer parser treat as "not this shape".
// Exported because the wire-side answer builders outside this package
// (mbtest's reach bot, reach.go) need the same label→vector read the
// in-package parser uses.
func ParseDivisionLabel(label string, n int) ([]int, bool) {
	if n <= 0 || label == "" {
		return nil, false
	}
	amts := make([]int, 0, n)
	i := 0
	for seg := 0; seg < n; seg++ {
		if seg > 0 {
			if i >= len(label) || label[i] != ',' {
				return nil, false
			}
			i++
		}
		start := i
		for i < len(label) && label[i] >= '0' && label[i] <= '9' {
			i++
		}
		if i == start {
			return nil, false
		}
		amt := 0
		for _, c := range label[start:i] {
			amt = amt*10 + int(c-'0')
			if amt > 1<<20 {
				return nil, false
			}
		}
		if !strings.HasPrefix(label[i:], " to ") {
			return nil, false
		}
		i += 4
		amts = append(amts, amt)
		// The name runs to the next segment start, or the label's end.
		next := func() (int, bool) {
			for j := i; j+1 < len(label); j++ {
				if label[j] == ',' && label[j+1] >= '0' && label[j+1] <= '9' {
					return j, true
				}
			}
			return 0, false
		}
		if seg < n-1 {
			j, ok := next()
			if !ok {
				return nil, false
			}
			i = j
		} else if _, ok := next(); ok {
			// Content after the n-th segment: an (n+1)-th one.
			return nil, false
		}
	}
	return amts, true
}

// parseDamageAssignment maps combatDamageAssignmentDecision back onto the
// ask's native options: each Assignments entry names an assignee the prompt
// itself offered (its BlockerIDs, in engine recipient order) with a damage
// figure, so the answer is a per-recipient vector, and the answered option
// is the one whose parsed label amounts equal that vector — the engine
// enumerates every legal composition exactly once, so the vector picks out
// one option. Decision.Validate stays the one legality fence: the mapped
// index is submitted through it like every other answer.
func (t *Translator) parseDamageAssignment(out mb.PromptOutputValue, p *Pending) Outcome {
	d := p.Decision
	cur := p.Prompt.PromptID
	dec, ok := out.(mb.DamageAssignmentDecision)
	if !ok {
		return Outcome{Err: errCode(mb.CodeWrongPromptType, "not a combatDamageAssignmentDecision answer", idPtr(cur))}
	}
	in, ok := p.Prompt.Input.Value.(mb.ChooseCombatDamageAssignmentInput)
	if !ok {
		return Outcome{Err: errCode(mb.CodeInvalidShape, "the open prompt carries no combat damage assignment", idPtr(cur))}
	}
	pos := make(map[string]int, len(in.BlockerIDs))
	for i, id := range in.BlockerIDs {
		pos[id] = i
	}
	vec := make([]int, len(in.BlockerIDs))
	for _, a := range dec.Assignments {
		i, ok := pos[a.AssigneeID]
		if !ok {
			return Outcome{Err: errCode(mb.CodeInvalidShape,
				fmt.Sprintf("%q was not offered as an assignee", a.AssigneeID), idPtr(cur))}
		}
		if a.Damage < 0 {
			return Outcome{Err: errCode(mb.CodeInvalidShape,
				fmt.Sprintf("damage %d is negative", a.Damage), idPtr(cur))}
		}
		if vec[i] != 0 {
			return Outcome{Err: errCode(mb.CodeInvalidShape,
				fmt.Sprintf("%q is assigned damage twice", a.AssigneeID), idPtr(cur))}
		}
		vec[i] = a.Damage
	}
	idx := -1
	for _, o := range d.Options {
		if amts, ok := ParseDivisionLabel(o.Label, len(vec)); ok && slices.Equal(amts, vec) {
			idx = o.Index
			break
		}
	}
	if idx < 0 {
		return Outcome{Err: errCode(mb.CodeInvalidShape,
			"the assignment does not match any offered division", idPtr(cur))}
	}
	intent := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}
	if err := d.Validate(intent); err != nil {
		return Outcome{Err: errCode(mb.CodeInvalidShape, err.Error(), idPtr(cur))}
	}
	return Outcome{Intent: &intent}
}
