package oracle

import "testing"

// CR 614.15 with CR 107.14f (gorge prevNextTake, prevTokInstr): Ria Ivor's
// Effect "the next time target creature would deal combat damage ... prevent
// that damage. If damage is prevented this way, create that many Mite tokens"
// spends as a token rider: the kernel reads the rider's token from the
// Effect's source card program (prevTokInstr), so the cardset's per-matchup
// remap must turn the program's pool token index into this table's CardID.
// It did not: the unremapped pool index 254 rode into the 58-entry game
// table, and spending the shield panicked
// (index out of range [254] with length 58) instead of minting the Mites
// (dr-mbc-... pair 1 of pool tail-12, seed 2680817112924530753).
func TestMBCVindictiveFlamestokerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_prevnext_token_rider")
}
