package oracle

import "testing"

// CR 603.6a, CR 303.4f: Brago, King Eternal exiles nine nonland permanents and
// returns them as one entering event; the returning Ossification asks which
// land it enchants, and once that is answered its enters trigger is put on the
// stack once. The kernel fired it on the answer and again with the batch's
// held entrants (their stamps' low three bits repeat from the ninth entrant),
// so it offered two orderings of Ossification's trigger where gorge had one
// (decision 648, pool tail-23 pair 7).
func TestMBCOssificationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "many_return_aura_enter_fires_once")
}
