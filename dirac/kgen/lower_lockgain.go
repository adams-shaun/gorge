package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// lockGainNode reads the node after a RememberDamaged$ DealDamage as the
// permanent life-gain lock on the damaged player (Screaming Nemesis: "If a
// player is dealt damage this way, they can't gain life for the rest of the
// game"): DB$ Effect | Duration$ Permanent | RememberObjects$
// Player.IsRemembered | StaticAbilities$ <svar> with the svar exactly
// Mode$ CantGainLife | ValidPlayer$ Player.IsRemembered, then only the
// Cleanup that clears the remembered set. The kernel folds the node into the
// damage (kop.DmgLockGain: a player dealt damage above 0 takes the pmNoGain
// mark, which playerBanned reads for the rest of the game), so the node
// itself lowers to nothing (lowerCtx.lockGainNode). nil: not this shape.
func (lc *lowerCtx) lockGainNode() *cards.SA {
	if lc.curNode == nil {
		return nil
	}
	n := lc.curNode.Sub
	if n == nil || n.API != "Effect" || n.Kind != "DB" {
		return nil
	}
	p := n.Params
	if strings.TrimSpace(p["Duration"]) != "Permanent" || strings.TrimSpace(p["RememberObjects"]) != "Player.IsRemembered" {
		return nil
	}
	body := strings.TrimSpace(lc.face.SVars[strings.TrimSpace(p["StaticAbilities"])])
	if body == "" {
		return nil
	}
	for _, kv := range strings.Split(body, "|") {
		k, v, _ := strings.Cut(strings.TrimSpace(kv), "$")
		switch strings.TrimSpace(k) {
		case "Mode":
			if strings.TrimSpace(v) != "CantGainLife" {
				return nil
			}
		case "ValidPlayer":
			if strings.TrimSpace(v) != "Player.IsRemembered" {
				return nil
			}
		case "Description":
		default:
			return nil
		}
	}
	for k := range p {
		switch k {
		case "Duration", "RememberObjects", "StaticAbilities", "SubAbility", "Description":
		default:
			return nil
		}
	}
	c := n.Sub
	if c == nil || c.API != "Cleanup" || c.Sub != nil || c.Params["ClearRemembered"] != "True" {
		return nil
	}
	return n
}
