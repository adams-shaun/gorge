package state

// Restart folds CR 727.1's game restart into g: the restarted game ends and
// its players start a new one (rule 103) with every Magic card they own
// (CR 727.2), except the cards in keep, which an effect exempted (CR 727.5:
// Karn Liberated's "leaving in exile all non-Aura permanent cards exiled with
// Karn") and which stay exactly where and as they are.
//
// Every other card -- in a library, hand, graveyard, on the battlefield, in
// exile, on the stack or in the command zone (a commander) -- becomes a NEW
// object (CR 400.7) in its OWNER's library, in object-ID order (the rules
// layer's Shuffle event orders it next). Its old object is parked in
// ZCeased. Tokens, copies, ability objects and dungeon objects are not Magic
// cards and simply cease. Sideboard and planar-deck objects are untouched.
//
// Every per-game player and game field returns to its genesis value: life is
// life (the format's starting life), every counter, mana pool and per-turn
// tally is cleared, commander tax and commander damage restart at zero (the
// Commanders list is rewritten to the commanders' new objects), and the
// turn, step, stack, extra turns and phases, designations (monarch,
// initiative), player control and delayed triggers are cleared. A player who
// already lost is not in the restarted game and stays lost (CR 727.1: "All
// players in that game when it ended").
func (g *Game) Restart(life int32, keep []ObjID) {
	kept := make(map[ObjID]bool, len(keep))
	for _, id := range keep {
		kept[id] = true
	}
	dungeon := map[ObjID]bool{}
	for i := range g.Players {
		if id := g.Players[i].DungeonObj; id != 0 {
			dungeon[id] = true
		}
	}
	// keptInZone preserves an exempted object in the zone list it is already
	// in, so "stay exactly where and as they are" holds in every zone, not
	// just exile. The battlefield is keyed by controller and every private
	// zone by owner (events' zoneOwner); the stack is shared, so it is kept
	// separately below.
	keptInZone := make([]map[Zone][]ObjID, len(g.Players))
	for p := range keptInZone {
		keptInZone[p] = map[Zone][]ObjID{}
	}
	var keptStack []ObjID
	// Old commander id -> its new object, so each player's Commanders list
	// names the new objects.
	renamed := map[ObjID]ObjID{}
	newLib := make([][]ObjID, len(g.Players))
	n := len(g.Objs)
	for i := 0; i < n; i++ {
		o := &g.Objs[i]
		switch o.Zone {
		case ZCeased, ZSideboard, ZPlanarDeck:
			continue
		}
		if kept[o.ID] {
			if o.Zone == ZStack {
				keptStack = append(keptStack, o.ID)
				continue
			}
			zonePlayer := o.Owner
			if o.Zone == ZBattlefield {
				zonePlayer = o.Controller
			}
			if int(zonePlayer) < len(g.Players) {
				keptInZone[zonePlayer][o.Zone] = append(keptInZone[zonePlayer][o.Zone], o.ID)
			}
			continue
		}
		id, owner, card := o.ID, o.Owner, o.Card
		isCard := !o.IsToken && !o.IsCopy && o.Ability == nil && !dungeon[id] && card != nil
		o.Zone = ZCeased
		// A player who lost before the restart is not in the new game
		// (CR 727.1, "All players in that game when it ended"), so their
		// cards cease instead of being dealt into a library nobody owns.
		if !isCard || int(owner) >= len(g.Players) || g.Players[owner].Lost {
			continue
		}
		fresh := g.AddObject(card, owner) // may reallocate g.Objs; o is not used past here
		renamed[id] = fresh.ID
		newLib[owner] = append(newLib[owner], fresh.ID)
	}
	for p := range g.Players {
		pid := PlayerID(p)
		keptZones := keptInZone[p]
		for _, z := range []Zone{ZLibrary, ZHand, ZBattlefield, ZGraveyard, ZCommand} {
			g.SetZone(z, pid, keptZones[z])
		}
		g.SetZone(ZExile, pid, keptZones[ZExile])
		var lib []ObjID
		lib = append(lib, newLib[p]...)
		lib = append(lib, keptZones[ZLibrary]...)
		g.SetZone(ZLibrary, pid, lib)
		pl := &g.Players[p]
		cmds := make([]ObjID, 0, len(pl.Commanders))
		for _, id := range pl.Commanders {
			if nid, ok := renamed[id]; ok {
				cmds = append(cmds, nid)
			} else if kept[id] {
				// A kept commander keeps its object; finishRestart must not
				// move it out of the zone it stayed in.
				cmds = append(cmds, id)
			}
		}
		*pl = Player{
			ID: pl.ID, Name: pl.Name, PlayerName: pl.PlayerName, Life: life, Lost: pl.Lost,
			Commanders: cmds,
			CmdCasts:   make([]int32, len(cmds)),
			CmdDamage:  make([]int32, len(pl.CmdDamage)),
		}
		if len(cmds) == 0 {
			pl.Commanders, pl.CmdCasts = nil, nil
		}
		if len(pl.CmdDamage) == 0 {
			pl.CmdDamage = nil
		}
	}
	g.Stack = keptStack
	g.ExcessDamageVictims = nil
	g.Turn, g.Step, g.Passes = 0, 0, 0
	g.Active, g.Priority = 0, 0
	g.StartingPlayer, g.HasStartingPlayer = 0, false
	g.Over, g.Winner, g.Draw = false, 0, false
	g.DelayedNext = 0
	g.ExtraTurns, g.SkipTurns, g.ExtraTurnQueue, g.ExtraPhases = nil, nil, nil, nil
	g.CombatsThisTurn, g.EndStepsThisTurn = 0, 0
	g.ResolvedThisTurn = nil
	g.Monarch, g.HasMonarch, g.TurnStartMonarch, g.HasTurnStartMonarch = 0, false, 0, false
	g.Initiative, g.HasInitiative = 0, false
	g.DayNight, g.PrevTurnSpells = DayNightNeither, 0
	g.ControlledBy, g.ControlArmedTurn, g.ControlCombatOnly = nil, nil, nil
	g.Delayed = nil
	g.Entered = nil
	// goadsSeen, blockersLive, preparedSources and crewedObjects are
	// fast-path gates whose stale-high value is always safe (a scan that
	// finds nothing); they are left as they are.
}
