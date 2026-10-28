package neutral

// This file translates the candidate kinds gorge's own engine never poses:
// distribute and choose_pile. The other three the neutral world answers,
// choose_cast_method, choose_option and choose_cost_option, reuse the cast
// and mode asks already in decision.go.
//
// distribute: the v2 host asks one amount per substep for one recipient
// (damage divided among targets, counters placed, and so on). gorge has no
// one ask for a whole division, but its combat-damage order is the
// deterministic split an engine_order host would take, so each substep is a
// one-choice KChoose whose first option is the amount that order assigns:
// lethal damage to a creature (its effective toughness, or 1 with
// deathtouch), everything left on the last recipient. Counters, mana and
// life have no lethal threshold, so they are shared as evenly as the
// remaining substeps allow; the last recipient takes the remainder. The
// options are Amount-labelled so the answer names the split, and the
// candidate's amount maps onto the option with that amount.
//
// choose_pile: gorge's only partition ask is KArrange, which selects cards
// one at a time rather than one of two ready-made piles, so there is no
// native pile ask to reuse. The deterministic stand-in is the first pile
// (pile_index 0), counted in Session.Approx rather than Session.Counts: it is
// a translated approximation, not an untranslated decision.

import (
	"fmt"
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// distribute translates one v2 distribute substep onto a gorge one-choice
// KChoose. The first option is the amount the ordering heuristic wants, so
// the bot's first-offer KChoose policy answers this step with that split.
func (s *Session) distribute(sd *protocol.SeatDecision, v *view.View) ([]mapping.NativeOp, *native, error) {
	if len(sd.Candidates) == 0 {
		return nil, nil, nil
	}
	first := sd.Candidates[0].Semantic
	purpose := str(first, "purpose")
	src, err := optRef(first, "source")
	if err != nil {
		return nil, nil, err
	}
	// Group each candidate under its recipient, keeping first-appearance
	// order, so a host that groups all recipients into one decision and a
	// host that asks one recipient per substep both keep a deterministic
	// option order (no map iteration).
	type group struct {
		key       string
		recipient *protocol.TargetRef
		remaining int
		items     []int
	}
	var groups []group
	recipients := make([]*protocol.TargetRef, len(sd.Candidates))
	amounts := make([]int, len(sd.Candidates))
	for i, c := range sd.Candidates {
		r, err := target(c.Semantic, "recipient")
		if err != nil {
			return nil, nil, err
		}
		if r == nil {
			return nil, nil, nil
		}
		key := ""
		switch {
		case r.Player != nil:
			key = "player:" + *r.Player
		case r.Object != nil:
			key = r.Object.ObjectID
		default:
			return nil, nil, nil
		}
		recipients[i], amounts[i] = r, num(c.Semantic, "amount")
		g := -1
		for j := range groups {
			if groups[j].key == key {
				g = j
				break
			}
		}
		if g < 0 {
			groups = append(groups, group{key: key, recipient: r, remaining: num(c.Semantic, "remaining")})
			g = len(groups) - 1
		}
		groups[g].items = append(groups[g].items, i)
	}
	var order []int
	for gi := range groups {
		g := &groups[gi]
		want := s.distributeShare(sd, v, purpose, src, g.recipient, g.remaining)
		slices.SortStableFunc(g.items, func(a, b int) int {
			aw, bw := 0, 0
			if amounts[a] != want {
				aw = 1
			}
			if amounts[b] != want {
				bw = 1
			}
			if aw != bw {
				return aw - bw
			}
			return amounts[a] - amounts[b]
		})
		order = append(order, g.items...)
	}

	d := decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Prompt: "Distribute " + purpose}
	if src != nil {
		d.Source = s.B.IDs.Of(src.ObjectID)
	}
	key := fmt.Sprintf("distribute|%s|%d|%d", purpose, sd.Group.GroupID, sd.Group.SubstepIndex)
	n := s.open(key, d)
	kind := "division"
	if purpose == "counters" {
		kind = "counter_dist"
	}
	pos := make([]int, len(sd.Candidates))
	for p, ci := range order {
		pos[ci] = p
		o := decision.Option{Index: len(n.d.Options), Kind: kind,
			Label: fmt.Sprintf("%d to %s", amounts[ci], refLabel(recipients[ci])), Amount: amounts[ci]}
		if r := recipients[ci]; r.Player != nil {
			if seat, err := Seat(*r.Player); err == nil {
				o.Player = seat
			}
		} else if r.Object != nil {
			o.Obj = s.B.IDs.Of(r.Object.ObjectID)
			if seat, err := Seat(r.Object.ControllerSeat); err == nil {
				o.Player = seat
			}
		}
		n.d.Options = append(n.d.Options, o)
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i := range sd.Candidates {
		ops[i] = mapping.NativeOp{Op: "choose", Option: pos[i]}
	}
	return ops, n, nil
}

// distributeShare is the amount this substep gives its recipient. The host's
// group tells us how many recipients are left; with no substep count the
// current recipient is treated as the last and takes everything. Damage and
// combat damage follow gorge's engine-order rule (lethal to each creature in
// order, the remainder on the last recipient); counters, mana and life have
// no lethal threshold and are spread evenly, with the last recipient taking
// the remainder.
func (s *Session) distributeShare(sd *protocol.SeatDecision, v *view.View, purpose string,
	src *protocol.ObjectRef, recipient *protocol.TargetRef, remaining int) int {
	steps := int(sd.Group.SubstepCount)
	step := int(sd.Group.SubstepIndex)
	if steps <= 1 || step >= steps-1 {
		return remaining
	}
	switch purpose {
	case "counters", "mana", "life", "other":
		return remaining / (steps - step)
	}
	lethal := remaining
	if recipient.Object != nil {
		if cv, ok := s.cardByV2(v, recipient.Object.ObjectID); ok && isCreature(cv) {
			// gorge's engine-order division measures lethal as the
			// blocker's (effective) toughness, not toughness minus marked
			// damage (mapping.EngineOrderSplit's caller).
			lethal = int(cv.Toughness)
			if lethal < 0 {
				lethal = 0
			}
			if src != nil {
				if a, ok := s.cardByV2(v, src.ObjectID); ok && hasKeyword(a, "Deathtouch") {
					lethal = 1
				}
			}
		}
	}
	// mapping.EngineOrderSplit is the one home of the engine-order rule:
	// this step is the first of a two-recipient split, the synthetic second
	// recipient being the remainder the host's later substeps receive.
	split := mapping.EngineOrderSplit(int32(remaining), []int32{int32(lethal), 0})
	return int(split[0])
}

// pile translates choose_pile to a one-choice KChoose over the offered
// piles. gorge has no native two-pile ask, so the first pile is the
// deterministic stand-in and the approximation is counted in Approx.
func (s *Session) pile(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	if len(sd.Candidates) == 0 {
		return nil, nil, nil
	}
	first := sd.Candidates[0].Semantic
	piles, err := field[[][]protocol.ObjectRef](first, "piles")
	if err != nil {
		return nil, nil, err
	}
	type cand struct {
		index, pile, cards int
	}
	offered := make([]cand, len(sd.Candidates))
	for i, c := range sd.Candidates {
		p := num(c.Semantic, "pile_index")
		cards := 0
		if p >= 0 && p < len(piles) {
			cards = len(piles[p])
		}
		offered[i] = cand{index: i, pile: p, cards: cards}
	}
	slices.SortStableFunc(offered, func(a, b cand) int { return a.pile - b.pile })
	n := s.open("pile", decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Prompt: "Choose a pile"})
	for _, c := range offered {
		n.d.Options = append(n.d.Options, decision.Option{Index: len(n.d.Options), Kind: "pile",
			Label: fmt.Sprintf("Pile %d (%d cards)", c.pile+1, c.cards)})
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for pos, c := range offered {
		ops[c.index] = mapping.NativeOp{Op: "choose", Option: pos}
	}
	s.Approx["choose_pile"]++
	return ops, n, nil
}

// refLabel names a distribute recipient for an option label.
func refLabel(r *protocol.TargetRef) string {
	switch {
	case r == nil:
		return "the recipient"
	case r.Player != nil:
		return *r.Player
	case r.Object != nil:
		return name(*r.Object)
	}
	return "the recipient"
}

// cardByV2 finds a visible battlefield card by its v2 id.
func (s *Session) cardByV2(v *view.View, id string) (view.CardView, bool) {
	for _, p := range v.Players {
		if cv, ok := findCard(p.Battlefield, s.B.IDs.Of(id)); ok {
			return cv, true
		}
	}
	return view.CardView{}, false
}
