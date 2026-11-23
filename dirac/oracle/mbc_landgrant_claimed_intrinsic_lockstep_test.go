package oracle

import "testing"

// Mage-Ring Responder's {7} cast was offered as an option by the kernel but
// not by gorge at p1's priority decision 824 (tail-41 pair 6, surface autopay,
// mulligans 1, seed 967027): Realmwright's granted Swamp gave Cabal Coffers an
// intrinsic "{T}: Add {B}" unit beside Coffers' own "{2}, {T}: Add B mana".
// Gorge's pay.ManaAbilitiesCoverIntrinsic (895d173b7d) counts only bare-{T}
// abilities as covering the intrinsic, so a costed ability does not stand in
// for it and gorge appends the unit; the kernel does the same (it keeps no
// dedup on a costed ability's Produced$), and the pair-6 recording replays
// clean. The test pins that agreement.
func TestMBCMageRingResponderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "landgrant_claimed_intrinsic")
}
