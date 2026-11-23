package oracle

import "testing"

// CR 106.1b, 601.2f: a Mystic Skull that has transformed into Mystic Monstrosity
// gives every land its controller has "{T}: Add one mana of any color". With
// Desolate Mire, Slagwoods Bridge, three Mountains, a Swamp and an Island all
// untapped that is seven any-colour sources, enough for Cruel Ultimatum's
// {U}{U}{B}{B}{B}{R}{R}. Gorge offered the cast and dirac did not (decision 384).
func TestMBCCruelUltimatumReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "seven_land_grant_any_colour_cast")
}
