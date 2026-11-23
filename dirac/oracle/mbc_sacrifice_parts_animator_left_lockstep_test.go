package oracle

import "testing"

// Tomb of Annihilation's Oubliette room sacrifices an artifact, a creature and
// a land part by part. The player sacrificed The Blackstaff of Waterdeep for the
// artifact part, which ended its animation of Servo Schematic ("for as long as
// it remains tapped"), so Servo Schematic was no creature for the creature
// part. The gorge side offered it anyway: the layer tables its resolution
// published at entry were not taken again between the parts (decision 625,
// tail-20-c000 pair 14).
func TestMBCServoSchematicReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sacrifice_parts_read_the_board_after_the_animator_left")
}
