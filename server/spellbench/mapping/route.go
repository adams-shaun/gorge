package mapping

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func optKinds(d *decision.Decision) (set map[string]bool, sorted string) {
	set = map[string]bool{}
	var ks []string
	for _, o := range d.Options {
		if !set[o.Kind] {
			set[o.Kind] = true
			ks = append(ks, o.Kind)
		}
	}
	sort.Strings(ks)
	return set, strings.Join(ks, ",")
}

func Route(d *decision.Decision) string {
	k, sorted := optKinds(d)
	switch d.Kind {
	case decision.KPriority:
		return "priority"
	case decision.KAttackers:
		return "attackers"
	case decision.KBlockers:
		return "blockers"
	case decision.KTarget:
		return "target"
	case decision.KTriggerOrder:
		return "trigger_order"
	case decision.KModes:
		for _, o := range d.Options {
			if o.Mode == decision.ModeUnlessPay || o.Mode == decision.ModeUnlessDecline {
				return "modes/unless"
			}
		}
		if k["discard"] {
			return "modes/discard"
		}
		return "modes/mode"
	case decision.KMulligan:
		if k["bottom"] {
			return "mulligan/bottom"
		}
		return "mulligan/keep"
	case decision.KTriggerOptional:
		if d.ResumeKind == "madness" {
			return "trigger_optional/madness"
		}
		return "trigger_optional/optional"
	case decision.KStartingPlayer:
		return "starting_player/player"
	case decision.KReplacement:
		if k["madness_exile"] || k["madness_graveyard"] {
			return "replacement/madness"
		}
		if k["replacement"] {
			return "replacement/order"
		}
	case decision.KArrange:
		if len(d.Options) > 0 {
			return "arrange/" + d.Options[0].Kind
		}
	case decision.KChoose:
		switch {
		case k["activate"] || (k["done"] && len(k) == 1):
			return "choose/mana_window"
		case k["trigger_cost_pay"] || k["trigger_cost_decline"]:
			return "choose/trigger_cost"
		case k["sacrifice"] || k["tapcost"] || k["returncost"] || k["exile_cost"] || k["exilecost"] || k["exile"] || (k["discard"] && d.Source != 0):
			return "choose/cost"
		case k["discard"]:
			return "choose/cleanup_discard"
		case k["yes"] || k["no"]:
			return "choose/yesno"
		case k["graveyard"] && k["top"]:
			return "choose/explore"
		}
		for _, o := range d.Options {
			if strings.HasPrefix(o.Kind, "pay_") {
				return "choose/pay_pip"
			}
		}
		if len(k) == 1 {
			for _, c := range []string{"search", "hand_move", "dig", "untap", "keep", "hidden_pick", "vote_card", "x", "number", "color", "type", "name", "division", "mana"} {
				if k[c] {
					return "choose/" + c
				}
			}
		}
		// The named-option classes come last so every route above keeps its
		// decision. A choice whose options name distinct objects is a
		// select_object ask; any other named-option ask is an effect option.
		// vote and twopiles keep their old unmapped fallback: vote encodes its
		// ballot index in Obj, and twopiles is a multi-pick owned by
		// sbp-unmapped-dig-division.
		//
		// A cumulative floor (MinSum) or budget (MaxSum/Budgeted, including
		// the second budget) is a whole-answer rule the one-pick candidates
		// below cannot express, and AllowNone's empty answer has no candidate
		// at all: those asks stay on the old unmapped fallback so the native
		// bot answers them instead of the v2 seat being posed an intent
		// Decision.Validate rejects. A Max of 0 likewise admits only the empty
		// answer.
		if len(d.Options) > 0 && d.Max >= 1 && !k["vote"] && !k["twopiles"] &&
			!d.AllowNone && d.MinSum <= 0 && !d.HasBudget() && !d.HasBudget2() {
			// choose/object commits exactly one pick; a Min>1 object ask
			// instead takes the option route's group substeps.
			if d.Min <= 1 && chooseObjectRoute(d, k) {
				return "choose/object"
			}
			return "choose/option"
		}
	}
	return "unmapped:" + string(d.Kind) + "/" + sorted
}

// chooseObjectRoute reports whether an ask's options are one select_object
// choice each: either the kinds are exactly {player}, one distinct player
// per option, or every option names a distinct object. Repeated objects are
// not a select_object ask (its candidates would be identical), so they use
// the option route instead.
func chooseObjectRoute(d *decision.Decision, k map[string]bool) bool {
	players := map[state.PlayerID]bool{}
	objects := map[state.ObjID]bool{}
	onlyPlayers := len(k) == 1 && k["player"]
	for _, o := range d.Options {
		if onlyPlayers {
			if players[o.Player] {
				return false
			}
			players[o.Player] = true
			continue
		}
		if o.Obj == 0 || objects[o.Obj] {
			return false
		}
		objects[o.Obj] = true
	}
	return len(d.Options) > 0
}
