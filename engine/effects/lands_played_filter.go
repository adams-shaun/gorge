package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// landPlayPred is one predicate of a PlayerCountPropertyYou$LandsPlayed
// <filter> argument: a played land's origin zone (inRealZone<Zone>, read as
// the zone it was played from, Forge's last-known zone of the played card) or
// its ownership relative to the reading player (YouOwn / YouDontOwn), either
// optionally negated with a leading "!".
type landPlayPred struct {
	neg    bool
	zone   state.Zone
	byZone bool
	owned  bool // byZone false: true for YouOwn, false for YouDontOwn
}

type landPlayOwnCode uint16

const (
	landPlayOwnYouOwn landPlayOwnCode = iota + 1
	landPlayOwnYouDontOwn
)

var landPlayOwnCodes = state.NewStrCodes(
	state.StrEntry[landPlayOwnCode]{Key: "YouOwn", Val: landPlayOwnYouOwn},
	state.StrEntry[landPlayOwnCode]{Key: "YouDontOwn", Val: landPlayOwnYouDontOwn},
)

// parseLandPlayFilter parses the argument into its comma-separated
// alternatives (Forge's Valid list: a play matching any one counts), each a
// "Card." head with "+"-joined predicates that must all hold. Any other word
// is not ok, so the read fails closed rather than counting unfiltered.
func parseLandPlayFilter(arg string) ([][]landPlayPred, bool) {
	var alts [][]landPlayPred
	for _, alt := range strings.Split(arg, ",") {
		body, ok := strings.CutPrefix(strings.TrimSpace(alt), "Card.")
		if !ok || body == "" {
			return nil, false
		}
		var preds []landPlayPred
		for _, w := range strings.Split(body, "+") {
			var p landPlayPred
			w, p.neg = strings.CutPrefix(w, "!")
			if z, ok := strings.CutPrefix(w, "inRealZone"); ok {
				zone, zok := parseZone(z)
				if !zok {
					return nil, false
				}
				p.byZone, p.zone = true, zone
			} else {
				switch landPlayOwnCodes.Code(w) {
				case landPlayOwnYouOwn:
					p.owned = true
				case landPlayOwnYouDontOwn:
				default:
					return nil, false
				}
			}
			preds = append(preds, p)
		}
		alts = append(alts, preds)
	}
	return alts, true
}

// landPlayMatches reports whether a play from `from` of a card owned by owner
// satisfies any alternative, read for player you.
func landPlayMatches(alts [][]landPlayPred, from state.Zone, owner, you state.PlayerID) bool {
	for _, preds := range alts {
		all := true
		for _, p := range preds {
			hold := owner == you
			if !p.owned {
				hold = !hold
			}
			if p.byZone {
				hold = from == p.zone
			}
			if hold == p.neg {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// landsPlayedFiltered answers PlayerCountPropertyYou$LandsPlayed <filter>
// for player p: the lands p played this turn whose play matches the filter.
// Every play in LandsPlayed beyond the Player.LandsPlayedOffHand records was
// of an owned card from the hand. Not ok for an argument outside the
// supported vocabulary (fail closed).
func landsPlayedFiltered(pl *state.Player, you state.PlayerID, arg string) (int32, bool) {
	alts, ok := parseLandPlayFilter(arg)
	if !ok {
		return 0, false
	}
	var n int32
	if fromHand := pl.LandsPlayed - int32(len(pl.LandsPlayedOffHand)); fromHand > 0 &&
		landPlayMatches(alts, state.ZHand, you, you) {
		n += fromHand
	}
	for _, lp := range pl.LandsPlayedOffHand {
		if landPlayMatches(alts, lp.From, lp.Owner, you) {
			n++
		}
	}
	return n, true
}

// landsPlayedCount answers PlayerCountPropertyYou$LandsPlayed for the
// resolving controller. An argument filters the plays (Spider-Man 2099's
// "Card.!inRealZoneHand,Card.YouDontOwn", Visions of Phyrexia's
// "Card.inRealZoneExile"); an unknown filter word fails closed.
func landsPlayedCount(g *state.Game, c *Ctx, arg string) (int32, bool) {
	if c.Controller < 0 || int(c.Controller) >= len(g.Players) {
		return 0, false
	}
	if a := strings.TrimSpace(arg); a != "" {
		return landsPlayedFiltered(&g.Players[c.Controller], c.Controller, a)
	}
	return g.Players[c.Controller].LandsPlayed, true
}
