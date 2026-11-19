package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The "you" player statics: printed S:Mode$ lines whose whole effect is a
// rule the engine consults at one point, scoped to the permanent's controller
// (ValidPlayer$ You). Each is read by a free function over the game state at
// its own site:
//
//   - GainLifeRadiation (Strong, the Brutish Thespian): effRadiationDrain
//     gains the life the rad-counter drain would have cost.
//   - Devotion (Altar of the Pantheon): devotionCount adds one symbol per
//     static, whatever colours are asked about.
//   - FlipCoinDoubler (Krark's Thumb): coinResult flips two coins and lets
//     the flipper keep either.
//   - DrawFromBottom (River Song): LibraryDrawCard names the library's last
//     card as the one a draw takes.
const (
	ModeGainLifeRadiation = "GainLifeRadiation"
	ModeDevotion          = "Devotion"
	ModeFlipCoinDoubler   = "FlipCoinDoubler"
	ModeDrawFromBottom    = "DrawFromBottom"
)

func init() {
	RegisterNonAPI("stat:"+ModeGainLifeRadiation, "stat:"+ModeDevotion,
		"stat:"+ModeFlipCoinDoubler, "stat:"+ModeDrawFromBottom)
}

// youStaticCount counts the live printed statics of mode that player p's
// permanents carry in the exact corpus shape: ValidPlayer$ You and no other
// selector or condition (any unread key fails closed, so a conditioned
// variant never becomes a blanket rule). A phased-out or face-down permanent
// has no functioning printed static (CR 702.26b, 708.2). One static counts
// once, so two copies of a permanent count twice.
func youStaticCount(g *state.Game, p state.PlayerID, mode string) int {
	if int(p) < 0 || int(p) >= len(g.Players) {
		return 0
	}
	n := 0
	for _, owner := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, owner.ID) {
			o := g.Obj(id)
			if o == nil || o.Controller != p || o.PhasedOut || o.FaceDown || o.Face() == nil {
				continue
			}
			for _, st := range o.Face().Statics {
				if st.Mode == mode && st.ParamStr(cards.PKValidPlayer) == "You" &&
					staticKeysReadable(st.Params, "Mode", "ValidPlayer", "Description") {
					n++
				}
			}
		}
	}
	return n
}

// LibraryDrawCard is the card of lib (player p's library, top first) that p's
// next draw takes: the top card, or the bottom one while p controls a
// DrawFromBottom static. lib must be non-empty.
func LibraryDrawCard(g *state.Game, p state.PlayerID, lib []state.ObjID) state.ObjID {
	if youStaticCount(g, p, ModeDrawFromBottom) > 0 {
		return lib[len(lib)-1]
	}
	return lib[0]
}
