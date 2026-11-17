package oracle

import "testing"

// A defeated Siege's controller is asked whether to cast its back face
// (Invasion of Karsus -> Refractive Embrace); one engine offered the ask and
// the other went straight to priority. The game was found in a tail-44 scan
// (Ledger Shredder / Toski decks) and is recorded on the custom pool: its table
// is built from its own deck lists, since a synthetic pool's roster is
// regenerated with the sources and may drop a recorded card (Fireblast).
func TestMBCLedgerShredderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "siege_defeat_cast_offer")
}
