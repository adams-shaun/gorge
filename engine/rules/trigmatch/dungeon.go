package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// dungeonCompletedMatches implements Mode$ DungeonCompleted ("Whenever you
// complete a dungeon", Varis, Silverymoon Ranger; Loot Dispute; Sefris of
// the Hidden Ways; Dungeon Crawler from the graveyard). CR 309.7: a player
// completes a dungeon as it leaves the command zone. Each of the three
// completion sites (effects' venture off the bottom room, rules'
// CompleteDungeon and the SBA batch, both guarded by !DungeonCompleted)
// emits exactly one events.DungeonComplete per completion, so one event is
// one firing. ValidPlayer$ is tested against the completing player with the
// trigger's controller as "you", the BecomeMonarchMatches shape.
func dungeonCompletedMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.DungeonComplete {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v := t.ParamStr(cards.PKValidPlayer); v != "" && !effects.MatchesPlayerSpec(e.Game(), v, ev.Player, ctrl) {
		return false
	}
	return true
}

func init() {
	registerTrigMatcher(dungeonCompletedMatches, "DungeonCompleted")
	effects.RegisterNonAPI("trig:DungeonCompleted")
}
