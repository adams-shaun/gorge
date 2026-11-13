package kgen

import (
	"strings"
	"testing"
)

// A graveyard pick's DefinedPlayer$ (lower_gravedefinedplayer.go): Exhume's
// each-player pick and Genju of the Fields' own-graveyard return lower; the
// game-wide pick (no fetch player) and every other spelling refuse by name.
func TestLowerGraveyardPickDefinedPlayer(t *testing.T) {
	const exhume = "SP$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | ChangeType$ Creature | DefinedPlayer$ Player | ChangeNum$ 1 | Hidden$ True | Mandatory$ True"
	refusals, ops := czLowered(t, exhume)
	if len(refusals) != 0 {
		t.Fatalf("Exhume's shape refused: %q", refusals)
	}
	if got, want := strings.Join(ops, " "), "RepeatEach: ChooseMove:from=Graveyard|Mandatory"; got != want {
		t.Errorf("Exhume ops = %q, want %q", got, want)
	}

	const genju = "SP$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ChangeType$ Card.StrictlySelf | DefinedPlayer$ You | Hidden$ True | Mandatory$ True"
	refusals, ops = czLowered(t, genju)
	if len(refusals) != 0 {
		t.Fatalf("Genju's shape refused: %q", refusals)
	}
	if got, want := strings.Join(ops, " "), "ChooseMove:from=Graveyard|Mandatory"; got != want {
		t.Errorf("Genju ops = %q, want %q", got, want)
	}

	for _, c := range []struct{ name, root, want string }{
		{"no fetch player is the game-wide pool", strings.Replace(exhume, " | DefinedPlayer$ Player", "", 1), "graveyard choice without an owner"},
		{"each player to the hand", strings.Replace(exhume, "Destination$ Battlefield", "Destination$ Hand", 1), "DefinedPlayer$ Player to Hand"},
		{"each player with a chooser", exhume + " | Chooser$ You", "DefinedPlayer$ Player with Chooser$"},
		{"each player may", strings.Replace(exhume, " | Mandatory$ True", " | Optional$ True", 1), "DefinedPlayer$ Player with Optional$"},
		{"each player tapped", exhume + " | Tapped$ True", "DefinedPlayer$ Player with Tapped"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := czLowered(t, c.root)
			kdefWant(t, got, c.want)
		})
	}

	// Arzakon's -7: the Echo of Eons sweep beside its loyalty cost lowers; a
	// further key still refuses by name
	const echo = "AB$ ChangeZoneAll | Cost$ SubCounter<7/LOYALTY> | Planeswalker$ True | Ultimate$ True | ChangeType$ Card | Origin$ Hand,Graveyard | Destination$ Library | Shuffle$ True | UseAllOriginZones$ True"
	if refusals, ops = czLowered(t, echo); len(refusals) != 0 || len(ops) == 0 || ops[0] != "ChangeZoneAll:from=Hand,Graveyard|ShuffleEach" {
		t.Errorf("the Echo shape with a cost: refusals %q ops %q", refusals, ops)
	}
	got, _ := czLowered(t, echo+" | RememberChanged$ True")
	kdefWant(t, got, "with RememberChanged$")
}
