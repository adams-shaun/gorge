package oracle

import "testing"

// Mystic Redaction and Solve the Equation ({2}{U} each) were offered at
// priority by gorge and not by the kernel (decision 334 of 334, tail-52 pair
// 52 at d93f6109a3). The kernel's filter-fee funding is what f700b2ac45
// changed (a source that funds by sacrifice alone, CR 605.1a): the recording
// diverges without that change and replays clean with it. The decks are the
// pair's own, so the table is built from them (tail-52 no longer holds every
// card in them).
func TestMBCMysticRedactionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mystic_redaction_cast_offer")
}
