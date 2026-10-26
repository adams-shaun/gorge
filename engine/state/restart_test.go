package state

import "testing"

// restartZoneCard adds a card owned by p to zone z (or the shared stack for
// ZStack) and returns its id. Battlefield membership is controller-keyed;
// AddObject seats the new object's controller at its owner, so p works for
// every zone.
func restartZoneCard(t *testing.T, g *Game, p PlayerID, z Zone, name string) ObjID {
	t.Helper()
	o := g.AddObject(testCard(t, "Name:"+name+"\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), p)
	o.Zone = z
	if z == ZStack {
		g.Stack = append(g.Stack, o.ID)
		return o.ID
	}
	g.SetZone(z, p, append(g.Zone(z, p), o.ID))
	return o.ID
}

func restartHasZone(g *Game, z Zone, p PlayerID, id ObjID) bool {
	for _, x := range g.Zone(z, p) {
		if x == id {
			return true
		}
	}
	return false
}

// TestRestartKeepsExemptCardsInEveryZone: CR 727.5's exempted objects keep
// their object AND their zone membership in every zone, not only exile; the
// old fold cleared every zone list but exile and nil'd the stack, leaving a
// kept card live but orphaned.
func TestRestartKeepsExemptCardsInEveryZone(t *testing.T) {
	g := NewGame([]string{"a", "b"})
	keepHand := restartZoneCard(t, g, 0, ZHand, "KeepHand")
	keepStack := restartZoneCard(t, g, 0, ZStack, "KeepStack")
	keepBF := restartZoneCard(t, g, 0, ZBattlefield, "KeepBattlefield")
	keepGrave := restartZoneCard(t, g, 0, ZGraveyard, "KeepGraveyard")
	keepExile := restartZoneCard(t, g, 0, ZExile, "KeepExile")
	keepCommand := restartZoneCard(t, g, 0, ZCommand, "KeepCommander")
	drop := restartZoneCard(t, g, 0, ZHand, "DropHand")
	g.Players[0].Commanders = []ObjID{keepCommand}
	g.Players[0].CmdCasts = []int32{2}
	g.Players[0].CmdDamage = []int32{3}
	keep := []ObjID{keepHand, keepStack, keepBF, keepGrave, keepExile, keepCommand}

	g.Restart(20, keep)

	for _, tc := range []struct {
		name string
		zone Zone
		id   ObjID
	}{
		{"hand", ZHand, keepHand},
		{"stack", ZStack, keepStack},
		{"battlefield", ZBattlefield, keepBF},
		{"graveyard", ZGraveyard, keepGrave},
		{"exile", ZExile, keepExile},
		{"command", ZCommand, keepCommand},
	} {
		if got := g.Obj(tc.id).Zone; got != tc.zone {
			t.Errorf("kept %s object %d zone = %s, want %s", tc.name, tc.id, got, tc.zone)
		}
		if !restartHasZone(g, tc.zone, 0, tc.id) {
			t.Errorf("kept %s object %d is not in %s %v (orphaned live object)", tc.name, tc.id, tc.zone, g.Zone(tc.zone, 0))
		}
	}
	if got := g.Zone(ZStack, 0); len(got) != 1 || got[0] != keepStack {
		t.Errorf("stack = %v, want [%d]", got, keepStack)
	}
	if got := g.Obj(drop).Zone; got != ZCeased {
		t.Errorf("non-kept hand card zone = %s, want Ceased", got)
	}
	lib := g.Zone(ZLibrary, 0)
	if len(lib) != 1 || lib[0] == drop || g.Obj(lib[0]).Card == nil {
		t.Fatalf("library = %v, want exactly one fresh card", lib)
	}
	if got := g.Obj(lib[0]).Face().Name; got != "DropHand" {
		t.Errorf("minted library card = %q, want DropHand", got)
	}
	if cmds := g.Players[0].Commanders; len(cmds) != 1 || cmds[0] != keepCommand {
		t.Errorf("Commanders = %v, want the kept commander [%d]", cmds, keepCommand)
	}
	if got := g.Players[0].CmdCasts; len(got) != 1 || got[0] != 0 {
		t.Errorf("CmdCasts = %v, want one zero", got)
	}
	if got := g.Players[0].CmdDamage; len(got) != 1 || got[0] != 0 {
		t.Errorf("CmdDamage = %v, want one zero", got)
	}
}

// TestRestartDropsLostSeatsCards: a player who lost before the restart is not
// in the new game (CR 727.1, CR 800.4), so their cards cease rather than being
// minted into a library finishRestart never shuffles or deals.
func TestRestartDropsLostSeatsCards(t *testing.T) {
	g := NewGame([]string{"a", "b", "c"})
	a := restartZoneCard(t, g, 0, ZLibrary, "A")
	b := restartZoneCard(t, g, 1, ZLibrary, "B")
	c := restartZoneCard(t, g, 2, ZLibrary, "C")
	g.Players[2].Lost = true

	g.Restart(20, nil)

	if got := g.Obj(c).Zone; got != ZCeased {
		t.Errorf("lost seat's card zone = %s, want Ceased", got)
	}
	if got := g.Zone(ZLibrary, 2); len(got) != 0 {
		t.Errorf("lost seat's library = %v, want empty", got)
	}
	for p, old := range []ObjID{a, b} {
		if got := g.Obj(old).Zone; got != ZCeased {
			t.Errorf("seat %d old card zone = %s, want Ceased", p, got)
		}
		lib := g.Zone(ZLibrary, PlayerID(p))
		if len(lib) != 1 || lib[0] == old {
			t.Errorf("seat %d library = %v, want one fresh card", p, lib)
		}
	}
}

// TestRestartClearsTerminalAndDesignationState: the new game is not the old
// one, so a fold after GameOver must not leave the new game over, and the
// per-game designations, turn position and player notes start at genesis.
func TestRestartClearsTerminalAndDesignationState(t *testing.T) {
	g := NewGame([]string{"a", "b"})
	g.Over, g.Winner, g.Draw = true, 1, true
	g.StartingPlayer, g.HasStartingPlayer = 1, true
	g.Turn, g.Step, g.Passes = 9, StepMain2, 3
	g.Active, g.Priority = 1, 1
	g.Monarch, g.HasMonarch = 1, true
	g.Initiative, g.HasInitiative = 1, true
	g.DayNight, g.PrevTurnSpells = DayNightNight, 3
	g.ExtraTurns = map[PlayerID]int{0: 1}
	g.SkipTurns = map[PlayerID]int{0: 1}
	g.DelayedNext = 5
	g.Players[0].Notes = []string{"note"}

	g.Restart(20, nil)

	if g.Over || g.Winner != 0 || g.Draw {
		t.Errorf("terminal state = Over %v Winner %d Draw %v, want false/0/false", g.Over, g.Winner, g.Draw)
	}
	if g.StartingPlayer != 0 || g.HasStartingPlayer {
		t.Errorf("starting player = %d (has %v), want none", g.StartingPlayer, g.HasStartingPlayer)
	}
	if g.Turn != 0 || g.Step != StepUntap || g.Passes != 0 || g.Active != 0 || g.Priority != 0 {
		t.Errorf("turn position = turn %d step %s passes %d active %d priority %d, want genesis", g.Turn, g.Step, g.Passes, g.Active, g.Priority)
	}
	if g.HasMonarch || g.HasInitiative {
		t.Errorf("designations = monarch %v initiative %v, want none", g.HasMonarch, g.HasInitiative)
	}
	if g.DayNight != DayNightNeither || g.PrevTurnSpells != 0 {
		t.Errorf("day/night = %v prev spells %d, want neither/0", g.DayNight, g.PrevTurnSpells)
	}
	if g.ExtraTurns != nil || g.SkipTurns != nil || g.DelayedNext != 0 {
		t.Errorf("extra turns %v skip turns %v delayed next %d, want cleared", g.ExtraTurns, g.SkipTurns, g.DelayedNext)
	}
	if got := g.Players[0].Notes; got != nil {
		t.Errorf("player notes = %v, want cleared", got)
	}
}
