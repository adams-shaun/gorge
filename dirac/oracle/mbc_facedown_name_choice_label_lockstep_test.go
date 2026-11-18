package oracle

import "testing"

// CR 201.4: a card-name choice ranges over the Oracle card reference, so the
// offered names are the choice's vocabulary, never a reference to a game
// object. With an opponent's face-down permanent whose face is Keeper of
// Secrets on the battlefield, gorge's face-down hiding (CR 708.5) once
// rewrote the "Keeper of Secrets" name option to "a face-down permanent" in
// the Cheering Fanatic attack trigger's name ask (decision 428 of synth
// tail-19-c001 pair 195, seed 629502, surface autopay): the ask offered one
// name fewer than the universe, its rewritten label named no card, and an
// answer taken from it would have recorded a name no spell can carry. A
// name-option label must survive the hiding the way a type-word label does
// (facedown_type_label).
func TestMBCCheeringFanaticReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "facedown_name_choice_label")
}
