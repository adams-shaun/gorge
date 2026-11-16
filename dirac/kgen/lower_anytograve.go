package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// "Whenever a card is put into a graveyard from anywhere" (ChangesZone with
// Origin$ Any, Destination$ Graveyard, a listener on the battlefield:
// Compost, Vulturous Zombie, The Haunt of Hightower). gorge matches the
// move's ValidCard$ once per card as the move completes (trigmatch
// ZoneChangeMatchesWithCapture), whatever zone the card comes from. The
// kernel splits graveyard arrivals in two (dirac/trig_tograve.go): a Dies line
// fires from the battlefield with the card's last known information, and a
// ToGraveyard line fires from placeFromK for every other origin, and a move
// reaches exactly one of them. So the one printed line lowers as two kernel
// triggers with the same body and the same ValidCard$: the ToGraveyard line
// with the every-zone-but-battlefield origin set, and the Dies line.
//
// A line naming the card itself (Dread, Serra Avatar, Worldspine Wurm) listens
// from the graveyard it lands in, which no battlefield walk sees, and a
// non-battlefield trigger zone (Nihilith's exile) is not a battlefield walk
// either: both stay on the refusal path.

// anyToGraveOrigin is the Origin$ set of the ToGraveyard half: every zone a
// card is placed into a graveyard from but the battlefield
// (trigToGraveyardSet reads it).
const anyToGraveOrigin = "Hand,Graveyard,Exile,Stack,Library,Command"

// anyToGraveHalves splits an Origin$ Any -> Graveyard battlefield listener into
// the origin-set (ToGraveyard) and battlefield-origin (Dies) spellings.
func anyToGraveHalves(t cards.Trigger) (other, bf cards.Trigger, ok bool) {
	if t.Mode != "ChangesZone" || strings.TrimSpace(t.Params["Destination"]) != "Graveyard" {
		return t, t, false
	}
	if o := strings.TrimSpace(t.Params["Origin"]); o != "" && o != "Any" {
		return t, t, false
	}
	if z := strings.TrimSpace(t.Params["TriggerZones"]); z != "" && z != "Battlefield" {
		return t, t, false
	}
	if strings.Contains(t.Params["ValidCard"], "Self") {
		return t, t, false
	}
	other, bf = t, t
	other.Params = make(map[string]string, len(t.Params))
	bf.Params = make(map[string]string, len(t.Params))
	for k, v := range t.Params {
		other.Params[k], bf.Params[k] = v, v
	}
	other.Params["Origin"] = anyToGraveOrigin
	bf.Params["Origin"] = "Battlefield"
	return other, bf, true
}

// lowerTriggerSplit lowers a T: line that is several kernel triggers (done is
// true, err the result), or reports done false for the ordinary single one.
//
// The Eminence two-zone spelling (TriggerZones$ Command,Battlefield, gorge
// zoneGate's zoneSpecContains): the same printed trigger functions from the
// command zone and from the battlefield. The kernel has one Trigger.Zone per
// trigger -- a battlefield subscription (the ordinary walks) or a
// command-zone listener (eachCommandTrigger, commanders only) -- so the line
// lowers as one ability per zone. The two are mutually exclusive where the
// kernel and gorge both fire it: a card in the command zone has no
// battlefield slot to subscribe, a card on the battlefield is not in a command
// zone, and a Phase trigger looks back in time in neither (gorge zoneGate's
// zones[] walk covers only MoveZone, Draw and PutOnStack events), so gorge's
// single zone gate reads the same.
func (lc *lowerCtx) lowerTriggerSplit(t cards.Trigger, zone string) (done bool, err error) {
	if zone != "" {
		return false, nil
	}
	markSplit := func(n int) {
		for _, a := range lc.out.Abilities[n:] {
			a.SplitPrev = true // the printed identity is the first half's (kop.Ability.SplitPrev)
		}
	}
	if halves, ok := triggerZoneHalves(t.Params["TriggerZones"]); ok {
		for j, half := range halves {
			tt := t
			tt.Params = make(map[string]string, len(t.Params))
			for k, v := range t.Params {
				tt.Params[k] = v
			}
			tt.Params["TriggerZones"] = half
			n := len(lc.out.Abilities)
			if err := lc.lowerTriggered(tt, zone); err != nil {
				return true, err
			}
			if j != 0 {
				markSplit(n)
			}
		}
		return true, nil
	}
	if other, bf, ok := anyToGraveHalves(t); ok {
		n := len(lc.out.Abilities)
		if err := lc.lowerTriggered(other, zone); err != nil {
			return true, err
		}
		n = len(lc.out.Abilities)
		if err := lc.lowerTriggered(bf, zone); err != nil {
			return true, err
		}
		markSplit(n)
		return true, nil
	}
	return false, nil
}
