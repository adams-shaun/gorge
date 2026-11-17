package trigmatch

import "github.com/adams-shaun/gorge/engine/events"

// SpellsCastLastTurn reads the previous turn's spells off log: the
// PutOnStack casts (less CR 733.1 reversals) between the second-to-last and
// the last TurnChange, in all and the most by any one player. Before a turn
// has completed there was no last turn and both are zero. Game.PrevTurnSpells
// is the day/night half of the same window (the outgoing active seat only,
// CR 731.2); the werewolf intervening-ifs count every player's spells.
func SpellsCastLastTurn(log []events.Event) (total, most int) {
	end := len(log)
	for end > 0 && log[end-1].Kind != events.TurnChange {
		end--
	}
	if end == 0 {
		return 0, 0
	}
	end--
	start := end
	for start > 0 && log[start-1].Kind != events.TurnChange {
		start--
	}
	if start == 0 {
		return 0, 0
	}
	var per [8]int
	var rev events.CastReversals
	for i := end - 1; i >= start; i-- {
		ev := log[i]
		if rev.Undone(&ev) || ev.Kind != events.PutOnStack {
			continue
		}
		if int(ev.Player) >= 0 && int(ev.Player) < len(per) {
			per[ev.Player]++
			total++
		}
	}
	for _, n := range per {
		most = max(most, n)
	}
	return total, most
}

// DayNightSwitched reports whether the DayNightChange ev logged in log switched an
// existing designation: an earlier DayNightChange precedes it with no CR 727
// restart between. Only a restart returns the game to neither (the fold
// ignores any other value), so the first change after the game's start or a
// restart is neither becoming day or night.
func DayNightSwitched(log []events.Event, ev events.Event) bool {
	i := len(log) - 1
	for i >= 0 && log[i].Seq != ev.Seq {
		i--
	}
	for i--; i >= 0; i-- {
		switch log[i].Kind {
		case events.DayNightChange:
			return true
		case events.GameRestart:
			return false
		}
	}
	return false
}
